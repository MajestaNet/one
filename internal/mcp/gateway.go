// Package mcp is an install-local MCP adapter over Majesta One HTTP families (ADR-010).
// It invents no AuthZ: tools succeed only when the caller's principal already has
// the required family scopes and object/capability grants.
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/dataengine"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/deploy"
	"github.com/MajestaNet/ide/internal/metadata"
)

// ErrUnauthorized is returned when the principal lacks a required family scope.
var ErrUnauthorized = errors.New("unauthorized")

// ErrForbidden is returned when object/FLS AuthZ denies the action.
var ErrForbidden = errors.New("forbidden")

// ErrNotFound is returned for missing tools or records.
var ErrNotFound = errors.New("not found")

// Deps are in-process services used by tool handlers (no duplicate business logic).
type Deps struct {
	Meta           *metadata.Service
	Data           *dataengine.Service
	Pool           *db.Pool
	ObjectAz       *authz.ObjectAuthz
	FieldAz        *authz.FieldAuthz
	RecordAccess   *authz.RecordAccessEvaluator
	Actions        ActionInvoker
	Deploy         *deploy.DeployEngine
	SystemAz       *authz.SystemAuthz
	AutomationAz   *authz.AutomationAuthz
	ProductVersion string
	Version        string
}

// ActionInvoker is the platform-action surface used by invoke_action.
type ActionInvoker interface {
	Invoke(ctx context.Context, actor *authz.Actor, apiName string, input map[string]any) (map[string]any, error)
}

