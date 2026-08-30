package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/MajestaNet/ide/internal/actions"
	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/deploy"
	"github.com/MajestaNet/ide/internal/metadata"
)

func requireCapability(ctx context.Context, deps Deps, actor *authz.Actor, cap string) error {
	if actor == nil {
		return fmt.Errorf("%w: authentication required", ErrUnauthorized)
	}
	if authz.HasAdminPrivilege(actor) {
		return nil
	}
	if deps.SystemAz == nil {
		return fmt.Errorf("%w: capability %s required", ErrForbidden, cap)
	}
	if err := deps.SystemAz.AssertCapability(ctx, actor, cap); err != nil {
		return fmt.Errorf("%w: capability %s required", ErrForbidden, cap)
	}
	return nil
}

func requireDeployScope(actor *authz.Actor) error {
	if actor == nil || !actor.HasScope(authz.ScopeDeploy) {
		return fmt.Errorf("%w: scope %s required", ErrForbidden, authz.ScopeDeploy)
	}
	return nil
}

func mapDomainErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, deploy.ErrBusy) {
		return err
	}
	var ae *actions.Error
	if errors.As(err, &ae) {
		switch ae.Status {
		case 401:
			return fmt.Errorf("%w: %s", ErrUnauthorized, ae.Error())
		case 403:
			return fmt.Errorf("%w: %s", ErrForbidden, ae.Error())
		case 404:
			return fmt.Errorf("%w: %s", ErrNotFound, ae.Error())
		default:
			return err
		}
	}
	switch {
	case errors.Is(err, authz.ErrForbidden), errors.Is(err, metadata.ErrForbidden):
		return fmt.Errorf("%w: %v", ErrForbidden, err)
	case errors.Is(err, metadata.ErrNotFound):
		return fmt.Errorf("%w: %v", ErrNotFound, err)
	default:
		return err
	}
}

func invokeAction(ctx context.Context, deps Deps, actor *authz.Actor, apiName string, input map[string]any) (any, error) {
	if err := requireScope(actor, authz.ScopeClient); err != nil {
		return nil, err
	}
	apiName = strings.TrimSpace(apiName)
	if apiName == "" {
		return nil, fmt.Errorf("apiName required")
	}
	if deps.Actions == nil {
		return nil, fmt.Errorf("platform actions unavailable")
	}
	if input == nil {
		input = map[string]any{}
	}
	out, err := deps.Actions.Invoke(ctx, actor, apiName, input)
	if err != nil {
		return nil, mapDomainErr(err)
	}
	return out, nil
}

func invokeSkill(ctx context.Context, deps Deps, actor *authz.Actor, apiName, playbookAPIName string, input map[string]any) (any, error) {
	if err := requireScope(actor, authz.ScopeClient); err != nil {
		return nil, err
	}
	apiName = strings.TrimSpace(apiName)
	playbookAPIName = strings.TrimSpace(playbookAPIName)
	if apiName == "" {
		return nil, fmt.Errorf("apiName required")
	}
	if actor != nil && strings.EqualFold(actor.PrincipalType, "agent") && playbookAPIName == "" {
		return nil, fmt.Errorf("%w: playbookApiName required for agent principals", ErrForbidden)
	}
	if playbookAPIName != "" {
		ok, err := playbookAllowsSkill(ctx, deps, playbookAPIName, apiName)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("%w: skill %s is not in allowedSkills for %s", ErrForbidden, apiName, playbookAPIName)
		}
	}
	if deps.AutomationAz == nil {
		return nil, fmt.Errorf("automation authorization not configured")
	}
	if err := deps.AutomationAz.AssertCanRunAutomation(ctx, actor, apiName); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrForbidden, err)
	}
	if deps.Pool == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	if input == nil {
		input = map[string]any{}
	}

	var defID, objectAPIName, runtime, execution string
	err := deps.Pool.QueryRow(ctx, `
SELECT id::text, COALESCE(object_api_name, ''), COALESCE(runtime, 'actions'), COALESCE(execution, 'async')
FROM metadata_automations WHERE api_name=$1 AND active=true`, apiName).
		Scan(&defID, &objectAPIName, &runtime, &execution)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: automation not found or inactive", ErrNotFound)
		}
		return nil, err
	}
	actorID := ""
	if actor != nil {
		actorID = actor.ID
	}
	payload, err := json.Marshal(map[string]any{
		"automationId":  defID,
		"apiName":       apiName,
		"objectApiName": objectAPIName,
		"action":        "manual",
		"actorId":       actorID,
		"input":         input,
		"runtime":       runtime,
		"execution":     execution,
	})
	if err != nil {
		return nil, err
	}
	var jobID, status string
	var createdAt time.Time
	err = deps.Pool.QueryRow(ctx, `
INSERT INTO jobs (job_type, payload)
VALUES ('automation.run', $1::jsonb)
RETURNING id::text, status, created_at`, string(payload)).Scan(&jobID, &status, &createdAt)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id":                jobID,
		"automationApiName": apiName,
		"status":            status,
		"execution":         execution,
		"input":             input,
		"createdAt":         createdAt,
	}, nil
}

