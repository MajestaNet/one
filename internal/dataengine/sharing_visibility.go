package dataengine

import (
	"context"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/db"
)

// BuildQueryVisibility constructs SQL visibility predicates for list/query.
// Caller must already AssertObjectAccess(read); HasObjectRead is set true under that contract.
// When the actor is admin or has viewAll for the object (or "*"), returns zero visibility (no SQL filter).
// When record sharing is enabled, returns VisibilitySharing; otherwise VisibilityLegacy (owner/creator).
func BuildQueryVisibility(
	ctx context.Context,
	pool *db.Pool,
	actor *authz.Actor,
	objectAPIName string,
	viewAll map[string]struct{},
) (QueryVisibility, error) {
	vis := QueryVisibility{}
	if pool == nil || actor == nil {
		return vis, nil
	}
	if actor.IsAdmin {
		return vis, nil
	}
	if _, ok := viewAll["*"]; ok {
		return vis, nil
	}
	if _, ok := viewAll[objectAPIName]; ok {
		return vis, nil
	}

	sharing := db.NewSharingStore(pool)
	org, err := sharing.GetOrganizationSettings(ctx)
	if err != nil {
		return vis, err
	}

	vis.UserID = actor.ID
	vis.HasObjectRead = true

	if !org.RecordSharingEnabled {
		vis.Mode = VisibilityOwnerCreator
		return vis, nil
	}

	vis.Mode = VisibilitySharing
	obj, err := sharing.GetObjectSharingSettings(ctx, objectAPIName)
	if err != nil {
		if err == db.ErrNotFound {
			vis.DefaultAccess = authz.DefaultAccessPrivate
			return vis, nil
		}
		return vis, err
	}
	vis.DefaultAccess = obj.DefaultAccess
	if actor.DataRoleID == "" {
		if roleID, err := db.NewDataRoleStore(pool).GetUserDataRoleID(ctx, actor.ID); err == nil && roleID != nil {
			actor.DataRoleID = *roleID
		}
	}
	if actor.DataRoleID != "" {
		sub, err := db.NewDataRoleStore(pool).ListSubordinateDataRoleIDs(ctx, actor.DataRoleID)
		if err != nil {
			return vis, err
		}
		vis.SubordinateDataRoleIDs = sub
	}
	return vis, nil
}