// ToolDesc describes one MCP tool.
type ToolDesc struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// ListTools returns the v1 tool catalog.
func ListTools() []ToolDesc {
	authNote := " AuthZ is enforced by Majesta One; 401/403 means the principal lacks grants."
	return []ToolDesc{
		{
			Name:        "describe_global",
			Description: "List objects via Client GET /client/v1/describe." + authNote,
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			Name:        "describe_object",
			Description: "Describe one object via Client GET /client/v1/describe/{object}." + authNote,
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"object": map[string]any{"type": "string"},
				},
				"required": []string{"object"},
			},
		},
		{
			Name:        "query",
			Description: "Run a Client query via POST /client/v1/query (body: object, filters, sort, limit, cursor)." + authNote,
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"object":  map[string]any{"type": "string"},
					"filters": map[string]any{"type": "array"},
					"sort":    map[string]any{"type": "array"},
					"limit":   map[string]any{"type": "integer"},
					"cursor":  map[string]any{"type": "string"},
				},
				"required": []string{"object"},
			},
		},
		{
			Name:        "search",
			Description: "Cross-object find via POST /client/v1/search (body: q, optional objects[], limit)." + authNote,
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"q":       map[string]any{"type": "string"},
					"objects": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"limit":   map[string]any{"type": "integer"},
				},
				"required": []string{"q"},
			},
		},
		{
			Name:        "get_record",
			Description: "Get a record via Client GET /client/v1/sobjects/{object}/{id}." + authNote,
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"object": map[string]any{"type": "string"},
					"id":     map[string]any{"type": "string"},
				},
				"required": []string{"object", "id"},
			},
		},
		{
			Name:        "create_record",
			Description: "Create a record via Client POST /client/v1/sobjects/{object}." + authNote,
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"object": map[string]any{"type": "string"},
					"data":   map[string]any{"type": "object"},
				},
				"required": []string{"object", "data"},
			},
		},
		{
			Name:        "update_record",
			Description: "Update a record via Client PATCH /client/v1/sobjects/{object}/{id}." + authNote,
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"object": map[string]any{"type": "string"},
					"id":     map[string]any{"type": "string"},
					"data":   map[string]any{"type": "object"},
				},
				"required": []string{"object", "id", "data"},
			},
		},
		{
			Name:        "get_object_metadata",
			Description: "Inspect Metadata definition via GET /metadata/v1/objects/{apiName} (requires scope:metadata)." + authNote,
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"apiName": map[string]any{"type": "string"},
				},
				"required": []string{"apiName"},
			},
		},
		{
			Name:        "list_agent_specs",
			Description: "List AgentSpecs via Metadata GET /metadata/v1/agents/playbooks (requires scope:metadata)." + authNote,
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			Name:        "create_agent_run",
			Description: "Create an agent run via Client POST /client/v1/agents/runs." + authNote,
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"playbookApiName": map[string]any{"type": "string"},
					"goal":            map[string]any{"type": "string"},
					"input":           map[string]any{"type": "object"},
					"dryRun":          map[string]any{"type": "boolean"},
					"approved":        map[string]any{"type": "boolean"},
				},
			},
		},
		{
			Name:        "get_agent_run",
			Description: "Get an agent run via Client GET /client/v1/agents/runs/{id}." + authNote,
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id": map[string]any{"type": "string"},
				},
				"required": []string{"id"},
			},
		},
		{
			Name:        "invoke_action",
			Description: "Invoke a platform action via POST /client/v1/actions/{apiName} (requires scope:client)." + authNote,
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"apiName": map[string]any{"type": "string"},
					"input":   map[string]any{"type": "object"},
				},
				"required": []string{"apiName"},
			},
		},
		{
			Name:        "invoke_skill",
			Description: "Invoke a named automation via POST /client/v1/automations/{apiName}/runs. Agents must pass playbookApiName; skill must be in allowedSkills and PS canRun." + authNote,
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"apiName":         map[string]any{"type": "string"},
					"input":           map[string]any{"type": "object"},
					"playbookApiName": map[string]any{"type": "string"},
				},
				"required": []string{"apiName"},
			},
		},
		{
			Name:        "list_objects_metadata",
			Description: "List object definitions via GET /metadata/v1/objects (requires scope:metadata)." + authNote,
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			Name:        "upsert_object",
			Description: "Create or update a customer object via POST/PATCH /metadata/v1/objects (requires scope:metadata and metadata.build)." + authNote,
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"apiName":     map[string]any{"type": "string"},
					"label":       map[string]any{"type": "string"},
					"pluralLabel": map[string]any{"type": "string"},
					"storageMode": map[string]any{"type": "string"},
					"features":    map[string]any{"type": "object"},
				},
				"required": []string{"apiName", "label"},
			},
		},
		{
			Name:        "upsert_field",
			Description: "Create or update a customer field via POST/PATCH /metadata/v1/fields (requires scope:metadata and metadata.build)." + authNote,
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"objectApiName": map[string]any{"type": "string"},
					"apiName":       map[string]any{"type": "string"},
					"label":         map[string]any{"type": "string"},
					"fieldType":     map[string]any{"type": "string"},
					"required":      map[string]any{"type": "boolean"},
				},
				"required": []string{"objectApiName", "apiName"},
			},
		},
		{
			Name:        "org_validate",
			Description: "Validate a local package vs this install via POST /deploy/v1/packages/validate-local (requires scope:deploy and deploy.promote)." + authNote,
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"artifact": map[string]any{"type": "object"},
					"bundleId": map[string]any{"type": "string"},
					"label":    map[string]any{"type": "string"},
				},
			},
		},
		{
			Name:        "org_deploy",
			Description: "Promote a validated bundle vs this install via POST /deploy/v1/promotions (requires scope:deploy and deploy.promote)." + authNote,
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"bundleId": map[string]any{"type": "string"},
					"dryRun":   map[string]any{"type": "boolean"},
				},
				"required": []string{"bundleId"},
			},
		},
		{
			Name:        "pack",
			Description: "Store a customer package artifact as a bundle via POST /deploy/v1/packages/pack (requires scope:deploy and deploy.promote)." + authNote,
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"artifact": map[string]any{"type": "object"},
					"label":    map[string]any{"type": "string"},
				},
				"required": []string{"artifact"},
			},
		},
		{
			Name:        "org_retrieve",
			Description: "Retrieve the install customer snapshot (Deploy export SoR) via GET /deploy/v1/packages/export (requires scope:deploy)." + authNote,
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			Name:        "install_version",
			Description: "Read install version via GET /version (authenticated; not Ops mutate)." + authNote,
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		},
	}
}

