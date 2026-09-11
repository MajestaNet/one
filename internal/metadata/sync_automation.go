package metadata

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/MajestaNet/ide/internal/automation"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/packages"
)

// SyncManagedAutomations upserts managed automation rows for a package and
// deactivates registry-removed apiNames (does not hard-delete).
func (s *Service) SyncManagedAutomations(ctx context.Context, packageName string, defs []packages.AutomationDef) error {
	keep := make([]string, 0, len(defs))
	for _, def := range defs {
		if err := s.SyncAutomationManaged(ctx, def, packageName); err != nil {
			return err
		}
		keep = append(keep, def.APIName)
	}
	return s.deactivateRemovedManagedAutomations(ctx, packageName, keep)
}

// SyncAutomationManaged inserts a managed automation (active=true) or refreshes
// product-owned columns. Existing install `active` is never overwritten.
func (s *Service) SyncAutomationManaged(ctx context.Context, def packages.AutomationDef, packageName string) error {
	if err := validateManagedAutomationDef(def, packageName); err != nil {
		return err
	}
	runtime := strings.TrimSpace(def.Runtime)
	if runtime == "" {
		runtime = automation.RuntimeCode
	} else {
		runtime = automation.NormalizeRuntime(runtime)
	}
	execution := automation.NormalizeExecution(def.Execution)
	trigger := strings.ToLower(strings.TrimSpace(def.TriggerEvent))

	var existingOwnership, existingPkg string
	err := s.pool.QueryRow(ctx, `
SELECT COALESCE(ownership, ''), COALESCE(package_name, '')
FROM metadata_automations WHERE api_name=$1`, def.APIName).Scan(&existingOwnership, &existingPkg)
	if errorsIsNoRows(err) {
		_, err = s.pool.Exec(ctx, `
INSERT INTO metadata_automations (
  api_name, label, description, object_api_name, trigger_event, active, condition, actions,
  package_name, ownership, runtime, execution, entry_file, source
)
VALUES ($1,$2,$3,$4,$5,true,NULL,'[]'::jsonb,$6,'managed',$7,$8,$9,$10)`,
			def.APIName, def.Label, def.Description, def.ObjectAPIName, trigger,
			packageName, runtime, execution, nilIfEmpty(def.EntryFile), nilIfEmpty(def.Source))
		if err != nil {
			return fmt.Errorf("insert managed automation %s: %w", def.APIName, err)
		}
		if err := db.EnsureAutomationInAccessCatalog(ctx, s.pool, def.APIName); err != nil {
			return fmt.Errorf("automation access catalog %s: %w", def.APIName, err)
		}
		return nil
	}
	if err != nil {
		return err
	}
	if existingOwnership != "managed" {
		return fmt.Errorf("%w: cannot sync managed automation over customer-owned %s", ErrConflict, def.APIName)
	}

	_, err = s.pool.Exec(ctx, `
UPDATE metadata_automations
SET label=$2, description=$3, object_api_name=$4, trigger_event=$5,
    runtime=$6, execution=$7, entry_file=$8, source=$9, package_name=$10, updated_at=now()
WHERE api_name=$1 AND ownership='managed'`,
		def.APIName, def.Label, def.Description, def.ObjectAPIName, trigger,
		runtime, execution, nilIfEmpty(def.EntryFile), nilIfEmpty(def.Source), packageName)
	if err != nil {
		return fmt.Errorf("update managed automation %s: %w", def.APIName, err)
	}
	return nil
}

func (s *Service) deactivateRemovedManagedAutomations(ctx context.Context, packageName string, keepAPINames []string) error {
	if packageName == "" {
		return fmt.Errorf("%w: packageName is required", ErrValidation)
	}
	_, err := s.pool.Exec(ctx, `
UPDATE metadata_automations
SET active=false, updated_at=now()
WHERE ownership='managed' AND package_name=$1
  AND NOT (api_name = ANY($2::text[]))`,
		packageName, keepAPINames)
	return err
}

func validateManagedAutomationDef(def packages.AutomationDef, packageName string) error {
	if strings.TrimSpace(packageName) == "" {
		return fmt.Errorf("%w: packageName is required", ErrValidation)
	}
	if err := packages.ValidateAutomationAPIName(def.APIName); err != nil {
		return fmt.Errorf("%w: %s", ErrValidation, err.Error())
	}
	if strings.TrimSpace(def.Label) == "" {
		return fmt.Errorf("%w: label is required", ErrValidation)
	}
	if strings.TrimSpace(def.Description) == "" {
		return fmt.Errorf("%w: description is required on managed automations", ErrValidation)
	}
	if err := ValidateAutomationDescription(def.Description); err != nil {
		return err
	}
	if strings.TrimSpace(def.ObjectAPIName) == "" {
		return fmt.Errorf("%w: objectApiName is required", ErrValidation)
	}
	trigger := strings.ToLower(strings.TrimSpace(def.TriggerEvent))
	switch trigger {
	case "create", "update", "delete", "write":
	default:
		return fmt.Errorf("%w: triggerEvent must be create, update, delete, or write", ErrValidation)
	}
	if src := def.Source; src != "" {
		path := def.EntryFile
		if path == "" {
			path = def.APIName
		}
		if err := automation.ValidateSourceImports(path, src); err != nil {
			return fmt.Errorf("%w: %s", ErrValidation, err.Error())
		}
	}
	return nil
}

func nilIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func errorsIsNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
