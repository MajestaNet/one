package authz

import (
	"context"
	"fmt"
)

// AutomationPermission is one automation_permissions row (plus optional allAutomations from the PS).
type AutomationPermission struct {
	PermissionSetID   string
	AutomationAPIName string
	CanRun            bool
}

// AutomationPermissionStore loads automation grants for permission set ids.
type AutomationPermissionStore interface {
	// ListByPermissionSets returns per-automation rows for the given set ids.
	ListByPermissionSets(ctx context.Context, permissionSetIDs []string) ([]AutomationPermission, error)
	// AnyAllAutomations reports whether any of the permission sets has all_automations=true.
	AnyAllAutomations(ctx context.Context, permissionSetIDs []string) (bool, error)
}

// AutomationAuthz evaluates whether an actor may start/run an automation.
type AutomationAuthz struct {
	Store AutomationPermissionStore
}

// ActorCanRunAutomation returns true when the actor is admin, has allAutomations on any
// assigned permission set, or has can_run=true for apiName on any assigned set (OR-union).
func (a *AutomationAuthz) ActorCanRunAutomation(ctx context.Context, actor *Actor, automationAPIName string) (bool, error) {
	if actor == nil {
		return false, fmt.Errorf("%w: no actor", ErrForbidden)
	}
	if automationAPIName == "" {
		return false, nil
	}
	if actor.IsAdmin {
		return true, nil
	}
	if a == nil || a.Store == nil || len(actor.PermissionSetIDs) == 0 {
		return false, nil
	}
	all, err := a.Store.AnyAllAutomations(ctx, actor.PermissionSetIDs)
	if err != nil {
		return false, err
	}
	if all {
		return true, nil
	}
	perms, err := a.Store.ListByPermissionSets(ctx, actor.PermissionSetIDs)
	if err != nil {
		return false, err
	}
	for _, p := range perms {
		if p.AutomationAPIName == automationAPIName && p.CanRun {
			return true, nil
		}
	}
	return false, nil
}

// AssertCanRunAutomation returns ErrForbidden when ActorCanRunAutomation is false.
func (a *AutomationAuthz) AssertCanRunAutomation(ctx context.Context, actor *Actor, automationAPIName string) error {
	ok, err := a.ActorCanRunAutomation(ctx, actor, automationAPIName)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: cannot run automation %s", ErrForbidden, automationAPIName)
	}
	return nil
}