// CallTool executes a named tool as the given actor.
func CallTool(ctx context.Context, deps Deps, actor *authz.Actor, name string, args map[string]any) (any, error) {
	if args == nil {
		args = map[string]any{}
	}
	switch name {
	case "describe_global":
		return describeGlobal(ctx, deps, actor)
	case "describe_object":
		return describeObject(ctx, deps, actor, strArg(args, "object"))
	case "query":
		return runQuery(ctx, deps, actor, args)
	case "search":
		return runSearch(ctx, deps, actor, args)
	case "get_record":
		return getRecord(ctx, deps, actor, strArg(args, "object"), strArg(args, "id"))
	case "create_record":
		data, _ := args["data"].(map[string]any)
		return createRecord(ctx, deps, actor, strArg(args, "object"), data)
	case "update_record":
		data, _ := args["data"].(map[string]any)
		return updateRecord(ctx, deps, actor, strArg(args, "object"), strArg(args, "id"), data)
	case "get_object_metadata":
		return getObjectMetadata(ctx, deps, actor, strArg(args, "apiName"))
	case "list_agent_specs":
		return listAgentSpecs(ctx, deps, actor)
	case "create_agent_run":
		return createAgentRun(ctx, deps, actor, args)
	case "get_agent_run":
		return getAgentRun(ctx, deps, actor, strArg(args, "id"))
	case "invoke_action":
		input, _ := args["input"].(map[string]any)
		return invokeAction(ctx, deps, actor, strArg(args, "apiName"), input)
	case "invoke_skill":
		input, _ := args["input"].(map[string]any)
		return invokeSkill(ctx, deps, actor, strArg(args, "apiName"), strArg(args, "playbookApiName"), input)
	case "list_objects_metadata":
		return listObjectsMetadata(ctx, deps, actor)
	case "upsert_object":
		return upsertObject(ctx, deps, actor, args)
	case "upsert_field":
		return upsertField(ctx, deps, actor, args)
	case "org_validate":
		return orgValidate(ctx, deps, actor, args)
	case "org_deploy":
		return orgDeploy(ctx, deps, actor, args)
	case "pack":
		return packArtifact(ctx, deps, actor, args)
	case "org_retrieve":
		return orgRetrieve(ctx, deps, actor)
	case "install_version":
		return installVersion(deps, actor)
	default:
		return nil, fmt.Errorf("%w: tool %s", ErrNotFound, name)
	}
}

func requireScope(actor *authz.Actor, scope authz.Scope) error {
	if actor == nil || !actor.HasScope(scope) {
		return fmt.Errorf("%w: scope %s required", ErrUnauthorized, scope)
	}
	return nil
}

func strArg(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return strings.TrimSpace(v)
}

func describeGlobal(ctx context.Context, deps Deps, actor *authz.Actor) (any, error) {
	if err := requireScope(actor, authz.ScopeClient); err != nil {
		return nil, err
	}
	if deps.Meta == nil {
		return nil, fmt.Errorf("metadata unavailable")
	}
	desc, err := deps.Meta.DescribeGlobal(ctx)
	if err != nil {
		return nil, err
	}
	if actor == nil || actor.IsAdmin || deps.ObjectAz == nil {
		return desc, nil
	}
	out := *desc
	out.SObjects = make([]metadata.GlobalSObjectRef, 0, len(desc.SObjects))
	for _, ref := range desc.SObjects {
		if err := deps.ObjectAz.AssertObjectAccess(ctx, actor, ref.Name, authz.ActionRead); err != nil {
			continue
		}
		out.SObjects = append(out.SObjects, ref)
	}
	return &out, nil
}

func describeObject(ctx context.Context, deps Deps, actor *authz.Actor, object string) (any, error) {
	if err := requireScope(actor, authz.ScopeClient); err != nil {
		return nil, err
	}
	if object == "" {
		return nil, fmt.Errorf("object required")
	}
	if deps.Meta == nil {
		return nil, fmt.Errorf("metadata unavailable")
	}
	desc, err := deps.Meta.Describe(ctx, object)
	if err != nil {
		return nil, err
	}
	if deps.ObjectAz != nil {
		if err := deps.ObjectAz.AssertObjectAccess(ctx, actor, object, authz.ActionRead); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrForbidden, err)
		}
	}
	if deps.FieldAz == nil || actor == nil || actor.IsAdmin {
		return desc, nil
	}
	out := *desc
	fields := make([]metadata.FieldDefinition, 0, len(desc.Fields))
	for _, f := range desc.Fields {
		ok, err := deps.FieldAz.FieldReadable(ctx, actor, object, f.APIName)
		if err != nil {
			return nil, err
		}
		if ok {
			fields = append(fields, f)
		}
	}
	out.Fields = fields
	return &out, nil
}

