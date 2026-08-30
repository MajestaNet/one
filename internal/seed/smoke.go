package seed

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/MajestaNet/ide/internal/db"
)

// PlatformSmokeAPIName is the managed product post-upgrade smoke suite (ADR-007).
const PlatformSmokeAPIName = "PlatformSmoke"

// EnsurePlatformSmokeSuite upserts the managed PlatformSmoke Deploy test suite.
// Customers may additionally register PostUpgradeSmoke for custom gates.
func EnsurePlatformSmokeSuite(ctx context.Context, pool *db.Pool) error {
	steps := []map[string]any{
		{"type": "objectExists", "objectApiName": "Account"},
		{"type": "fieldExists", "objectApiName": "Account", "fieldApiName": "Name"},
		{"type": "objectExists", "objectApiName": "Contact"},
		{"type": "fieldExists", "objectApiName": "Contact", "fieldApiName": "LastName"},
		{"type": "fieldExists", "objectApiName": "Contact", "fieldApiName": "AccountId"},
		{"type": "fieldExists", "objectApiName": "Account", "fieldApiName": "AccountNumber"},
		{"type": "fieldExists", "objectApiName": "Contact", "fieldApiName": "JobTitle"},
	}
	stepsJSON, err := json.Marshal(steps)
	if err != nil {
		return err
	}
	desc := "Product post-upgrade smoke (managed). Run via Deploy tests after ECS image rolls."
	_, err = pool.Exec(ctx, `
INSERT INTO customer_tests (api_name, label, description, active, steps, package_name, ownership)
VALUES ($1, $2, $3, true, $4::jsonb, 'platform', 'managed')
ON CONFLICT (api_name) DO UPDATE SET
  label = EXCLUDED.label,
  description = EXCLUDED.description,
  active = true,
  steps = EXCLUDED.steps,
  package_name = 'platform',
  ownership = 'managed',
  updated_at = now()
WHERE customer_tests.ownership = 'managed' OR customer_tests.api_name = $1`,
		PlatformSmokeAPIName,
		"Platform smoke",
		desc,
		string(stepsJSON),
	)
	if err != nil {
		return fmt.Errorf("upsert PlatformSmoke: %w", err)
	}
	return nil
}
