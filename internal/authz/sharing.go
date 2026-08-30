package authz

import (
	"context"
)

// DefaultAccess constants mirror db/object OWD values.
const (
	DefaultAccessPrivate         = "private"
	DefaultAccessPublicRead      = "public_read"
	DefaultAccessPublicReadWrite = "public_read_write"
)

// SharingContext holds install/object sharing state for evaluation.
type SharingContext struct {
	RecordSharingEnabled bool
	DefaultAccess        string
}

// RecordAccessInput is one record visibility check.
type RecordAccessInput struct {
	ActorID         string
	ActorDataRoleID string
	OwnerID         string
	OwnerDataRoleID string
	CreatedByID     string
	ObjectName      string
	HasObjectRead   bool
	HasObjectUpdate bool
}

// HierarchyIndex maps data role id -> parent id for ancestor walks.
// API Roles (`roles.parent_role_id`) are unused; sharing walks data_roles only (ADR-016).
type HierarchyIndex map[string]*string

// IsAncestorRole reports whether ancestorID is on the path from roleID up (inclusive).
func IsAncestorRole(hierarchy HierarchyIndex, ancestorID, roleID string) bool {
	if ancestorID == "" || roleID == "" {
		return false
	}
	cur := roleID
	seen := map[string]struct{}{}
	for cur != "" {
		if cur == ancestorID {
			return true
		}
		if _, ok := seen[cur]; ok {
			break
		}
		seen[cur] = struct{}{}
		parent := hierarchy[cur]
		if parent == nil {
			break
		}
		cur = *parent
	}
	return false
}

// CanViewWithSharing evaluates record read access when sharing is enabled.
func CanViewWithSharing(in RecordAccessInput, sh SharingContext, hierarchy HierarchyIndex, hasGrant bool) bool {
	if !sh.RecordSharingEnabled {
		return false
	}
	if in.ActorID != "" && in.OwnerID != "" && in.ActorID == in.OwnerID {
		return true
	}
	if in.ActorID != "" && in.CreatedByID != "" && in.ActorID == in.CreatedByID {
		return true
	}
	switch sh.DefaultAccess {
	case DefaultAccessPublicRead, DefaultAccessPublicReadWrite:
		if in.HasObjectRead {
			return true
		}
	}
	if in.OwnerID != "" && in.ActorDataRoleID != "" && in.OwnerDataRoleID != "" {
		if IsAncestorRole(hierarchy, in.ActorDataRoleID, in.OwnerDataRoleID) {
			return true
		}
	}
	return hasGrant
}

// CanModifyWithSharing evaluates record write access when sharing is enabled.
func CanModifyWithSharing(in RecordAccessInput, sh SharingContext, hierarchy HierarchyIndex, hasReadWriteGrant bool) bool {
	if !sh.RecordSharingEnabled {
		return false
	}
	if in.ActorID != "" && in.OwnerID != "" && in.ActorID == in.OwnerID {
		return true
	}
	if in.OwnerID == "" && in.ActorID != "" && in.CreatedByID != "" && in.ActorID == in.CreatedByID {
		return true
	}
	if sh.DefaultAccess == DefaultAccessPublicReadWrite && in.HasObjectUpdate {
		return true
	}
	if in.OwnerID != "" && in.ActorDataRoleID != "" && in.OwnerDataRoleID != "" {
		if IsAncestorRole(hierarchy, in.ActorDataRoleID, in.OwnerDataRoleID) {
			return true
		}
	}
	return hasReadWriteGrant
}

// SharingGrantChecker tests materialized record_access_grants.
type SharingGrantChecker interface {
	HasRecordGrant(ctx context.Context, recordID, userID string, needWrite bool) (bool, error)
	HasRecordGrantForObject(ctx context.Context, objectAPIName, recordID, userID string, needWrite bool) (bool, error)
}

// SharingSettingsLoader loads org and object sharing settings.
type SharingSettingsLoader interface {
	RecordSharingEnabled(ctx context.Context) (bool, error)
	ObjectDefaultAccess(ctx context.Context, objectAPIName string) (string, error)
}

// DataRoleHierarchyLoader loads role hierarchy and user data roles.
type DataRoleHierarchyLoader interface {
	LoadHierarchy(ctx context.Context) (HierarchyIndex, error)
	UserDataRoleID(ctx context.Context, userID string) (*string, error)
}

// RecordAccessEvaluator combines legacy and sharing visibility.
type RecordAccessEvaluator struct {
	Settings  SharingSettingsLoader
	Hierarchy DataRoleHierarchyLoader
	Grants    SharingGrantChecker
}