func runQuery(ctx context.Context, deps Deps, actor *authz.Actor, args map[string]any) (any, error) {
	if err := requireScope(actor, authz.ScopeClient); err != nil {
		return nil, err
	}
	if deps.Data == nil || deps.ObjectAz == nil {
		return nil, fmt.Errorf("data-engine unavailable")
	}
	object, _ := args["object"].(string)
	object = strings.TrimSpace(object)
	if object == "" {
		return nil, fmt.Errorf("object required")
	}
	if err := deps.ObjectAz.AssertObjectAccess(ctx, actor, object, authz.ActionRead); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrForbidden, err)
	}
	viewAll, err := deps.ObjectAz.GetViewAllObjects(ctx, actor)
	if err != nil {
		return nil, err
	}
	vis, err := dataengine.BuildQueryVisibility(ctx, deps.Pool, actor, object, viewAll)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	result, err := deps.Data.Query(ctx, raw, vis)
	if err != nil {
		return nil, err
	}
	// Visibility is enforced in SQL via QueryVisibility; retain FLS strip only.
	records := result.Records
	if deps.FieldAz != nil {
		stripped := make([]dataengine.SObjectRecord, 0, len(records))
		for _, rec := range records {
			outRec, err := deps.FieldAz.StripUnreadableFields(ctx, actor, object, rec)
			if err != nil {
				return nil, err
			}
			stripped = append(stripped, outRec)
		}
		records = stripped
	}
	out := map[string]any{
		"records":   records,
		"totalSize": len(records),
		"done":      result.Done,
		"queryPlan": result.QueryPlan,
	}
	if result.NextCursor != "" {
		out["nextCursor"] = result.NextCursor
	}
	return out, nil
}

func runSearch(ctx context.Context, deps Deps, actor *authz.Actor, args map[string]any) (any, error) {
	if err := requireScope(actor, authz.ScopeClient); err != nil {
		return nil, err
	}
	if deps.Data == nil || deps.ObjectAz == nil || deps.Meta == nil {
		return nil, fmt.Errorf("data-engine unavailable")
	}
	q, _ := args["q"].(string)
	req := dataengine.SearchRequest{Query: q}
	if lim, ok := intArg(args["limit"]); ok {
		req.Limit = lim
	}
	if raw, ok := args["objects"]; ok {
		req.Objects = stringSliceArg(raw)
	}
	named := len(req.Objects) > 0
	scopes, err := resolveMCPSearchScopes(ctx, deps, actor, req.Objects)
	if err != nil {
		return nil, err
	}
	if named && len(scopes) == 0 {
		return nil, &dataengine.ValidationError{Message: "no searchable objects in request"}
	}
	result, err := deps.Data.Search(ctx, req, scopes)
	if err != nil {
		return nil, err
	}
	hits := result.Hits
	if deps.FieldAz != nil {
		stripped := make([]dataengine.SearchHit, 0, len(hits))
		flsCtx := authz.ContextWithFLSCache(ctx)
		for _, hit := range hits {
			out, err := stripMCPSearchHitFLS(flsCtx, deps, actor, hit)
			if err != nil {
				return nil, err
			}
			stripped = append(stripped, out)
		}
		hits = stripped
	}
	return map[string]any{"query": result.Query, "hits": hits}, nil
}

