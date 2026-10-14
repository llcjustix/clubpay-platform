package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"testing"

	"clubpay/internal/config"
	"clubpay/internal/core"
	clubdb "clubpay/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

func agentRoutesTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	raw := os.Getenv("MOBILE_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("requires disposable PostgreSQL")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	schema := "agent_routes_" + randomHex(6)
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	cfg, err := pgxpool.ParseConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Close(); _, _ = admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); admin.Close() })
	if err = clubdb.RunMigrations(ctx, pool, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	return pool
}

func TestAgentControllerRoutesAcrossClients(t *testing.T) {
	pool := agentRoutesTestDB(t)
	ctx := context.Background()
	var club, pc, user, other string
	if err := pool.QueryRow(ctx, "SELECT club_id,id FROM pc_refs LIMIT 1").Scan(&club, &pc); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "INSERT INTO clubs(name,slug) VALUES('Other routes club','other-routes') RETURNING id").Scan(&other); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "INSERT INTO users(name,role) VALUES('Routes owner','owner') RETURNING id").Scan(&user); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO user_club_roles(user_id,club_id,role,status) VALUES($1,$2,'owner','active')", user, club); err != nil {
		t.Fatal(err)
	}
	// Two different browser sessions authenticate as the same owner.
	for _, token := range []string{"routes-browser-a", "routes-browser-b"} {
		if _, err := pool.Exec(ctx, "INSERT INTO auth_sessions(user_id,token_hash,expires_at) VALUES($1,$2,now()+interval '1 hour')", user, hashToken(token)); err != nil {
			t.Fatal(err)
		}
	}
	server := NewServer(config.Config{AppEnv: "production", CoreToken: "disposable-agent-token"}, pool, core.NewMockAdapter())
	call := func(s *Server, method, path, body, token string) (int, map[string]any) {
		t.Helper()
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+token)
		out := httptest.NewRecorder()
		s.Routes().ServeHTTP(out, req)
		var payload map[string]any
		_ = json.Unmarshal(out.Body.Bytes(), &payload)
		return out.Code, payload
	}
	savePath := "/api/backoffice/clubs/" + club + "/agent-controller-routes"
	pair := `{"controller_url":"192.168.100.153:8080","fallback_controller_url":"192.168.100.152:8080"}`
	if code, out := call(server, "POST", savePath, pair, "routes-browser-a"); code != 200 {
		t.Fatal(code, out)
	}
	// Recreate the Server: browser B must load durable data without any cache.
	restarted := NewServer(config.Config{AppEnv: "production", CoreToken: "disposable-agent-token"}, pool, core.NewMockAdapter())
	code, out := call(restarted, "GET", "/api/backoffice/clubs/"+club+"/settings", "", "routes-browser-b")
	if code != 200 {
		t.Fatal(code, out)
	}
	routes := out["agent_controller_routes"].(map[string]any)
	if routes["controller_url"] != "http://192.168.100.153:8080" || routes["fallback_controller_url"] != "http://192.168.100.152:8080" {
		t.Fatal(routes)
	}
	if _, exists := routes["core_token"]; exists {
		t.Fatal("route settings leaked enrollment credential")
	}
	code, out = call(restarted, "POST", "/api/backoffice/pcs/"+pc+"/agent-enrollment", "{}", "routes-browser-b")
	if code != 200 {
		t.Fatal(code, out)
	}
	enrollment := out["enrollment"].(map[string]any)
	if enrollment["controller_url"] != routes["controller_url"] || enrollment["fallback_controller_url"] != routes["fallback_controller_url"] {
		t.Fatal("installer did not use durable club pair", enrollment)
	}
	for _, body := range []string{
		`{"controller_url":""}`,
		`{"controller_url":"192.168.100.153:8080","fallback_controller_url":"http://user:password@bad"}`,
		`{"controller_url":"https://bad.example/path"}`,
	} {
		if code, _ := call(server, "POST", savePath, body, "routes-browser-a"); code != 400 {
			t.Fatal("invalid routes accepted", code)
		}
	}
	current, err := server.agentControllerRoutes(ctx, club)
	if err != nil || current.ControllerURL != routes["controller_url"] || current.FallbackControllerURL != routes["fallback_controller_url"] {
		t.Fatal("validation modified saved pair", current, err)
	}
	if code, _ := call(server, "POST", "/api/backoffice/clubs/"+other+"/agent-controller-routes", pair, "routes-browser-a"); code != 403 {
		t.Fatal("cross-club write accepted", code)
	}
	untouched, err := server.agentControllerRoutes(ctx, other)
	if err != nil || untouched.ControllerURL != "" {
		t.Fatal("other club changed", untouched, err)
	}
	if code, _ := call(server, "POST", savePath, pair, "bad-token"); code != 401 {
		t.Fatal(code)
	}
	// Clear fallback intentionally; saving must not remap an installed PC.
	if code, out := call(server, "POST", savePath, `{"controller_url":"192.168.100.153:8080","fallback_controller_url":" "}`, "routes-browser-a"); code != 200 {
		t.Fatal(code, out)
	}
	var pcFallback string
	if err := pool.QueryRow(ctx, "SELECT agent_fallback_controller_url FROM pc_refs WHERE id=$1", pc).Scan(&pcFallback); err != nil || pcFallback != routes["fallback_controller_url"] {
		t.Fatal("saving remapped existing Agent", pcFallback, err)
	}
	current, err = server.agentControllerRoutes(ctx, club)
	if err != nil || current.FallbackControllerURL != "" {
		t.Fatal("fallback not cleared", current, err)
	}
	// Legacy clients can still explicitly enroll a PC with their selected pair.
	if code, _ := call(server, "POST", "/api/backoffice/pcs/"+pc+"/agent-enrollment", pair, "routes-browser-b"); code != 200 {
		t.Fatal(code)
	}
	if _, err := pool.Exec(ctx, "UPDATE user_club_roles SET role='manager' WHERE user_id=$1 AND club_id=$2", user, club); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{savePath, "/api/backoffice/pcs/" + pc + "/agent-enrollment"} {
		if code, _ := call(server, "POST", path, pair, "routes-browser-b"); code != 403 {
			t.Fatal("manager obtained deployment authority", code)
		}
	}
}