// CanViewRecordFull evaluates visibility including sharing when enabled.
func (e *RecordAccessEvaluator) CanViewRecordFull(
	ctx context.Context,
	actor *Actor,
	recordID, ownerID, createdByID, objectAPIName string,
	viewAll map[string]struct{},
	hasObjectRead bool,
) (bool, error) {
	if CanViewRecord(actor, ownerID, createdByID, objectAPIName, viewAll) {
		return true, nil
	}
	if e == nil || e.Settings == nil {
		return false, nil
	}
	enabled, err := e.Settings.RecordSharingEnabled(ctx)
	if err != nil || !enabled {
		return false, err
	}
	defAccess, err := e.ObjectDefaultAccess(ctx, objectAPIName)
	if err != nil {
		return false, err
	}
	in, hierarchy, hasGrant, err := e.buildInput(ctx, actor, recordID, ownerID, createdByID, objectAPIName, hasObjectRead, false)
	if err != nil {
		return false, err
	}
	sh := SharingContext{RecordSharingEnabled: true, DefaultAccess: defAccess}
	return CanViewWithSharing(in, sh, hierarchy, hasGrant), nil
}

// CanModifyRecordFull evaluates modify access including sharing when enabled.
func (e *RecordAccessEvaluator) CanModifyRecordFull(
	ctx context.Context,
	actor *Actor,
	recordID, ownerID, createdByID, objectAPIName string,
	modifyAll map[string]struct{},
	hasObjectUpdate bool,
) (bool, error) {
	if CanModifyRecord(actor, ownerID, createdByID, objectAPIName, modifyAll) {
		return true, nil
	}
	if e == nil || e.Settings == nil {
		return false, nil
	}
	enabled, err := e.Settings.RecordSharingEnabled(ctx)
	if err != nil || !enabled {
		return false, err
	}
	defAccess, err := e.ObjectDefaultAccess(ctx, objectAPIName)
	if err != nil {
		return false, err
	}
	in, hierarchy, _, err := e.buildInput(ctx, actor, recordID, ownerID, createdByID, objectAPIName, false, hasObjectUpdate)
	if err != nil {
		return false, err
	}
	var hasRW bool
	if e.Grants != nil && recordID != "" && actor != nil {
		hasRW, err = e.Grants.HasRecordGrantForObject(ctx, objectAPIName, recordID, actor.ID, true)
		if err != nil {
			return false, err
		}
	}
	sh := SharingContext{RecordSharingEnabled: true, DefaultAccess: defAccess}
	return CanModifyWithSharing(in, sh, hierarchy, hasRW), nil
}

func (e *RecordAccessEvaluator) ObjectDefaultAccess(ctx context.Context, objectAPIName string) (string, error) {
	if e.Settings == nil {
		return DefaultAccessPrivate, nil
	}
	return e.Settings.ObjectDefaultAccess(ctx, objectAPIName)
}

func (e *RecordAccessEvaluator) buildInput(
	ctx context.Context,
	actor *Actor,
	recordID, ownerID, createdByID, objectAPIName string,
	hasObjectRead, hasObjectUpdate bool,
) (RecordAccessInput, HierarchyIndex, bool, error) {
	in := RecordAccessInput{
		ObjectName:      objectAPIName,
		OwnerID:         ownerID,
		CreatedByID:     createdByID,
		HasObjectRead:   hasObjectRead,
		HasObjectUpdate: hasObjectUpdate,
	}
	if actor != nil {
		in.ActorID = actor.ID
		in.ActorDataRoleID = actor.DataRoleID
	}
	var hierarchy HierarchyIndex
	if e.Hierarchy != nil {
		var err error
		hierarchy, err = e.Hierarchy.LoadHierarchy(ctx)
		if err != nil {
			return in, nil, false, err
		}
		if in.ActorDataRoleID == "" && actor != nil {
			roleID, err := e.Hierarchy.UserDataRoleID(ctx, actor.ID)
			if err != nil {
				return in, hierarchy, false, err
			}
			if roleID != nil {
				in.ActorDataRoleID = *roleID
				actor.DataRoleID = *roleID
			}
		}
		if ownerID != "" {
			roleID, err := e.Hierarchy.UserDataRoleID(ctx, ownerID)
			if err != nil {
				return in, hierarchy, false, err
			}
			if roleID != nil {
				in.OwnerDataRoleID = *roleID
			}
		}
	}
	var hasGrant bool
	if e.Grants != nil && recordID != "" && actor != nil {
		var err error
		hasGrant, err = e.Grants.HasRecordGrantForObject(ctx, objectAPIName, recordID, actor.ID, false)
		if err != nil {
			return in, hierarchy, false, err
		}
	}
	return in, hierarchy, hasGrant, nil
}