func resolveMCPSearchScopes(ctx context.Context, deps Deps, actor *authz.Actor, requested []string) ([]dataengine.SearchScope, error) {
	named := len(requested) > 0
	var names []string
	if named {
		for _, n := range requested {
			n = strings.TrimSpace(n)
			if n != "" {
				names = append(names, n)
			}
		}
		if len(names) == 0 {
			return nil, &dataengine.ValidationError{Message: "objects must not be empty"}
		}
	} else {
		listed, err := deps.Meta.ListObjects(ctx)
		if err != nil {
			return nil, err
		}
		for _, o := range listed {
			names = append(names, o.APIName)
		}
	}
	viewAll, err := deps.ObjectAz.GetViewAllObjects(ctx, actor)
	if err != nil {
		return nil, err
	}
	var scopes []dataengine.SearchScope
	for _, name := range names {
		obj, err := deps.Meta.GetObject(ctx, name)
		if err != nil {
			if named && errors.Is(err, metadata.ErrNotFound) {
				return nil, &dataengine.ValidationError{Message: "unknown object: " + name}
			}
			if named {
				return nil, err
			}
			continue
		}
		if db.IsKernelStorage(obj.StorageMode) {
			if named {
				return nil, &dataengine.ValidationError{Message: name + " is not a flexible object"}
			}
			continue
		}
		fields, err := deps.Meta.GetFields(ctx, name)
		if err != nil {
			return nil, err
		}
		if !dataengine.HasSearchableField(fields) {
			if named {
				return nil, &dataengine.ValidationError{Message: "object has no searchable fields: " + name}
			}
			continue
		}
		if err := deps.ObjectAz.AssertObjectAccess(ctx, actor, name, authz.ActionRead); err != nil {
			if named {
				return nil, &dataengine.ValidationError{Message: "cannot read object: " + name}
			}
			continue
		}
		vis, err := dataengine.BuildQueryVisibility(ctx, deps.Pool, actor, name, viewAll)
		if err != nil {
			return nil, err
		}
		scopes = append(scopes, dataengine.SearchScope{
			ObjectAPIName: name,
			StorageMode:   obj.StorageMode,
			Visibility:    vis,
		})
	}
	return scopes, nil
}

func stripMCPSearchHitFLS(ctx context.Context, deps Deps, actor *authz.Actor, hit dataengine.SearchHit) (dataengine.SearchHit, error) {
	titleReadable := false
	for _, field := range []string{"Name", "Subject", "Title", "FirstName", "LastName"} {
		ok, err := deps.FieldAz.FieldReadable(ctx, actor, hit.Object, field)
		if err != nil {
			return hit, err
		}
		if ok {
			titleReadable = true
			break
		}
	}
	if !titleReadable {
		hit.Title = ""
	}
	subReadable := false
	for _, field := range []string{"Email", "Phone", "MobilePhone", "AccountNumber", "SerialNumber", "ProductCode"} {
		ok, err := deps.FieldAz.FieldReadable(ctx, actor, hit.Object, field)
		if err != nil {
			return hit, err
		}
		if ok {
			subReadable = true
			break
		}
	}
	if !subReadable {
		hit.Subtitle = ""
	}
	return hit, nil
}

func intArg(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}

func stringSliceArg(v any) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			s, _ := item.(string)
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func canViewRecordMCP(ctx context.Context, deps Deps, actor *authz.Actor, recordID, ownerID, createdByID, objectAPIName string, viewAll map[string]struct{}) (bool, error) {
	if deps.RecordAccess != nil {
		return deps.RecordAccess.CanViewRecordFull(ctx, actor, recordID, ownerID, createdByID, objectAPIName, viewAll, true)
	}
	return authz.CanViewRecord(actor, ownerID, createdByID, objectAPIName, viewAll), nil
}

func canModifyRecordMCP(ctx context.Context, deps Deps, actor *authz.Actor, recordID, ownerID, createdByID, objectAPIName string, modifyAll map[string]struct{}) (bool, error) {
	if deps.RecordAccess != nil {
		return deps.RecordAccess.CanModifyRecordFull(ctx, actor, recordID, ownerID, createdByID, objectAPIName, modifyAll, true)
	}
	return authz.CanModifyRecord(actor, ownerID, createdByID, objectAPIName, modifyAll), nil
}

func getRecord(ctx context.Context, deps Deps, actor *authz.Actor, object, id string) (any, error) {
	if err := requireScope(actor, authz.ScopeClient); err != nil {
		return nil, err
	}
	if object == "" || id == "" {
		return nil, fmt.Errorf("object and id required")
	}
	if deps.Data == nil || deps.ObjectAz == nil {
		return nil, fmt.Errorf("data-engine unavailable")
	}
	if err := deps.ObjectAz.AssertObjectAccess(ctx, actor, object, authz.ActionRead); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrForbidden, err)
	}
	rec, err := deps.Data.Get(ctx, object, id)
	if err != nil {
		return nil, err
	}
	viewAll, err := deps.ObjectAz.GetViewAllObjects(ctx, actor)
	if err != nil {
		return nil, err
	}
	ownerID, _ := rec["OwnerId"].(string)
	createdByID, _ := rec["CreatedById"].(string)
	recID, _ := rec["Id"].(string)
	ok, err := canViewRecordMCP(ctx, deps, actor, recID, ownerID, createdByID, object, viewAll)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("%w: not allowed", ErrForbidden)
	}
	if deps.FieldAz != nil {
		rec, err = deps.FieldAz.StripUnreadableFields(ctx, actor, object, rec)
		if err != nil {
			return nil, err
		}
	}
	return rec, nil
}

