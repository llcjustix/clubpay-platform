package authsecurity

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// These hashes are from the public development seed. Production must never
	// accept them, including when an older Controller syncs a seeded account.
	publicSuperadminHash = "14fc0a272b7f9316762fee996f8a553f76acdb155113f7e22020ba25c87e687c"
	publicOwnerHash      = "f1aacffd5fc4cd7e018dd516d1e4b2b29e8618292024e3d3995456b588f769c8"
	publicManagerHash    = "a3c75316794fb37d045f6b84db41904b645ce82593b439ace3501f86f755e6ec"
)

func HashPassword(password string) string {
	sum := sha256.Sum256([]byte("clubpay-demo-salt:" + password))
	return hex.EncodeToString(sum[:])
}

func IsPublicDemoHash(hash string) bool {
	switch hash {
	case publicSuperadminHash, publicOwnerHash, publicManagerHash:
		return true
	default:
		return false
	}
}

// HardenProductionDemoAccounts replaces the public seed passwords and revokes
// their existing sessions. It is safe to run at every startup: only accounts
// still using their original seed hash are changed. Development seeds remain
// available outside production.
func HardenProductionDemoAccounts(ctx context.Context, pool *pgxpool.Pool, appEnv, nodeMode, bootstrapPassword string) (int, int, error) {
	if !strings.EqualFold(appEnv, "production") {
		return 0, 0, nil
	}
	if strings.EqualFold(nodeMode, "cloud") {
		var needsBootstrap bool
		err := pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM users
				WHERE lower(COALESCE(email, '')) = 'superadmin@clubpay.local'
				  AND password_hash = $1
			)
		`, publicSuperadminHash).Scan(&needsBootstrap)
		if err != nil {
			return 0, 0, err
		}
		if needsBootstrap && len(bootstrapPassword) < 32 {
			return 0, 0, fmt.Errorf("SUPERADMIN_BOOTSTRAP_PASSWORD must be at least 32 characters before production can start")
		}
	}
	replacementHash := ""
	if bootstrapPassword != "" {
		replacementHash = HashPassword(bootstrapPassword)
	}
	var rotated, revoked int
	err := pool.QueryRow(ctx, `
		WITH rotated AS (
			UPDATE users
			SET password_hash = CASE
				WHEN password_hash = $1 AND $4 <> '' THEN $4
				ELSE encode(digest('clubpay-demo-salt:' || encode(gen_random_bytes(32), 'hex'), 'sha256'), 'hex')
			END,
			updated_at = now()
			WHERE (lower(COALESCE(email, '')) = 'superadmin@clubpay.local' AND password_hash = $1)
			   OR (lower(COALESCE(email, '')) = 'owner@clubpay.local' AND password_hash = $2)
			   OR (lower(COALESCE(email, '')) = 'admin@clubpay.local' AND password_hash = $3)
			RETURNING id
		), revoked AS (
			DELETE FROM auth_sessions WHERE user_id IN (SELECT id FROM rotated) RETURNING id
		)
		SELECT (SELECT count(*) FROM rotated), (SELECT count(*) FROM revoked)
	`, publicSuperadminHash, publicOwnerHash, publicManagerHash, replacementHash).Scan(&rotated, &revoked)
	return rotated, revoked, err
}
