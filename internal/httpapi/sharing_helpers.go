package httpapi

import (
	"context"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/dataengine"
)

func (s *Server) buildQueryVisibility(ctx context.Context, actor *authz.Actor, objectAPIName string, viewAll map[string]struct{}) (dataengine.QueryVisibility, error) {
	return dataengine.BuildQueryVisibility(ctx, s.pool, actor, objectAPIName, viewAll)
}

func (s *Server) canViewRecord(ctx context.Context, actor *authz.Actor, recordID, ownerID, createdByID, objectAPIName string, viewAll map[string]struct{}) (bool, error) {
	if s.recordAccess != nil {
		return s.recordAccess.CanViewRecordFull(ctx, actor, recordID, ownerID, createdByID, objectAPIName, viewAll, true)
	}
	return authz.CanViewRecord(actor, ownerID, createdByID, objectAPIName, viewAll), nil
}

func (s *Server) canModifyRecord(ctx context.Context, actor *authz.Actor, recordID, ownerID, createdByID, objectAPIName string, modifyAll map[string]struct{}) (bool, error) {
	if s.recordAccess != nil {
		return s.recordAccess.CanModifyRecordFull(ctx, actor, recordID, ownerID, createdByID, objectAPIName, modifyAll, true)
	}
	return authz.CanModifyRecord(actor, ownerID, createdByID, objectAPIName, modifyAll), nil
}