func createRecord(ctx context.Context, deps Deps, actor *authz.Actor, object string, data map[string]any) (any, error) {
	if err := requireScope(actor, authz.ScopeClient); err != nil {
		return nil, err
	}
	if object == "" {
		return nil, fmt.Errorf("object required")
	}
	if data == nil {
		data = map[string]any{}
	}
	if deps.Data == nil || deps.ObjectAz == nil {
		return nil, fmt.Errorf("data-engine unavailable")
	}
	if err := deps.ObjectAz.AssertObjectAccess(ctx, actor, object, authz.ActionCreate); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrForbidden, err)
	}
	if deps.FieldAz != nil {
		if err := deps.FieldAz.AssertEditableFields(ctx, actor, object, data); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrForbidden, err)
		}
	}
	rec, err := deps.Data.Create(ctx, object, data, actor)
	if err != nil {
		return nil, err
	}
	if deps.FieldAz != nil {
		rec, err = deps.FieldAz.StripUnreadableFields(ctx, actor, object, rec)
		if err != nil {
			return nil, err
		}
	}
	return rec, nil
}

func updateRecord(ctx context.Context, deps Deps, actor *authz.Actor, object, id string, data map[string]any) (any, error) {
	if err := requireScope(actor, authz.ScopeClient); err != nil {
		return nil, err
	}
	if object == "" || id == "" {
		return nil, fmt.Errorf("object and id required")
	}
	if data == nil {
		data = map[string]any{}
	}
	if deps.Data == nil || deps.ObjectAz == nil {
		return nil, fmt.Errorf("data-engine unavailable")
	}
	if err := deps.ObjectAz.AssertObjectAccess(ctx, actor, object, authz.ActionUpdate); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrForbidden, err)
	}
	existing, err := deps.Data.Get(ctx, object, id)
	if err != nil {
		return nil, err
	}
	modifyAll, err := deps.ObjectAz.GetModifyAllObjects(ctx, actor)
	if err != nil {
		return nil, err
	}
	ownerID, _ := existing["OwnerId"].(string)
	createdByID, _ := existing["CreatedById"].(string)
	ok, err := canModifyRecordMCP(ctx, deps, actor, id, ownerID, createdByID, object, modifyAll)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("%w: not allowed", ErrForbidden)
	}
	if deps.FieldAz != nil {
		if err := deps.FieldAz.AssertEditableFields(ctx, actor, object, data); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrForbidden, err)
		}
	}
	rec, err := deps.Data.Update(ctx, object, id, data, actor)
	if err != nil {
		return nil, err
	}
	if deps.FieldAz != nil {
		rec, err = deps.FieldAz.StripUnreadableFields(ctx, actor, object, rec)
		if err != nil {
			return nil, err
		}
	}
	return rec, nil
}

func getObjectMetadata(ctx context.Context, deps Deps, actor *authz.Actor, apiName string) (any, error) {
	if err := requireScope(actor, authz.ScopeMetadata); err != nil {
		return nil, err
	}
	if apiName == "" {
		return nil, fmt.Errorf("apiName required")
	}
	if deps.Meta == nil {
		return nil, fmt.Errorf("metadata unavailable")
	}
	return deps.Meta.Describe(ctx, apiName)
}

