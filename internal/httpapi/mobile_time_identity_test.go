package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMobileLongTimeOperationIdentity(t *testing.T) {
	for _, flow := range []string{"balance", "payment", "extension"} {
		t.Run(flow, func(t *testing.T) {
			pool, s, club, pc := snapshotStorageFixture(t)
			s.cfg.MockPaymentsEnabled = true
			s.cfg.MobileReturnBaseURL = "https://fixture.example.test"
			ctx := context.Background()
			const seconds = 35*3600 + 18*60 + 49
			durationSeconds := seconds
			if flow != "balance" {
				// Custom-sum purchases round fractional seconds up, never down.
				durationSeconds = seconds + 1
			}
			var player, session, token, qr string
			token = "mob_a_time_identity_fixture"
			if err := pool.QueryRow(ctx, `INSERT INTO players(phone) VALUES('+998900000097') RETURNING id`).Scan(&player); err != nil {
				t.Fatal(err)
			}
			if err := pool.QueryRow(ctx, `INSERT INTO mobile_sessions(player_id,device_hash,expires_at) VALUES($1,'time-fixture',now()+interval '1 day') RETURNING id`, player).Scan(&session); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, `INSERT INTO mobile_tokens(token_hash,session_id,kind,expires_at) VALUES($1,$2,'access',now()+interval '1 hour')`, hashToken(token), session); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, `UPDATE zones SET hourly_price_tiyin=1500000 WHERE club_id=$1`, club); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, `UPDATE pc_refs SET label='1',status_cache='available' WHERE id=$1`, pc); err != nil {
				t.Fatal(err)
			}
			if err := pool.QueryRow(ctx, `SELECT public_token FROM qr_codes WHERE pc_ref_id=$1 AND type='static_pc' AND status='active' LIMIT 1`, pc).Scan(&qr); err != nil {
				t.Fatal(err)
			}
			var parent string
			if flow == "extension" {
				if err := pool.QueryRow(ctx, `INSERT INTO game_access_grants(club_id,pc_ref_id,player_id,duration_minutes,duration_seconds,status,source,core_session_id,accepted_at,planned_ends_at,grace_ends_at)
					VALUES($1,$2,$3,60,3600,'accepted','player_balance','fixture-existing-session',now(),now()+interval '1 hour',now()+interval '65 minutes') RETURNING id`, club, pc, player).Scan(&parent); err != nil {
					t.Fatal(err)
				}
				var err error
				qr, err = s.ensureSessionExtendToken(ctx, club, pc, parent, time.Now().Add(2*time.Hour))
				if err != nil {
					t.Fatal(err)
				}
				if _, err = pool.Exec(ctx, `UPDATE pc_refs SET status_cache='occupied' WHERE id=$1`, pc); err != nil {
					t.Fatal(err)
				}
			}
			request := func(method, path, key string, body any, code int) map[string]any {
				t.Helper()
				payload, _ := json.Marshal(body)
				req := httptest.NewRequest(method, path, bytes.NewReader(payload))
				req.Header.Set("Authorization", "Bearer "+token)
				if key != "" {
					req.Header.Set("Idempotency-Key", key)
				}
				out := httptest.NewRecorder()
				s.Routes().ServeHTTP(out, req)
				if out.Code != code {
					t.Fatalf("%s %s: HTTP %d: %s", method, path, out.Code, out.Body.String())
				}
				var result map[string]any
				if err := json.Unmarshal(out.Body.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				return result
			}
			var grant, invoice string
			if flow == "balance" {
				tx, err := pool.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback(ctx)
				if err = s.recordPlayerTime(ctx, tx, player, club, seconds, "manual_adjustment", "", "", "fixture-long-balance"); err != nil {
					t.Fatal(err)
				}
				if err = tx.Commit(ctx); err != nil {
					t.Fatal(err)
				}
				for i := 0; i < 2; i++ {
					result := request("POST", "/api/player-balance/redeem", "fixture-long-balance-request", map[string]any{"qr_token": qr}, 200)
					if result["seconds_used"] != float64(seconds) || (grant != "" && result["grant_id"] != grant) {
						t.Fatal("balance duration or idempotency changed", result)
					}
					grant = result["grant_id"].(string)
				}
				var balance, debits int
				if err = pool.QueryRow(ctx, `SELECT seconds_balance FROM player_club_balances WHERE player_id=$1 AND club_id=$2`, player, club).Scan(&balance); err != nil || balance != 0 {
					t.Fatal("incorrect remaining balance", balance, err)
				}
				if err = pool.QueryRow(ctx, `SELECT count(*) FROM player_time_ledger WHERE player_id=$1 AND kind='session_start'`, player).Scan(&debits); err != nil || debits != 1 {
					t.Fatal("duplicate balance debit", debits, err)
				}
			} else {
				for i := 0; i < 2; i++ {
					result := request("POST", "/api/checkouts", "fixture-long-payment-request", map[string]any{"qr_token": qr, "amount_uzs": 529705, "payment_provider": "mock"}, 201)
					order := result["order"].(map[string]any)
					if invoice != "" && order["invoice_id"] != invoice {
						t.Fatal("duplicate checkout", result)
					}
					invoice = order["invoice_id"].(string)
				}
				for i := 0; i < 2; i++ {
					paid, err := s.completeMockPayment(ctx, invoice, player)
					if err != nil || (grant != "" && paid != grant) {
						t.Fatal("payment callback duration/idempotency failure", paid, err)
					}
					grant = paid
				}
				order := request("GET", "/api/mobile/orders/"+invoice, "", nil, 200)
				if order["session_seconds"] != float64(durationSeconds) || order["pc_id"] != pc || order["club_id"] != club || order["is_extension"] != (flow == "extension") {
					t.Fatal("order mixed up PC, duration or extension", order)
				}
			}
			status := request("GET", "/api/mobile/sessions/"+grant, "", nil, 200)
			if status["session_seconds"] != float64(durationSeconds) || status["pc_id"] != pc || status["club_id"] != club || status["pc_label"] != "1" {
				t.Fatal("session mixed up PC or duration", status)
			}
			var actual, count int
			if err := pool.QueryRow(ctx, `SELECT duration_seconds FROM game_access_grants WHERE id=$1`, grant).Scan(&actual); err != nil || actual != durationSeconds {
				t.Fatal("time was truncated at a day boundary", actual, err)
			}
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM game_access_grants WHERE player_id=$1 AND duration_seconds=$2`, player, durationSeconds).Scan(&count); err != nil || count != 1 {
				t.Fatal(fmt.Sprintf("duplicate %s grant", flow), count, err)
			}
		})
	}
}
