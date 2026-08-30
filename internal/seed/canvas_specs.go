package seed

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/MajestaNet/ide/internal/canvas"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/packages"
)

// SyncCanvasSpecTemplates upserts managed CanvasSpecs from a module (ADR-018 Phase 5a).
// Ownership stays managed; package_name is the module name. Customer rows with the same
// api_name are never overwritten.
func SyncCanvasSpecTemplates(ctx context.Context, pool *db.Pool, m packages.Module) error {
	if pool == nil || len(m.CanvasSpecTemplates) == 0 {
		return nil
	}
	for _, t := range m.CanvasSpecTemplates {
		if t.APIName == "" || t.Label == "" {
			return fmt.Errorf("canvas template missing apiName/label in package %s", m.Name)
		}
		layout := json.RawMessage(t.LayoutJSON)
		nodes := json.RawMessage(t.NodesJSON)
		bindings := json.RawMessage(`[]`)
		if t.BindingsJSON != "" {
			bindings = json.RawMessage(t.BindingsJSON)
		}
		if err := canvas.ValidateSpecBody(layout, nodes, bindings); err != nil {
			return fmt.Errorf("canvas template %s: %w", t.APIName, err)
		}
		var ownership string
		err := pool.QueryRow(ctx, `SELECT ownership FROM metadata_canvases WHERE api_name=$1`, t.APIName).Scan(&ownership)
		if err == nil && ownership == "custom" {
			// Customer already owns this apiName — leave alone.
			continue
		}
		_, err = pool.Exec(ctx, `
INSERT INTO metadata_canvases (
  api_name, label, description, layout, nodes, data_bindings, active, ownership, package_name
) VALUES ($1,$2,$3,$4::jsonb,$5::jsonb,$6::jsonb,true,'managed',$7)
ON CONFLICT (api_name) DO UPDATE SET
  label=EXCLUDED.label,
  description=EXCLUDED.description,
  layout=EXCLUDED.layout,
  nodes=EXCLUDED.nodes,
  data_bindings=EXCLUDED.data_bindings,
  active=true,
  ownership='managed',
  package_name=EXCLUDED.package_name,
  updated_at=now()
WHERE metadata_canvases.ownership='managed'`,
			t.APIName, t.Label, t.Description,
			string(layout), string(nodes), string(bindings), m.Name,
		)
		if err != nil {
			return fmt.Errorf("sync canvas %s: %w", t.APIName, err)
		}
		if err := db.EnsureToolInAccessCatalog(ctx, pool, t.APIName); err != nil {
			return fmt.Errorf("tool access %s: %w", t.APIName, err)
		}
		if m.Name == "sales" || m.Name == "service" {
			_, err := pool.Exec(ctx, `
INSERT INTO tool_permissions (permission_set_id, tool_api_name, can_open)
SELECT id, $1, true FROM permission_sets WHERE api_name = 'Operate'
ON CONFLICT (permission_set_id, tool_api_name) DO UPDATE SET can_open = true`, t.APIName)
			if err != nil {
				return fmt.Errorf("operate tool access %s: %w", t.APIName, err)
			}
		}
	}
	return nil
}
