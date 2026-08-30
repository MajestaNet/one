package db

import (
	"context"

	"github.com/MajestaNet/ide/internal/authz"
)

// SharingEvalAdapter implements authz sharing loader interfaces.
type SharingEvalAdapter struct {
	Sharing   *SharingStore
	DataRoles *DataRoleStore
}

// NewSharingEvalAdapter wires sharing stores for record access evaluation.
func NewSharingEvalAdapter(pool *Pool) *SharingEvalAdapter {
	return &SharingEvalAdapter{
		Sharing:   NewSharingStore(pool),
		DataRoles: NewDataRoleStore(pool),
	}
}

// RecordSharingEnabled implements authz.SharingSettingsLoader.
func (a *SharingEvalAdapter) RecordSharingEnabled(ctx context.Context) (bool, error) {
	if a.Sharing == nil {
		return false, nil
	}
	s, err := a.Sharing.GetOrganizationSettings(ctx)
	if err != nil {
		return false, err
	}
	return s.RecordSharingEnabled, nil
}

// ObjectDefaultAccess implements authz.SharingSettingsLoader.
func (a *SharingEvalAdapter) ObjectDefaultAccess(ctx context.Context, objectAPIName string) (string, error) {
	if a.Sharing == nil {
		return authz.DefaultAccessPrivate, nil
	}
	o, err := a.Sharing.GetObjectSharingSettings(ctx, objectAPIName)
	if err != nil {
		if err == ErrNotFound {
			return authz.DefaultAccessPrivate, nil
		}
		return "", err
	}
	return o.DefaultAccess, nil
}

// LoadHierarchy implements authz.DataRoleHierarchyLoader.
func (a *SharingEvalAdapter) LoadHierarchy(ctx context.Context) (authz.HierarchyIndex, error) {
	if a.DataRoles == nil {
		return authz.HierarchyIndex{}, nil
	}
	return a.DataRoles.LoadDataRoleHierarchy(ctx)
}

// UserDataRoleID implements authz.DataRoleHierarchyLoader.
func (a *SharingEvalAdapter) UserDataRoleID(ctx context.Context, userID string) (*string, error) {
	if a.DataRoles == nil {
		return nil, nil
	}
	return a.DataRoles.GetUserDataRoleID(ctx, userID)
}

// HasRecordGrant implements authz.SharingGrantChecker.
func (a *SharingEvalAdapter) HasRecordGrant(ctx context.Context, recordID, userID string, needWrite bool) (bool, error) {
	if a.Sharing == nil {
		return false, nil
	}
	return a.Sharing.HasRecordGrant(ctx, recordID, userID, needWrite)
}

// HasRecordGrantForObject implements authz.SharingGrantChecker.
func (a *SharingEvalAdapter) HasRecordGrantForObject(ctx context.Context, objectAPIName, recordID, userID string, needWrite bool) (bool, error) {
	if a.Sharing == nil {
		return false, nil
	}
	return a.Sharing.HasRecordGrantForObject(ctx, objectAPIName, recordID, userID, needWrite)
}

// NewRecordAccessEvaluator constructs an evaluator backed by Postgres.
func NewRecordAccessEvaluator(pool *Pool) *authz.RecordAccessEvaluator {
	adapter := NewSharingEvalAdapter(pool)
	return &authz.RecordAccessEvaluator{
		Settings:  adapter,
		Hierarchy: adapter,
		Grants:    adapter,
	}
}
