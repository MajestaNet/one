package db

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// DeviceCertificate is an enrolled IDE/device cert for requireDeviceCert mode.
type DeviceCertificate struct {
	ID             string
	UserID         string
	DeviceID       string
	Label          string
	Fingerprint    string
	CertificatePEM string
	RevokedAt      *time.Time
	ExpiresAt      time.Time
	CreatedAt      time.Time
}

// DeviceCertActive reports whether device_id is an active enrollment for the user.
func DeviceCertActive(ctx context.Context, pool *Pool, userID, deviceID string) (bool, error) {
	if pool == nil || userID == "" || deviceID == "" {
		return false, nil
	}
	var ok bool
	err := pool.QueryRow(ctx, `
SELECT EXISTS(
  SELECT 1 FROM device_certificates
  WHERE user_id = $1::uuid AND device_id = $2
    AND revoked_at IS NULL AND expires_at > now()
)`, userID, strings.TrimSpace(deviceID)).Scan(&ok)
	return ok, err
}

// UpsertDeviceCertificate stores or refreshes an enrollment.
func UpsertDeviceCertificate(ctx context.Context, pool *Pool, userID, deviceID, label, certPEM string, expiresAt time.Time) (*DeviceCertificate, error) {
	fp := fingerprintPEM(certPEM)
	row := pool.QueryRow(ctx, `
INSERT INTO device_certificates (user_id, device_id, label, fingerprint, certificate_pem, expires_at)
VALUES ($1::uuid, $2, $3, $4, $5, $6)
ON CONFLICT (user_id, device_id) DO UPDATE SET
  label = EXCLUDED.label,
  fingerprint = EXCLUDED.fingerprint,
  certificate_pem = EXCLUDED.certificate_pem,
  expires_at = EXCLUDED.expires_at,
  revoked_at = NULL
RETURNING id::text, user_id::text, device_id, label, fingerprint, certificate_pem, revoked_at, expires_at, created_at`,
		userID, strings.TrimSpace(deviceID), label, fp, certPEM, expiresAt)
	return scanDeviceCert(row)
}

// RevokeDeviceCertificate soft-revokes a device enrollment.
func RevokeDeviceCertificate(ctx context.Context, pool *Pool, userID, deviceID string) error {
	tag, err := pool.Exec(ctx, `
UPDATE device_certificates SET revoked_at = now()
WHERE user_id = $1::uuid AND device_id = $2 AND revoked_at IS NULL`,
		userID, strings.TrimSpace(deviceID))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListDeviceCertificates lists enrollments for a user.
func ListDeviceCertificates(ctx context.Context, pool *Pool, userID string) ([]DeviceCertificate, error) {
	rows, err := pool.Query(ctx, `
SELECT id::text, user_id::text, device_id, label, fingerprint, certificate_pem, revoked_at, expires_at, created_at
FROM device_certificates WHERE user_id = $1::uuid ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DeviceCertificate
	for rows.Next() {
		c, err := scanDeviceCert(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

func scanDeviceCert(row pgx.Row) (*DeviceCertificate, error) {
	var c DeviceCertificate
	err := row.Scan(&c.ID, &c.UserID, &c.DeviceID, &c.Label, &c.Fingerprint, &c.CertificatePEM, &c.RevokedAt, &c.ExpiresAt, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func fingerprintPEM(pem string) string {
	sum := sha256.Sum256([]byte(pem))
	return hex.EncodeToString(sum[:])
}

// IntegrationAPINameForPrincipal returns the Connected App api_name for a principal, if any.
func IntegrationAPINameForPrincipal(ctx context.Context, pool *Pool, principalID string) (string, error) {
	var name string
	err := pool.QueryRow(ctx, `
SELECT api_name FROM integration_configs WHERE principal_id = $1::uuid AND is_active = true
ORDER BY ownership = 'managed' DESC, api_name ASC LIMIT 1`, principalID).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return name, err
}

// IntegrationAPINameForCognitoClient maps an identity provider app client id (OIDC aud /
// identity_links.subject) to the Connected App api_name for the linked service principal.
func IntegrationAPINameForCognitoClient(ctx context.Context, pool *Pool, clientID string) (string, error) {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return "", ErrNotFound
	}
	var name string
	err := pool.QueryRow(ctx, `
SELECT ic.api_name
FROM identity_links il
JOIN integration_configs ic ON ic.principal_id = il.user_id
WHERE il.subject = $1 AND ic.is_active = true
ORDER BY ic.ownership = 'managed' DESC, ic.api_name ASC
LIMIT 1`, clientID).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return name, err
}
