package authz

import (
	"context"
	"errors"
	"fmt"
)

// CrudAction is an object-level permission action.
type CrudAction string

const (
	ActionCreate CrudAction = "create"
	ActionRead   CrudAction = "read"
	ActionUpdate CrudAction = "update"
	ActionDelete CrudAction = "delete"
)

// ErrForbidden is returned when AuthZ denies an action.
var ErrForbidden = errors.New("forbidden")

// ObjectPermission is one object_permissions row.
type ObjectPermission struct {
	PermissionSetID string
	ObjectAPIName   string
	CanCreate       bool
	CanRead         bool
	CanUpdate       bool
	CanDelete       bool
	ViewAll         bool
	ModifyAll       bool
}

// ObjectPermissionStore loads object_permissions for permission set ids.
type ObjectPermissionStore interface {
	ListByPermissionSets(ctx context.Context, permissionSetIDs []string) ([]ObjectPermission, error)
}

// ObjectAuthz evaluates object CRUD and record visibility.
type ObjectAuthz struct {
	Store ObjectPermissionStore
}

// AssertObjectAccess allows the action or returns ErrForbidden.
func (a *ObjectAuthz) AssertObjectAccess(ctx context.Context, actor *Actor, objectAPIName string, action CrudAction) error {
	if actor == nil {
		return fmt.Errorf("%w: no actor", ErrForbidden)
	}
	if actor.IsAdmin {
		return nil
	}
	if len(actor.PermissionSetIDs) == 0 {
		return fmt.Errorf("%w: no permission sets for %s on %s", ErrForbidden, action, objectAPIName)
	}
	if a.Store == nil {
		return fmt.Errorf("%w: object permission store not configured", ErrForbidden)
	}
	perms, err := a.Store.ListByPermissionSets(ctx, actor.PermissionSetIDs)
	if err != nil {
		return err
	}
	for _, p := range perms {
		if p.ObjectAPIName != objectAPIName {
			continue
		}
		if allows(p, action) {
			return nil
		}
	}
	return fmt.Errorf("%w: not allowed to %s %s", ErrForbidden, action, objectAPIName)
}

func allows(p ObjectPermission, action CrudAction) bool {
	switch action {
	case ActionCreate:
		return p.CanCreate || p.ModifyAll
	case ActionRead:
		return p.CanRead || p.ViewAll || p.ModifyAll
	case ActionUpdate:
		return p.CanUpdate || p.ModifyAll
	case ActionDelete:
		return p.CanDelete || p.ModifyAll
	default:
		return false
	}
}

// CanViewRecord reports CreatedBy/Owner/view-all visibility (admin always true).
// OwnerId match applies only when ownerID is non-empty; CreatedById always participates.
func CanViewRecord(actor *Actor, ownerID, createdByID, objectAPIName string, viewAllObjects map[string]struct{}) bool {
	if actor == nil {
		return false
	}
	if actor.IsAdmin {
		return true
	}
	if ownerID != "" && ownerID == actor.ID {
		return true
	}
	if createdByID != "" && createdByID == actor.ID {
		return true
	}
	if _, ok := viewAllObjects["*"]; ok {
		return true
	}
	_, ok := viewAllObjects[objectAPIName]
	return ok
}

// CanModifyRecord reports owner-or-modifyAll update/delete rights (admin always true).
// When OwnerId is set, only the owner (or modifyAll/admin) may modify — CreatedBy alone is not enough.
// When OwnerId is empty, CreatedById may modify.
func CanModifyRecord(actor *Actor, ownerID, createdByID, objectAPIName string, modifyAllObjects map[string]struct{}) bool {
	if actor == nil {
		return false
	}
	if actor.IsAdmin {
		return true
	}
	if _, ok := modifyAllObjects["*"]; ok {
		return true
	}
	if _, ok := modifyAllObjects[objectAPIName]; ok {
		return true
	}
	if ownerID != "" {
		return ownerID == actor.ID
	}
	return createdByID != "" && createdByID == actor.ID
}

// HasModifyAll reports whether the actor has modifyAll on the object (or is admin).
func HasModifyAll(actor *Actor, objectAPIName string, modifyAllObjects map[string]struct{}) bool {
	if actor == nil {
		return false
	}
	if actor.IsAdmin {
		return true
	}
	if _, ok := modifyAllObjects["*"]; ok {
		return true
	}
	_, ok := modifyAllObjects[objectAPIName]
	return ok
}

// AssertOwnerIDWritable allows OwnerId assignment only for admin/modifyAll, or self-assign.
func AssertOwnerIDWritable(actor *Actor, objectAPIName string, newOwnerID *string, modifyAllObjects map[string]struct{}) error {
	if actor == nil {
		return fmt.Errorf("%w: no actor", ErrForbidden)
	}
	if HasModifyAll(actor, objectAPIName, modifyAllObjects) {
		return nil
	}
	// Non-privileged callers may only self-assign; clearing OwnerId requires modifyAll.
	if newOwnerID != nil && *newOwnerID == actor.ID {
		return nil
	}
	return fmt.Errorf("%w: OwnerId requires modifyAll or admin", ErrForbidden)
}

// GetViewAllObjects returns object api names where viewAll or modifyAll is granted.
// Admins get a set containing "*".
func (a *ObjectAuthz) GetViewAllObjects(ctx context.Context, actor *Actor) (map[string]struct{}, error) {
	out := map[string]struct{}{}
	if actor == nil {
		return out, nil
	}
	if actor.IsAdmin {
		out["*"] = struct{}{}
		return out, nil
	}
	if a.Store == nil || len(actor.PermissionSetIDs) == 0 {
		return out, nil
	}
	perms, err := a.Store.ListByPermissionSets(ctx, actor.PermissionSetIDs)
	if err != nil {
		return nil, err
	}
	for _, p := range perms {
		if p.ViewAll || p.ModifyAll {
			out[p.ObjectAPIName] = struct{}{}
		}
	}
	return out, nil
}

// GetModifyAllObjects returns object api names where modifyAll is granted.
// Admins get a set containing "*".
func (a *ObjectAuthz) GetModifyAllObjects(ctx context.Context, actor *Actor) (map[string]struct{}, error) {
	out := map[string]struct{}{}
	if actor == nil {
		return out, nil
	}
	if actor.IsAdmin {
		out["*"] = struct{}{}
		return out, nil
	}
	if a.Store == nil || len(actor.PermissionSetIDs) == 0 {
		return out, nil
	}
	perms, err := a.Store.ListByPermissionSets(ctx, actor.PermissionSetIDs)
	if err != nil {
		return nil, err
	}
	for _, p := range perms {
		if p.ModifyAll {
			out[p.ObjectAPIName] = struct{}{}
		}
	}
	return out, nil
}
