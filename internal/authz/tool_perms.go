package authz

import (
	"context"
	"fmt"
)

// ToolPermission is one tool_permissions row.
type ToolPermission struct {
	PermissionSetID string
	ToolAPIName     string
	CanOpen         bool
	CanInteract     bool
	CanModify       bool
	CanPublish      bool
}

// ToolAccess is the OR-union of one actor's grants for a ToolSpec.
type ToolAccess struct {
	CanOpen     bool `json:"canOpen"`
	CanInteract bool `json:"canInteract"`
	CanModify   bool `json:"canModify"`
	CanPublish  bool `json:"canPublish"`
}

// ToolPermissionStore loads ToolSpec grants for permission set ids.
type ToolPermissionStore interface {
	// ListByPermissionSets returns per-ToolSpec rows for the given set ids.
	ListByPermissionSets(ctx context.Context, permissionSetIDs []string) ([]ToolPermission, error)
	// AnyAllTools reports whether any of the permission sets has all_tools=true.
	AnyAllTools(ctx context.Context, permissionSetIDs []string) (bool, error)
}

// ToolAuthz evaluates whether an actor may open a ToolSpec.
type ToolAuthz struct {
	Store ToolPermissionStore
}

// ActorToolAccess returns the OR-union of explicit ToolSpec grants. all_tools
// preserves open + interact behavior; modify and publish remain explicit.
func (a *ToolAuthz) ActorToolAccess(ctx context.Context, actor *Actor, toolAPIName string) (ToolAccess, error) {
	if actor == nil {
		return ToolAccess{}, fmt.Errorf("%w: no actor", ErrForbidden)
	}
	if toolAPIName == "" {
		return ToolAccess{}, nil
	}
	if actor.IsAdmin {
		return ToolAccess{CanOpen: true, CanInteract: true, CanModify: true, CanPublish: true}, nil
	}
	if a == nil || a.Store == nil || len(actor.PermissionSetIDs) == 0 {
		return ToolAccess{}, nil
	}
	all, err := a.Store.AnyAllTools(ctx, actor.PermissionSetIDs)
	if err != nil {
		return ToolAccess{}, err
	}
	access := ToolAccess{CanOpen: all, CanInteract: all}
	perms, err := a.Store.ListByPermissionSets(ctx, actor.PermissionSetIDs)
	if err != nil {
		return ToolAccess{}, err
	}
	for _, p := range perms {
		if p.ToolAPIName != toolAPIName {
			continue
		}
		access.CanOpen = access.CanOpen || p.CanOpen
		access.CanInteract = access.CanInteract || p.CanInteract
		access.CanModify = access.CanModify || p.CanModify
		access.CanPublish = access.CanPublish || p.CanPublish
	}
	return access, nil
}

// ActorCanOpenTool preserves the original boolean authorization surface.
func (a *ToolAuthz) ActorCanOpenTool(ctx context.Context, actor *Actor, toolAPIName string) (bool, error) {
	access, err := a.ActorToolAccess(ctx, actor, toolAPIName)
	return access.CanOpen, err
}

// AssertCanOpenTool returns ErrForbidden when ActorCanOpenTool is false.
func (a *ToolAuthz) AssertCanOpenTool(ctx context.Context, actor *Actor, toolAPIName string) error {
	ok, err := a.ActorCanOpenTool(ctx, actor, toolAPIName)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: cannot open tool %s", ErrForbidden, toolAPIName)
	}
	return nil
}
