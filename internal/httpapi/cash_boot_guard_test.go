package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"clubpay/internal/config"
	"clubpay/internal/core"
	clubdb "clubpay/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

type cashBootGateCore struct {
	core.Adapter
	server         *Server
	pool           *pgxpool.Pool
	pendingVisible bool
	startError     bool
}

func (c *cashBootGateCore) StartSession(ctx context.Context, cmd core.StartSessionCommand) (core.StartSessionResult, error) {
	var status string
	if err := c.pool.QueryRow(ctx, "SELECT status FROM game_access_grants WHERE id=$1", cmd.GrantID).Scan(&status); err != nil {
		return core.StartSessionResult{}, fmt.Errorf("pending reservation not committed: %w", err)
	}
	c.pendingVisible = status == "pending"
	release, err := c.server.bootCommandGate(ctx, cmd.PCExternalID, "start_session")
	if err != nil {
		return core.StartSessionResult{}, err
	}
	defer release()
	if c.startError {
		return core.StartSessionResult{}, fmt.Errorf("test agent rejected start")
	}
	return core.StartSessionResult{CoreSessionID: "cash-boot-regression-session"}, nil
}

func TestCashStartCommitsPendingBeforeBootCommandGate(t *testing.T) {
	raw := os.Getenv("MOBILE_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("requires disposable PostgreSQL")
	}
	for _, tc := range []struct{ reject, fenced bool }{{false, false}, {true, false}, {false, true}} {
		reject := tc.reject
		t.Run(fmt.Sprint("agentReject=", reject, "/fenced=", tc.fenced), func(t *testing.T) {
			ctx := context.Background()
			admin, err := pgxpool.New(ctx, raw)
			if err != nil {
				t.Fatal(err)
			}
			defer admin.Close()
			schema := "cash_boot_" + randomHex(6)
			if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
				t.Fatal(err)
			}
			defer admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
			cfg, err := pgxpool.ParseConfig(raw)
			if err != nil {
				t.Fatal(err)
			}
			cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
			pool, err := pgxpool.NewWithConfig(ctx, cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer pool.Close()
			if err = clubdb.RunMigrations(ctx, pool, "../../migrations"); err != nil {
				t.Fatal(err)
			}
			var club, pc, external, user string
			if err = pool.QueryRow(ctx, "SELECT club_id,id,external_pc_id FROM pc_refs WHERE status_cache='available' LIMIT 1").Scan(&club, &pc, &external); err != nil {
				t.Fatal(err)
			}
			if _, err = pool.Exec(ctx, "INSERT INTO boot_guard_stations(pc_ref_id) VALUES($1)", pc); err != nil {
				t.Fatal(err)
			}
			if err = pool.QueryRow(ctx, "INSERT INTO users(club_id,name,email,role,global_role) VALUES($1,'Cash regression operator',$2,'admin','super_admin') RETURNING id", club, "cash-"+randomHex(5)+"@example.test").Scan(&user); err != nil {
				t.Fatal(err)
			}
			token := "cash-regression-" + randomHex(8)
			if _, err = pool.Exec(ctx, "INSERT INTO auth_sessions(user_id,token_hash,expires_at) VALUES($1,$2,now()+interval '1 hour')", user, hashToken(token)); err != nil {
				t.Fatal(err)
			}
			if tc.fenced {
				if _, err = pool.Exec(ctx, "INSERT INTO boot_guard_leases(pc_ref_id,command_id,lease_id,fence,expires_at,held) VALUES($1,'expired-test','test-lease',123,now()-interval '1 second',true)", pc); err != nil {
					t.Fatal(err)
				}
			}
			adapter := &cashBootGateCore{Adapter: core.NewMockAdapter(), pool: pool, startError: reject}
			server := NewServer(config.Config{BootGuardToken: "regression-guard", BootGuardClubID: club}, pool, adapter)
			adapter.server = server
			body, _ := json.Marshal(map[string]any{"pc_id": pc, "amount_uzs": 500, "reason": "cash_payment"})
			deadline, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			req := httptest.NewRequest("POST", "/api/admin/cash-sessions", bytes.NewReader(body)).WithContext(deadline)
			req.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			server.Routes().ServeHTTP(w, req)
			if tc.fenced {
				if w.Code != 409 || !strings.Contains(w.Body.String(), "cpb_station_fenced") {
					t.Fatalf("quarantine returned server failure: %d %s", w.Code, w.Body.String())
				}
				if adapter.pendingVisible {
					t.Fatal("cash dispatched while held")
				}
				var count int
				if err = pool.QueryRow(ctx, "SELECT (SELECT count(*) FROM cash_payments WHERE pc_ref_id=$1)+(SELECT count(*) FROM game_access_grants WHERE pc_ref_id=$1)", pc).Scan(&count); err != nil || count != 0 {
					t.Fatal("rejected cash transaction retained rows", count, err)
				}
				return
			}
			expected := 201
			grantStatus := "accepted"
			if reject {
				expected = 500
				grantStatus = "start_failed"
			}
			if w.Code != expected {
				t.Fatalf("status %d, body %s", w.Code, w.Body.String())
			}
			if !adapter.pendingVisible {
				t.Fatal("command dispatched before pending reservation was durable")
			}
			var status string
			if err = pool.QueryRow(ctx, "SELECT status FROM game_access_grants WHERE pc_ref_id=$1", pc).Scan(&status); err != nil || status != grantStatus {
				t.Fatalf("grant outcome %q: %v", status, err)
			}
		})
	}
}