func TestAgentControllerRoutesMigration(t *testing.T) {
	pool := agentRoutesTestDB(t)
	ctx := context.Background()
	var club string
	if err := pool.QueryRow(ctx, "SELECT club_id FROM pc_refs LIMIT 1").Scan(&club); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE pc_refs SET agent_primary_controller_url='http://192.168.100.153:8080',agent_fallback_controller_url='http://192.168.100.152:8080' WHERE club_id=$1", club); err != nil {
		t.Fatal(err)
	}
	sql, err := os.ReadFile("../../migrations/027_club_agent_controller_routes.sql")
	if err != nil {
		t.Fatal(err)
	}
	run := func() {
		t.Helper()
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			t.Fatal(err)
		}
	}
	run()
	server := NewServer(config.Config{}, pool, core.NewMockAdapter())
	routes, err := server.agentControllerRoutes(ctx, club)
	if err != nil || routes.ControllerURL != "http://192.168.100.153:8080" || routes.FallbackControllerURL != "http://192.168.100.152:8080" {
		t.Fatal(routes, err)
	}
	if _, err := pool.Exec(ctx, "UPDATE clubs SET agent_primary_controller_url='http://new-controller:8080',agent_fallback_controller_url=NULL WHERE id=$1", club); err != nil {
		t.Fatal(err)
	}
	run()
	routes, err = server.agentControllerRoutes(ctx, club)
	if err != nil || routes.ControllerURL != "http://new-controller:8080" || routes.FallbackControllerURL != "" {
		t.Fatal("migration overwrote saved settings", routes, err)
	}
	if _, err := pool.Exec(ctx, "UPDATE clubs SET agent_primary_controller_url=NULL WHERE id=$1", club); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE pc_refs SET agent_fallback_controller_url='http://conflicting:8080' WHERE id=(SELECT id FROM pc_refs WHERE club_id=$1 LIMIT 1)", club); err != nil {
		t.Fatal(err)
	}
	run()
	routes, err = server.agentControllerRoutes(ctx, club)
	if err != nil || routes.ControllerURL != "" {
		t.Fatal("migration guessed from conflicting pairs", routes, err)
	}
}

func TestAgentControllerRoutesEdgeSnapshot(t *testing.T) {
	pool := agentRoutesTestDB(t)
	ctx := context.Background()
	var club string
	if err := pool.QueryRow(ctx, "SELECT club_id FROM pc_refs LIMIT 1").Scan(&club); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE clubs SET agent_primary_controller_url='http://primary:8080',agent_fallback_controller_url='http://fallback:8080' WHERE id=$1", club); err != nil {
		t.Fatal(err)
	}
	server := NewServer(config.Config{}, pool, core.NewMockAdapter())
	snapshot, err := server.edgeSnapshotData(ctx, club, true)
	if err != nil {
		t.Fatal(err)
	}
	payloadClub := snapshot["club"].(map[string]any)
	if payloadClub["agent_primary_controller_url"] != "http://primary:8080" {
		t.Fatal("snapshot dropped routes")
	}
	if _, err := pool.Exec(ctx, "UPDATE clubs SET agent_primary_controller_url=NULL,agent_fallback_controller_url=NULL WHERE id=$1", club); err != nil {
		t.Fatal(err)
	}
	if err := server.applyEdgeSnapshotData(ctx, club, snapshot); err != nil {
		t.Fatal(err)
	}
	routes, err := server.agentControllerRoutes(ctx, club)
	if err != nil || routes.FallbackControllerURL != "http://fallback:8080" {
		t.Fatal(routes, err)
	}
	payloadClub["agent_fallback_controller_url"] = nil
	if err := server.applyEdgeSnapshotData(ctx, club, snapshot); err != nil {
		t.Fatal(err)
	}
	routes, err = server.agentControllerRoutes(ctx, club)
	if err != nil || routes.FallbackControllerURL != "" {
		t.Fatal("snapshot did not clear fallback", routes, err)
	}
	delete(payloadClub, "agent_primary_controller_url")
	delete(payloadClub, "agent_fallback_controller_url")
	if err := server.applyEdgeSnapshotData(ctx, club, snapshot); err != nil {
		t.Fatal(err)
	}
	routes, err = server.agentControllerRoutes(ctx, club)
	if err != nil || routes.ControllerURL != "http://primary:8080" {
		t.Fatal("older snapshot erased routes", routes, err)
	}
}
