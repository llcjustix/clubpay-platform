package httpapi

import (
	"bytes"
	"clubpay/internal/config"
	"clubpay/internal/core"
	clubdb "clubpay/internal/db"
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestBootGuardFencingIntegration(t *testing.T) {
	raw := os.Getenv("MOBILE_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("requires disposable PostgreSQL")
	}
	ctx := context.Background()
	admin, e := pgxpool.New(ctx, raw)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close()
	schema := "cpb_test_" + randomHex(6)
	if _, e = admin.Exec(ctx, "CREATE SCHEMA "+schema); e != nil {
		t.Fatal(e)
	}
	defer admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	cfg, _ := pgxpool.ParseConfig(raw)
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	db, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if e = clubdb.RunMigrations(ctx, db, "../../migrations"); e != nil {
		t.Fatal(e)
	}
	var club, pc, external string
	if e = db.QueryRow(ctx, "SELECT club_id,id,external_pc_id FROM pc_refs LIMIT 1").Scan(&club, &pc, &external); e != nil {
		t.Fatal(e)
	}
	// Live guard is deliberately offline; only one explicit cold provisioning is
	// authorized. Later operations without live critical-state evidence must deny.
	ws := core.NewWSController("agent-private", time.Second)
	server := NewServer(config.Config{BootGuardToken: "dedicated-boot-credential", BootGuardClubID: club}, db, ws)
	request := func(path string, body any, token string) (int, map[string]any) {
		v, _ := json.Marshal(body)
		r := httptest.NewRequest("POST", path, bytes.NewReader(v))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		server.Routes().ServeHTTP(w, r)
		var out map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out
	}
	token := "dedicated-boot-credential"
	if code, _ := request("/api/cpb/v1/stations", map[string]any{"club_id": club, "external_pc_id": external, "cold_provision": true}, "foreign"); code != 401 {
		t.Fatal(code)
	}
	if code, out := request("/api/cpb/v1/stations", map[string]any{"club_id": club, "external_pc_id": external, "cold_provision": true}, token); code != 200 {
		t.Fatal(code, out)
	}
	q := bootLeaseRequest{ClubID: club, ExternalPCID: external, CommandID: "first", Operation: "bootstrap", TTLSeconds: 60}
	code, out := request("/api/cpb/v1/leases/acquire", q, token)
	if code != 200 {
		t.Fatal(code, out)
	}
	q.LeaseID = out["lease_id"].(string)
	q.Fence = int64(out["fence"].(float64))

	critical := false
	live := &bootStatusCore{Adapter: ws, status: core.PCStatus{AgentOnline: true, Status: "blocked", AgentCritical: &critical}}
	server.core = live
	if c, _ := request("/api/cpb/v1/leases/validate", q, token); c != 200 {
		t.Fatal("valid live guard", c)
	}
	critical = true
	if c, _ := request("/api/cpb/v1/leases/validate", q, token); c != 409 {
		t.Fatal("new critical state accepted by validate", c)
	}
	live.err = fmt.Errorf("unavailable")
	if c, _ := request("/api/cpb/v1/leases/validate", q, token); c != 503 {
		t.Fatal("unavailable live guard accepted", c)
	}
	server.core = ws
	if code, _ = request("/api/cpb/v1/leases/acquire", q, token); code != 200 {
		t.Fatal("duplicate lease", code)
	}
	foreign := q
	foreign.CommandID = "second"
	if code, _ = request("/api/cpb/v1/leases/acquire", foreign, token); code != 409 {
		t.Fatal("fence replaced", code)
	}
	if finish, err := server.bootCommandGate(ctx, external, "start_session"); err == nil {
		finish()
		t.Fatal("start allowed")
	}
	// Even another process writing grants directly cannot bypass the latch.
	if _, err := db.Exec(ctx, `INSERT INTO game_access_grants(club_id,pc_ref_id,duration_minutes,status) VALUES($1,$2,1,'pending')`, club, pc); err == nil {
		t.Fatal("grant admitted while fenced")
	}
	// Expiry forbids mutation but retains scheduling quarantine across restart.
	if _, e = db.Exec(ctx, "UPDATE boot_guard_leases SET expires_at=now()-interval '1 second' WHERE pc_ref_id=$1", pc); e != nil {
		t.Fatal(e)
	}
	if code, _ = request("/api/cpb/v1/leases/validate", q, token); code != 409 {
		t.Fatal("stale lease valid", code)
	}
	server = NewServer(config.Config{BootGuardToken: token, BootGuardClubID: club}, db, ws)
	if finish, err := server.bootCommandGate(ctx, external, "unlock"); err == nil {
		finish()
		t.Fatal("restart lost fence")
	}
	stale := q
	stale.Fence--
	if code, _ = request("/api/cpb/v1/leases/release", stale, token); code != 409 {
		t.Fatal("stale fence released")
	}
	if code, out = request("/api/cpb/v1/leases/release", q, token); code != 200 {
		t.Fatal(code, out)
	}
	q.CommandID = "next"
	q.LeaseID = ""
	q.Fence = 0
	if code, _ = request("/api/cpb/v1/leases/acquire", q, token); code != 409 {
		t.Fatal("offline provision reused", code)
	}
	// An in-flight Controller command owns the station lock. Acquire must wait
	// for it, then observe the new pending session instead of allowing a switch.
	finish, e := server.bootCommandGate(ctx, external, "start_session")
	if e != nil {
		t.Fatal(e)
	}
	done := make(chan int, 1)
	go func() { c, _ := request("/api/cpb/v1/leases/acquire", q, token); done <- c }()
	select {
	case <-done:
		t.Fatal("acquire raced in-flight command")
	case <-time.After(80 * time.Millisecond):
	}
	finish()
	select {
	case c := <-done:
		if c != 409 {
			t.Fatal(c)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("acquire stuck")
	}
}

// Explicit admission checks cover state written before lease acquisition.
func TestBootGuardActiveAndBookingIntegration(t *testing.T) {
	raw := os.Getenv("MOBILE_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("requires disposable PostgreSQL")
	}
	ctx := context.Background()
	admin, e := pgxpool.New(ctx, raw)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close()
	schema := "cpb_busy_" + randomHex(6)
	_, e = admin.Exec(ctx, "CREATE SCHEMA "+schema)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	cfg, _ := pgxpool.ParseConfig(raw)
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	db, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if e = clubdb.RunMigrations(ctx, db, "../../migrations"); e != nil {
		t.Fatal(e)
	}
	var club, pc, external, player string
	if e = db.QueryRow(ctx, "SELECT club_id,id,external_pc_id FROM pc_refs LIMIT 1").Scan(&club, &pc, &external); e != nil {
		t.Fatal(e)
	}
	_, e = db.Exec(ctx, "INSERT INTO boot_guard_stations(pc_ref_id,cold_provision) VALUES($1,true)", pc)
	if e != nil {
		t.Fatal(e)
	}
	s := NewServer(config.Config{BootGuardToken: "boot", BootGuardClubID: club}, db, core.NewWSController("agent", time.Second))
	request := func() int {
		b, _ := json.Marshal(bootLeaseRequest{ClubID: club, ExternalPCID: external, CommandID: "busy", Operation: "bootstrap", TTLSeconds: 60})
		r := httptest.NewRequest("POST", "/api/cpb/v1/leases/acquire", bytes.NewReader(b))
		r.Header.Set("Authorization", "Bearer boot")
		w := httptest.NewRecorder()
		s.Routes().ServeHTTP(w, r)
		return w.Code
	}
	_, e = db.Exec(ctx, `INSERT INTO game_access_grants(club_id,pc_ref_id,duration_minutes,status) VALUES($1,$2,1,'pending')`, club, pc)
	if e != nil {
		t.Fatal(e)
	}
	if c := request(); c != 409 {
		t.Fatal("active admission", c)
	}
	_, e = db.Exec(ctx, "UPDATE game_access_grants SET status='failed' WHERE pc_ref_id=$1", pc)
	if e != nil {
		t.Fatal(e)
	}
	e = db.QueryRow(ctx, "INSERT INTO players(phone) VALUES('+998900991234') RETURNING id").Scan(&player)
	if e != nil {
		t.Fatal(e)
	}
	_, e = db.Exec(ctx, `INSERT INTO mobile_reservations(player_id,club_id,pc_ref_id,starts_at,duration_minutes,entry_code) VALUES($1,$2,$3,now()+interval '20 minutes',60,'991234')`, player, club, pc)
	if e != nil {
		t.Fatal(e)
	}
	if c := request(); c != 409 {
		t.Fatal("booking admission", c)
	}
	if _, e = db.Exec(ctx, "UPDATE mobile_reservations SET starts_at=now()+interval '2 hours' WHERE pc_ref_id=$1", pc); e != nil {
		t.Fatal(e)
	}
	if c := request(); c != 200 {
		t.Fatal("far booking blocked bootstrap", c)
	}
	if _, e = db.Exec(ctx, "UPDATE mobile_reservations SET starts_at=now()+interval '1 minute' WHERE pc_ref_id=$1", pc); e == nil {
		t.Fatal("rescheduled booking bypassed fence")
	}
	if _, e = db.Exec(ctx, "UPDATE mobile_reservations SET status='cancelled' WHERE pc_ref_id=$1", pc); e != nil {
		t.Fatal("cancellation blocked", e)
	}
}

type bootStatusCore struct {
	core.Adapter
	status core.PCStatus
	err    error
}

func (c *bootStatusCore) GetPCStatus(context.Context, string) (core.PCStatus, error) {
	return c.status, c.err
}