func listAgentSpecs(ctx context.Context, deps Deps, actor *authz.Actor) (any, error) {
	if err := requireScope(actor, authz.ScopeMetadata); err != nil {
		return nil, err
	}
	if deps.Pool == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	rows, err := deps.Pool.Query(ctx, `
SELECT api_name, label, goal_template, COALESCE(instructions, ''), ownership, package_name, active
FROM agent_playbooks ORDER BY api_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []map[string]any{}
	for rows.Next() {
		var apiName, label, goal, instructions, ownership, pkg string
		var active bool
		if err := rows.Scan(&apiName, &label, &goal, &instructions, &ownership, &pkg, &active); err != nil {
			return nil, err
		}
		list = append(list, map[string]any{
			"apiName": apiName, "label": label, "goalTemplate": goal,
			"instructions": instructions, "ownership": ownership, "packageName": pkg, "active": active,
		})
	}
	return map[string]any{"playbooks": list}, nil
}

func createAgentRun(ctx context.Context, deps Deps, actor *authz.Actor, args map[string]any) (any, error) {
	if err := requireScope(actor, authz.ScopeClient); err != nil {
		return nil, err
	}
	if deps.Pool == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	playbook, _ := args["playbookApiName"].(string)
	goal, _ := args["goal"].(string)
	input, _ := args["input"].(map[string]any)
	if input == nil {
		input = map[string]any{}
	}
	dryRun, _ := args["dryRun"].(bool)
	approved, _ := args["approved"].(bool)

	var playbookGoal *string
	var requireApproval bool
	var instructions string
	var allowedTools, objectScopes []byte
	if playbook != "" {
		var g string
		err := deps.Pool.QueryRow(ctx, `
SELECT goal_template, require_approval, COALESCE(instructions, ''), allowed_tools, object_scopes
FROM agent_playbooks WHERE api_name=$1 AND active=true`, playbook).
			Scan(&g, &requireApproval, &instructions, &allowedTools, &objectScopes)
		if err == nil {
			playbookGoal = &g
			if instructions != "" {
				if _, ok := input["instructions"]; !ok {
					input["instructions"] = instructions
				}
			}
		}
	}
	if goal == "" && playbookGoal != nil {
		goal = *playbookGoal
	}
	if goal == "" {
		return nil, fmt.Errorf("goal is required")
	}
	status := "queued"
	if requireApproval && !approved {
		status = "awaiting_approval"
	}
	inputJSON, _ := json.Marshal(input)
	var actorID *string
	if actor != nil && actor.ID != "" {
		actorID = &actor.ID
	}
	var runID string
	err := deps.Pool.QueryRow(ctx, `
INSERT INTO agent_runs (playbook_api_name, status, goal, input, actor_id, dry_run)
VALUES ($1,$2,$3,$4::jsonb,$5::uuid,$6)
RETURNING id::text`, nullableStr(playbook), status, goal, string(inputJSON), actorID, dryRun).Scan(&runID)
	if err != nil {
		return nil, err
	}
	if status == "queued" {
		tools := []string{"sobjects.read", "query"}
		if len(allowedTools) > 0 {
			_ = json.Unmarshal(allowedTools, &tools)
		}
		scopes := []string{}
		if len(objectScopes) > 0 {
			_ = json.Unmarshal(objectScopes, &scopes)
		}
		jobPayload, _ := json.Marshal(map[string]any{
			"runId": runID, "goal": goal, "dryRun": dryRun,
			"playbookApiName": nullableStr(playbook), "allowedTools": tools,
			"objectScopes": scopes, "input": input,
		})
		_, _ = deps.Pool.Exec(ctx, `INSERT INTO jobs (job_type, payload) VALUES ('agent.run', $1::jsonb)`, string(jobPayload))
	}
	return map[string]any{
		"id": runID, "playbookApiName": nullableStr(playbook), "status": status,
		"goal": goal, "input": input, "dryRun": dryRun,
	}, nil
}

func getAgentRun(ctx context.Context, deps Deps, actor *authz.Actor, id string) (any, error) {
	if err := requireScope(actor, authz.ScopeClient); err != nil {
		return nil, err
	}
	if id == "" {
		return nil, fmt.Errorf("id required")
	}
	if deps.Pool == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	var playbook, goal *string
	var status string
	var input, output []byte
	var dryRun bool
	err := deps.Pool.QueryRow(ctx, `
SELECT playbook_api_name, status, goal, input, output, dry_run
FROM agent_runs WHERE id=$1::uuid`, id).Scan(&playbook, &status, &goal, &input, &output, &dryRun)
	if err != nil {
		return nil, err
	}
	m := map[string]any{"id": id, "status": status, "dryRun": dryRun}
	if playbook != nil {
		m["playbookApiName"] = *playbook
	}
	if goal != nil {
		m["goal"] = *goal
	}
	var in, out any
	_ = json.Unmarshal(input, &in)
	_ = json.Unmarshal(output, &out)
	m["input"] = in
	m["output"] = out
	return m, nil
}

func nullableStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