func playbookAllowsSkill(ctx context.Context, deps Deps, playbook, skill string) (bool, error) {
	if deps.Pool == nil {
		return false, fmt.Errorf("database unavailable")
	}
	var raw []byte
	err := deps.Pool.QueryRow(ctx, `
SELECT COALESCE(allowed_skills, '[]'::jsonb)
FROM agent_playbooks WHERE api_name=$1 AND active=true`, playbook).Scan(&raw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, fmt.Errorf("%w: playbook %s not found", ErrNotFound, playbook)
		}
		return false, err
	}
	var skills []string
	_ = json.Unmarshal(raw, &skills)
	for _, s := range skills {
		if strings.TrimSpace(s) == skill {
			return true, nil
		}
	}
	return false, nil
}

func listObjectsMetadata(ctx context.Context, deps Deps, actor *authz.Actor) (any, error) {
	if err := requireScope(actor, authz.ScopeMetadata); err != nil {
		return nil, err
	}
	if deps.Meta == nil {
		return nil, fmt.Errorf("metadata unavailable")
	}
	objs, err := deps.Meta.ListObjects(ctx)
	if err != nil {
		return nil, mapDomainErr(err)
	}
	return map[string]any{"objects": objs}, nil
}

func upsertObject(ctx context.Context, deps Deps, actor *authz.Actor, args map[string]any) (any, error) {
	if err := requireScope(actor, authz.ScopeMetadata); err != nil {
		return nil, err
	}
	if err := requireCapability(ctx, deps, actor, authz.CapMetadataBuild); err != nil {
		return nil, err
	}
	if deps.Meta == nil {
		return nil, fmt.Errorf("metadata unavailable")
	}
	raw, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	var body metadata.ObjectDefinition
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, fmt.Errorf("invalid object payload")
	}
	if strings.TrimSpace(body.APIName) == "" || strings.TrimSpace(body.Label) == "" {
		return nil, fmt.Errorf("apiName and label required")
	}
	if body.PluralLabel == "" {
		body.PluralLabel = body.Label + "s"
	}
	existing, err := deps.Meta.GetObject(ctx, body.APIName)
	if err != nil && !errors.Is(err, metadata.ErrNotFound) {
		return nil, mapDomainErr(err)
	}
	if errors.Is(err, metadata.ErrNotFound) || existing.APIName == "" {
		obj, ierr := deps.Meta.InsertObject(ctx, body, metadata.CreateOptions{})
		if ierr != nil {
			return nil, mapDomainErr(ierr)
		}
		return obj, nil
	}
	obj, uerr := deps.Meta.UpdateObject(ctx, body.APIName, body)
	if uerr != nil {
		return nil, mapDomainErr(uerr)
	}
	return obj, nil
}

func upsertField(ctx context.Context, deps Deps, actor *authz.Actor, args map[string]any) (any, error) {
	if err := requireScope(actor, authz.ScopeMetadata); err != nil {
		return nil, err
	}
	if err := requireCapability(ctx, deps, actor, authz.CapMetadataBuild); err != nil {
		return nil, err
	}
	if deps.Meta == nil {
		return nil, fmt.Errorf("metadata unavailable")
	}
	objectAPIName := strArg(args, "objectApiName")
	apiName := strArg(args, "apiName")
	if objectAPIName == "" || apiName == "" {
		return nil, fmt.Errorf("objectApiName and apiName required")
	}
	raw, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	var body metadata.FieldDefinition
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, fmt.Errorf("invalid field payload")
	}
	body.ObjectAPIName = objectAPIName
	body.APIName = apiName
	existing, err := deps.Meta.GetField(ctx, objectAPIName, apiName)
	if err != nil && !errors.Is(err, metadata.ErrNotFound) {
		return nil, mapDomainErr(err)
	}
	if errors.Is(err, metadata.ErrNotFound) || existing.APIName == "" {
		if body.Label == "" || body.FieldType == "" {
			return nil, fmt.Errorf("label and fieldType required to create a field")
		}
		_, filterableSet := args["filterable"]
		_, sortableSet := args["sortable"]
		_, indexedSet := args["indexed"]
		metadata.ApplyFieldTypeDefaults(&body, filterableSet, sortableSet, indexedSet)
		field, ierr := deps.Meta.InsertField(ctx, body, metadata.CreateOptions{})
		if ierr != nil {
			return nil, mapDomainErr(ierr)
		}
		return field, nil
	}
	patch := fieldPatchFromArgs(args)
	field, uerr := deps.Meta.UpdateField(ctx, objectAPIName, apiName, patch)
	if uerr != nil {
		return nil, mapDomainErr(uerr)
	}
	return field, nil
}

