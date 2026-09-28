package authsecurity

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	clubdb "clubpay/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPublicDemoHashes(t *testing.T) {
	for _, hash := range []string{publicSuperadminHash, publicOwnerHash, publicManagerHash} {
		if !IsPublicDemoHash(hash) {
			t.Fatalf("public demo hash %q was not blocked", hash)
		}
	}
	if IsPublicDemoHash(HashPassword("a different password")) {
		t.Fatal("unrelated password was blocked")
	}
}

func TestProductionDemoRotationSurvivesRestart(t *testing.T) {
	databaseURL := os.Getenv("MOBILE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set MOBILE_TEST_DATABASE_URL for PostgreSQL integration tests")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("auth_security_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, `DROP SCHEMA `+schema+` CASCADE`)
	cfg, err := pgxpool.ParseConfig(databaseURL)
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
	var superID string
	if err = pool.QueryRow(ctx, `SELECT id FROM users WHERE email = 'superadmin@clubpay.local'`).Scan(&superID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO auth_sessions (user_id, token_hash, expires_at) VALUES ($1, 'old-demo-session', now() + interval '1 day')`, superID); err != nil {
		t.Fatal(err)
	}
	if _, _, err = HardenProductionDemoAccounts(ctx, pool, "production", "cloud", ""); err == nil {
		t.Fatal("production cloud started with a public superadmin password and no replacement")
	}
	if rotated, _, err := HardenProductionDemoAccounts(ctx, pool, "development", "cloud", ""); err != nil || rotated != 0 {
		t.Fatalf("development seed changed: rotated=%d err=%v", rotated, err)
	}
	password := "a-long-unique-production-password-for-testing"
	rotated, revoked, err := HardenProductionDemoAccounts(ctx, pool, "production", "cloud", password)
	if err != nil || rotated != 3 || revoked != 1 {
		t.Fatalf("rotation: rotated=%d revoked=%d err=%v", rotated, revoked, err)
	}
	var superHash string
	if err = pool.QueryRow(ctx, `SELECT password_hash FROM users WHERE id = $1`, superID).Scan(&superHash); err != nil {
		t.Fatal(err)
	}
	if superHash != HashPassword(password) || IsPublicDemoHash(superHash) {
		t.Fatal("superadmin was not rotated to the private bootstrap password")
	}
	var remaining int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE password_hash = ANY($1)`, []string{publicSuperadminHash, publicOwnerHash, publicManagerHash}).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("public demo hashes remain: count=%d err=%v", remaining, err)
	}
	if err = clubdb.RunMigrations(ctx, pool, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT password_hash FROM users WHERE id = $1`, superID).Scan(&superHash); err != nil || superHash != HashPassword(password) {
		t.Fatalf("restart restored the public password: hash=%q err=%v", superHash, err)
	}
	rotated, revoked, err = HardenProductionDemoAccounts(ctx, pool, "production", "cloud", "")
	if err != nil || rotated != 0 || revoked != 0 {
		t.Fatalf("second startup changed credentials: rotated=%d revoked=%d err=%v", rotated, revoked, err)
	}
}
