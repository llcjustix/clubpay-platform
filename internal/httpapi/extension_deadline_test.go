package httpapi

import (
	"context"
	"os"
	"testing"
	"time"

	"clubpay/internal/config"
	"clubpay/internal/core"
	clubdb "clubpay/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The Agent emits session_extended while replying to extend_session.  Its
// deadline is authoritative: a Controller must not append the same addSeconds
// to that already-updated value while completing the child grant.
func TestExtensionKeepsAgentConfirmedDeadline(t *testing.T) {
	raw := os.Getenv("MOBILE_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("requires disposable PostgreSQL")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := "extension_deadline_" + randomHex(6)
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	poolConfig, err := pgxpool.ParseConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	poolConfig.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = clubdb.RunMigrations(ctx, pool, "../../migrations"); err != nil {
		t.Fatal(err)
	}

	var clubID, pcID, externalPCID string
	if err = pool.QueryRow(ctx, `
		SELECT club_id, id, external_pc_id FROM pc_refs ORDER BY number LIMIT 1
	`).Scan(&clubID, &pcID, &externalPCID); err != nil {
		t.Fatal(err)
	}
	var parentID, childID string
	if err = pool.QueryRow(ctx, `
		INSERT INTO game_access_grants (
			club_id, pc_ref_id, duration_minutes, duration_seconds, status,
			core_session_id, source, accepted_at, planned_ends_at, grace_ends_at
		) VALUES ($1,$2,60,3600,'accepted','agent-session','online_payment',now(),now()+interval '1 hour',now()+interval '1 hour')
		RETURNING id
	`, clubID, pcID).Scan(&parentID); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `
		INSERT INTO game_access_grants (
			club_id, pc_ref_id, parent_grant_id, duration_minutes, duration_seconds,
			status, core_session_id, source
		) VALUES ($1,$2,$3,60,3600,'pending','agent-session','session_extend')
		RETURNING id
	`, clubID, pcID, parentID).Scan(&childID); err != nil {
		t.Fatal(err)
	}

	server := NewServer(config.Config{SessionGraceSeconds: 180}, pool, core.NewMockAdapter())
	if err = server.extendGrantSession(ctx, childID, parentID, "agent-session", clubID, pcID, externalPCID, 3600, "online_payment", "", ""); err != nil {
		t.Fatal(err)
	}

	var duration, remaining int
	var planned time.Time
	if err = pool.QueryRow(ctx, `
		SELECT duration_seconds, planned_ends_at FROM game_access_grants WHERE id=$1
	`, parentID).Scan(&duration, &planned); err != nil {
		t.Fatal(err)
	}
	if duration != 7200 {
		t.Fatalf("root duration = %d, want 7200", duration)
	}
	// MockAdapter returns an Agent-confirmed deadline one hour from now.  The
	// previous bug transformed it into two hours by adding the extension again.
	remaining = int(time.Until(planned).Seconds())
	if remaining < 3500 || remaining > 3700 {
		t.Fatalf("deadline was extended twice: %d seconds remain", remaining)
	}
	var childStatus string
	if err = pool.QueryRow(ctx, `SELECT status FROM game_access_grants WHERE id=$1`, childID).Scan(&childStatus); err != nil {
		t.Fatal(err)
	}
	if childStatus != "extended" {
		t.Fatalf("extension grant status = %q, want extended", childStatus)
	}
}