func fieldPatchFromArgs(args map[string]any) metadata.FieldPatch {
	var patch metadata.FieldPatch
	if v, ok := args["label"].(string); ok {
		patch.Label = &v
	}
	if v, ok := args["required"].(bool); ok {
		patch.Required = &v
	}
	if v, ok := args["uniqueField"].(bool); ok {
		patch.UniqueField = &v
	}
	if v, ok := args["externalId"].(bool); ok {
		patch.ExternalID = &v
	}
	if v, ok := args["indexed"].(bool); ok {
		patch.Indexed = &v
	}
	if v, ok := args["filterable"].(bool); ok {
		patch.Filterable = &v
	}
	if v, ok := args["sortable"].(bool); ok {
		patch.Sortable = &v
	}
	if v, ok := args["searchable"].(bool); ok {
		patch.Searchable = &v
	}
	return patch
}

func orgValidate(ctx context.Context, deps Deps, actor *authz.Actor, args map[string]any) (any, error) {
	if err := requireDeployScope(actor); err != nil {
		return nil, err
	}
	if err := requireCapability(ctx, deps, actor, authz.CapDeployPromote); err != nil {
		return nil, err
	}
	if deps.Deploy == nil {
		return nil, fmt.Errorf("deploy engine unavailable")
	}
	var createdBy *string
	if actor != nil && actor.ID != "" {
		createdBy = &actor.ID
	}
	label := strArg(args, "label")
	var labelPtr *string
	if label != "" {
		labelPtr = &label
	}
	result, queued, err := deps.Deploy.EnqueueValidate(ctx, struct {
		Artifact  any
		BundleID  string
		Label     *string
		CreatedBy *string
	}{
		Artifact:  args["artifact"],
		BundleID:  strArg(args, "bundleId"),
		Label:     labelPtr,
		CreatedBy: createdBy,
	}, 0)
	if err != nil {
		return nil, mapDomainErr(err)
	}
	if queued != nil {
		return queued, nil
	}
	return result, nil
}

func orgDeploy(ctx context.Context, deps Deps, actor *authz.Actor, args map[string]any) (any, error) {
	if err := requireDeployScope(actor); err != nil {
		return nil, err
	}
	if err := requireCapability(ctx, deps, actor, authz.CapDeployPromote); err != nil {
		return nil, err
	}
	if deps.Deploy == nil {
		return nil, fmt.Errorf("deploy engine unavailable")
	}
	bundleID := strArg(args, "bundleId")
	if bundleID == "" {
		return nil, fmt.Errorf("bundleId required")
	}
	dryRun, _ := args["dryRun"].(bool)
	var createdBy *string
	if actor != nil && actor.ID != "" {
		createdBy = &actor.ID
	}
	result, queued, err := deps.Deploy.EnqueuePromote(ctx, struct {
		BundleID  string
		DryRun    bool
		CreatedBy *string
	}{BundleID: bundleID, DryRun: dryRun, CreatedBy: createdBy}, true)
	if err != nil {
		return nil, mapDomainErr(err)
	}
	if queued != nil {
		return queued, nil
	}
	return result, nil
}

func packArtifact(ctx context.Context, deps Deps, actor *authz.Actor, args map[string]any) (any, error) {
	if err := requireDeployScope(actor); err != nil {
		return nil, err
	}
	if err := requireCapability(ctx, deps, actor, authz.CapDeployPromote); err != nil {
		return nil, err
	}
	if deps.Deploy == nil {
		return nil, fmt.Errorf("deploy engine unavailable")
	}
	if args["artifact"] == nil {
		return nil, fmt.Errorf("artifact required")
	}
	label := strArg(args, "label")
	var labelPtr *string
	if label != "" {
		labelPtr = &label
	}
	var createdBy *string
	if actor != nil && actor.ID != "" {
		createdBy = &actor.ID
	}
	row, err := deps.Deploy.CreateBundleFromArtifact(ctx, struct {
		Artifact  any
		Label     *string
		CreatedBy *string
		Origin    string
		Signature *string
	}{
		Artifact:  args["artifact"],
		Label:     labelPtr,
		CreatedBy: createdBy,
		Origin:    "customer-package",
	})
	if err != nil {
		return nil, mapDomainErr(err)
	}
	return row, nil
}

func orgRetrieve(ctx context.Context, deps Deps, actor *authz.Actor) (any, error) {
	if err := requireDeployScope(actor); err != nil {
		return nil, err
	}
	if deps.Deploy == nil {
		return nil, fmt.Errorf("deploy engine unavailable")
	}
	row, err := deps.Deploy.CreateBundleFromSnapshot(ctx, struct {
		Label               *string
		CreatedBy           *string
		ProductVersionRange string
	}{})
	if err != nil {
		return nil, mapDomainErr(err)
	}
	return row, nil
}

func installVersion(deps Deps, actor *authz.Actor) (any, error) {
	if actor == nil {
		return nil, fmt.Errorf("%w: authentication required", ErrUnauthorized)
	}
	return map[string]any{
		"version":        deps.Version,
		"productVersion": deps.ProductVersion,
		"runtime":        "go",
	}, nil
}
