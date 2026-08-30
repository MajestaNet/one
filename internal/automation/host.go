package automation

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/MajestaNet/ide/internal/authz"
)

// SyncMutatorBridge adapts SyncMutator to HostBridge (Phase 4 sync Deno path).
type SyncMutatorBridge struct {
	Inner SyncMutator
}

func (b SyncMutatorBridge) CreateRecord(ctx context.Context, objectAPIName string, data map[string]any) (string, error) {
	return b.Inner.CreateRecord(ctx, objectAPIName, data)
}

func (b SyncMutatorBridge) UpdateRecord(ctx context.Context, objectAPIName, recordID string, data map[string]any) error {
	return b.Inner.UpdateRecord(ctx, objectAPIName, recordID, data)
}

func (b SyncMutatorBridge) GetRecord(ctx context.Context, objectAPIName, recordID string) (map[string]any, error) {
	return b.Inner.GetRecord(ctx, objectAPIName, recordID)
}

func (b SyncMutatorBridge) DeleteRecord(context.Context, string, string) error {
	return fmt.Errorf("deleteRecord is not available in sync automations yet")
}

func (b SyncMutatorBridge) Query(context.Context, map[string]any) (map[string]any, error) {
	return nil, fmt.Errorf("query is not available in sync automations yet")
}

func (b SyncMutatorBridge) InvokeAction(context.Context, string, map[string]any) (map[string]any, error) {
	return nil, fmt.Errorf("invokeAction is not available on this host")
}

// AuthzHost wraps a HostBridge with object CRUD AuthZ as the run-as actor.
type AuthzHost struct {
	Inner  HostBridge
	Object *authz.ObjectAuthz
	Actor  *authz.Actor
}

func (h AuthzHost) CreateRecord(ctx context.Context, objectAPIName string, data map[string]any) (string, error) {
	if h.Object != nil {
		if err := h.Object.AssertObjectAccess(ctx, h.Actor, objectAPIName, authz.ActionCreate); err != nil {
			return "", err
		}
	}
	return h.Inner.CreateRecord(ctx, objectAPIName, data)
}

func (h AuthzHost) UpdateRecord(ctx context.Context, objectAPIName, recordID string, data map[string]any) error {
	if h.Object != nil {
		if err := h.Object.AssertObjectAccess(ctx, h.Actor, objectAPIName, authz.ActionUpdate); err != nil {
			return err
		}
	}
	return h.Inner.UpdateRecord(ctx, objectAPIName, recordID, data)
}

func (h AuthzHost) GetRecord(ctx context.Context, objectAPIName, recordID string) (map[string]any, error) {
	if h.Object != nil {
		if err := h.Object.AssertObjectAccess(ctx, h.Actor, objectAPIName, authz.ActionRead); err != nil {
			return nil, err
		}
	}
	return h.Inner.GetRecord(ctx, objectAPIName, recordID)
}

func (h AuthzHost) DeleteRecord(ctx context.Context, objectAPIName, recordID string) error {
	if h.Object != nil {
		if err := h.Object.AssertObjectAccess(ctx, h.Actor, objectAPIName, authz.ActionDelete); err != nil {
			return err
		}
	}
	return h.Inner.DeleteRecord(ctx, objectAPIName, recordID)
}

func (h AuthzHost) Query(ctx context.Context, req map[string]any) (map[string]any, error) {
	obj, _ := req["objectApiName"].(string)
	if obj == "" {
		obj, _ = req["object"].(string)
	}
	if obj != "" && h.Object != nil {
		if err := h.Object.AssertObjectAccess(ctx, h.Actor, obj, authz.ActionRead); err != nil {
			return nil, err
		}
	}
	return h.Inner.Query(ctx, req)
}

func (h AuthzHost) InvokeAction(ctx context.Context, apiName string, input map[string]any) (map[string]any, error) {
	return h.Inner.InvokeAction(ctx, apiName, input)
}

func (h AuthzHost) HandleUnitRPC(ctx context.Context, method string, argsJSON json.RawMessage) (any, bool, error) {
	return forwardUnitRPC(h.Inner, ctx, method, argsJSON)
}

type syncGuestCtxKey struct{}

// WithSyncGuest marks ctx as a sync guest automation (only syncSafe actions).
func WithSyncGuest(ctx context.Context) context.Context {
	return context.WithValue(ctx, syncGuestCtxKey{}, true)
}

// IsSyncGuest reports whether ctx is a sync guest invoke.
func IsSyncGuest(ctx context.Context) bool {
	v, _ := ctx.Value(syncGuestCtxKey{}).(bool)
	return v
}

// ActionInvokeFunc is the host implementation of ctx.invokeAction.
type ActionInvokeFunc func(ctx context.Context, apiName string, input map[string]any) (map[string]any, error)

// BindActions wraps inner so InvokeAction dispatches to fn.
func BindActions(inner HostBridge, fn ActionInvokeFunc) HostBridge {
	if fn == nil {
		return inner
	}
	return actionBoundHost{inner: inner, fn: fn}
}

type actionBoundHost struct {
	inner HostBridge
	fn    ActionInvokeFunc
}

func (h actionBoundHost) CreateRecord(ctx context.Context, objectAPIName string, data map[string]any) (string, error) {
	return h.inner.CreateRecord(ctx, objectAPIName, data)
}
func (h actionBoundHost) UpdateRecord(ctx context.Context, objectAPIName, recordID string, data map[string]any) error {
	return h.inner.UpdateRecord(ctx, objectAPIName, recordID, data)
}
func (h actionBoundHost) GetRecord(ctx context.Context, objectAPIName, recordID string) (map[string]any, error) {
	return h.inner.GetRecord(ctx, objectAPIName, recordID)
}
func (h actionBoundHost) DeleteRecord(ctx context.Context, objectAPIName, recordID string) error {
	return h.inner.DeleteRecord(ctx, objectAPIName, recordID)
}
func (h actionBoundHost) Query(ctx context.Context, req map[string]any) (map[string]any, error) {
	return h.inner.Query(ctx, req)
}
func (h actionBoundHost) InvokeAction(ctx context.Context, apiName string, input map[string]any) (map[string]any, error) {
	return h.fn(ctx, apiName, input)
}

func (h actionBoundHost) HandleUnitRPC(ctx context.Context, method string, argsJSON json.RawMessage) (any, bool, error) {
	return forwardUnitRPC(h.inner, ctx, method, argsJSON)
}
