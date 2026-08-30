package authz

import (
	"context"
	"fmt"
	"sync"
)

// FieldPermission is one field_permissions row.
type FieldPermission struct {
	PermissionSetID string
	ObjectAPIName   string
	FieldAPIName    string
	CanRead         bool
	CanEdit         bool
}

// FieldPermissionStore loads field_permissions for permission set ids.
type FieldPermissionStore interface {
	ListByPermissionSets(ctx context.Context, permissionSetIDs []string) ([]FieldPermission, error)
}

// FieldAuthz evaluates field-level security (deny-by-default; OR-union across permission sets).
type FieldAuthz struct {
	Store FieldPermissionStore
}

// systemFieldsAlwaysReadable are never stripped by FLS.
var systemFieldsAlwaysReadable = map[string]struct{}{
	"Id": {}, "OwnerId": {}, "CreatedAt": {}, "UpdatedAt": {},
	"CreatedById": {}, "LastModifiedById": {},
}

// IsSystemField reports whether FLS never strips or blocks the field.
func IsSystemField(apiName string) bool {
	_, ok := systemFieldsAlwaysReadable[apiName]
	return ok
}

type fieldFLS struct {
	canRead bool
	canEdit bool
}

type flsCacheCtxKey struct{}

type flsRequestCache struct {
	mu    sync.Mutex
	byKey map[string]map[string]fieldFLS
}

// ContextWithFLSCache returns a child context that memoizes effectiveFLS per actor+object
// for the lifetime of the request (avoids N× ListByPermissionSets on query pages).
func ContextWithFLSCache(ctx context.Context) context.Context {
	return context.WithValue(ctx, flsCacheCtxKey{}, &flsRequestCache{byKey: map[string]map[string]fieldFLS{}})
}

// effectiveFLS builds the OR-union map of field grants for the actor on one object.
// Fields absent from the map are denied (deny-by-default).
// When ctx was wrapped with ContextWithFLSCache, results are memoized per object for the request.
func (a *FieldAuthz) effectiveFLS(ctx context.Context, actor *Actor, objectAPIName string) (map[string]fieldFLS, error) {
	out := map[string]fieldFLS{}
	if actor == nil || actor.IsAdmin || a == nil || a.Store == nil || len(actor.PermissionSetIDs) == 0 {
		return out, nil
	}
	cacheKey := actor.ID + "\x00" + objectAPIName
	if cache, ok := ctx.Value(flsCacheCtxKey{}).(*flsRequestCache); ok {
		cache.mu.Lock()
		if m, hit := cache.byKey[cacheKey]; hit {
			cache.mu.Unlock()
			return m, nil
		}
		cache.mu.Unlock()
	}
	perms, err := a.Store.ListByPermissionSets(ctx, actor.PermissionSetIDs)
	if err != nil {
		return nil, err
	}
	for _, p := range perms {
		if p.ObjectAPIName != objectAPIName {
			continue
		}
		cur := out[p.FieldAPIName]
		cur.canRead = cur.canRead || p.CanRead
		cur.canEdit = cur.canEdit || p.CanEdit
		out[p.FieldAPIName] = cur
	}
	if cache, ok := ctx.Value(flsCacheCtxKey{}).(*flsRequestCache); ok {
		cache.mu.Lock()
		cache.byKey[cacheKey] = out
		cache.mu.Unlock()
	}
	return out, nil
}

// StripUnreadableFields removes fields the actor cannot read. Mutates and returns data.
// Deny-by-default: fields with no grant across assigned permission sets are stripped.
func (a *FieldAuthz) StripUnreadableFields(ctx context.Context, actor *Actor, objectAPIName string, data map[string]any) (map[string]any, error) {
	if data == nil || actor == nil || actor.IsAdmin {
		return data, nil
	}
	fls, err := a.effectiveFLS(ctx, actor, objectAPIName)
	if err != nil {
		return nil, err
	}
	for key := range data {
		if _, sys := systemFieldsAlwaysReadable[key]; sys {
			continue
		}
		rule, ok := fls[key]
		if !ok || !rule.canRead {
			delete(data, key)
		}
	}
	return data, nil
}

// AssertEditableFields returns ErrForbidden if any key in input is not editable.
// Deny-by-default: fields with no grant across assigned permission sets are forbidden.
func (a *FieldAuthz) AssertEditableFields(ctx context.Context, actor *Actor, objectAPIName string, input map[string]any) error {
	if input == nil || actor == nil || actor.IsAdmin {
		return nil
	}
	fls, err := a.effectiveFLS(ctx, actor, objectAPIName)
	if err != nil {
		return err
	}
	for key := range input {
		if len(key) > 0 && key[0] == '_' {
			continue
		}
		if _, sys := systemFieldsAlwaysReadable[key]; sys {
			continue
		}
		rule, ok := fls[key]
		if !ok || !rule.canEdit {
			return fmt.Errorf("%w: not allowed to edit field %s.%s", ErrForbidden, objectAPIName, key)
		}
	}
	return nil
}

// FieldReadable reports whether the actor may read the field (deny-by-default).
func (a *FieldAuthz) FieldReadable(ctx context.Context, actor *Actor, objectAPIName, fieldAPIName string) (bool, error) {
	if actor == nil {
		return false, nil
	}
	if actor.IsAdmin || IsSystemField(fieldAPIName) {
		return true, nil
	}
	fls, err := a.effectiveFLS(ctx, actor, objectAPIName)
	if err != nil {
		return false, err
	}
	rule, ok := fls[fieldAPIName]
	return ok && rule.canRead, nil
}
