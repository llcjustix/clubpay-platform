package httpapi

import (
	"clubpay/internal/authsecurity"
	"clubpay/internal/config"
	"clubpay/internal/core"
	"context"
	"testing"
)

func TestEdgeSnapshotPreservesCloudSuperadmin(t *testing.T) {
	pool, s, club, pc := snapshotStorageFixture(t)
	ctx := context.Background()
	var id string
	if err := pool.QueryRow(ctx, `SELECT id FROM users WHERE global_role='super_admin' ORDER BY id LIMIT 1`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	payload, err := s.edgeSnapshotData(ctx, club, true)
	if err != nil {
		t.Fatal(err)
	}
	expected := authsecurity.HashPassword("disposable-regression-cloud-password")
	if _, err = pool.Exec(ctx, `UPDATE users SET password_hash=$1,name='Cloud authoritative admin',status='active' WHERE id=$2`, expected, id); err != nil {
		t.Fatal(err)
	}
	for _, u := range payload["users"].([]map[string]any) {
		if u["id"] == id {
			u["global_role"] = ""
			u["name"] = "stale edge admin"
			u["status"] = "disabled"
		}
	}
	for _, p := range payload["pcs"].([]map[string]any) {
		if p["id"] == pc {
			p["status_cache"] = "sleeping"
		}
	}
	if err = s.applyEdgeSnapshotData(ctx, club, payload); err != nil {
		t.Fatal(err)
	}
	var hash, name, status, role, pcStatus string
	if err = pool.QueryRow(ctx, `SELECT password_hash,name,status,global_role FROM users WHERE id=$1`, id).Scan(&hash, &name, &status, &role); err != nil {
		t.Fatal(err)
	}
	if hash != expected || name != "Cloud authoritative admin" || status != "active" || role != "super_admin" {
		t.Fatal("stale edge snapshot replaced Cloud superadmin credentials or identity")
	}
	if err = pool.QueryRow(ctx, `SELECT status_cache FROM pc_refs WHERE id=$1`, pc).Scan(&pcStatus); err != nil || pcStatus != "sleeping" {
		t.Fatalf("operational PC sync lost: %v", err)
	}
}

func TestEdgeSnapshotLocalImportsCanonicalSuperadmin(t *testing.T) {
	pool, cloud, club, _ := snapshotStorageFixture(t)
	ctx := context.Background()
	payload, err := cloud.edgeSnapshotData(ctx, club, true)
	if err != nil {
		t.Fatal(err)
	}
	var id string
	expected := authsecurity.HashPassword("disposable-regression-canonical-password")
	for _, u := range payload["users"].([]map[string]any) {
		if u["global_role"] == "super_admin" {
			id = u["id"].(string)
			u["password_hash"] = expected
		}
	}
	local := NewServer(config.Config{NodeMode: "edge", AppEnv: "production"}, pool, core.NewMockAdapter())
	if err = local.applyEdgeSnapshotData(ctx, club, payload); err != nil {
		t.Fatal(err)
	}
	var hash string
	if err = pool.QueryRow(ctx, `SELECT password_hash FROM users WHERE id=$1`, id).Scan(&hash); err != nil || hash != expected {
		t.Fatalf("local canonical authentication import failed: %v", err)
	}
}

func TestEdgeSnapshotRejectsGlobalAdminInjection(t *testing.T) {
	for _, sameEmail := range []bool{false, true} {
		t.Run(map[bool]string{false: "new_global_admin", true: "canonical_email_other_id"}[sameEmail], func(t *testing.T) {
			pool, s, club, _ := snapshotStorageFixture(t)
			ctx := context.Background()
			payload, err := s.edgeSnapshotData(ctx, club, true)
			if err != nil {
				t.Fatal(err)
			}
			var id string
			if err = pool.QueryRow(ctx, `SELECT gen_random_uuid()`).Scan(&id); err != nil {
				t.Fatal(err)
			}
			user := map[string]any{"id": id, "name": "Snapshot injection fixture", "email": "injection-fixture@example.invalid", "global_role": "super_admin", "password_hash": "fixture-only"}
			if sameEmail {
				user["email"] = "SUPERADMIN@CLUBPAY.LOCAL"
				user["global_role"] = ""
			}
			payload["users"] = append(payload["users"].([]map[string]any), user)
			payload["user_club_roles"] = append(payload["user_club_roles"].([]map[string]any), map[string]any{"user_id": id, "club_id": club, "role": "owner"})
			if err = s.applyEdgeSnapshotData(ctx, club, payload); err != nil {
				t.Fatal(err)
			}
			var count int
			if err = pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE id=$1`, id).Scan(&count); err != nil || count != 0 {
				t.Fatalf("snapshot injected global identity: count=%d err=%v", count, err)
			}
		})
	}
}
