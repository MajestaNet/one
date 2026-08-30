package actions

import (
	"context"
	"fmt"

	"github.com/MajestaNet/ide/internal/authz"
)

func (s *Service) assertObject(ctx context.Context, actor *authz.Actor, objectAPIName string, action authz.CrudAction) error {
	if s.ObjectAz == nil {
		if actor != nil && actor.IsAdmin {
			return nil
		}
		return errForbidden(fmt.Sprintf("not allowed to %s %s", action, objectAPIName))
	}
	if err := s.ObjectAz.AssertObjectAccess(ctx, actor, objectAPIName, action); err != nil {
		return errForbidden(err.Error())
	}
	return nil
}

func (s *Service) assertEditable(ctx context.Context, actor *authz.Actor, objectAPIName string, input map[string]any) error {
	if s.FieldAz == nil {
		return nil
	}
	if err := s.FieldAz.AssertEditableFields(ctx, actor, objectAPIName, input); err != nil {
		return errForbidden(err.Error())
	}
	return nil
}

func (s *Service) assertReadableFields(ctx context.Context, actor *authz.Actor, objectAPIName string, fields ...string) error {
	if s.FieldAz == nil {
		return nil
	}
	for _, field := range fields {
		ok, err := s.FieldAz.FieldReadable(ctx, actor, objectAPIName, field)
		if err != nil {
			return err
		}
		if !ok {
			return errForbidden(fmt.Sprintf("not allowed to read field %s.%s", objectAPIName, field))
		}
	}
	return nil
}

func (s *Service) assertViewRecord(ctx context.Context, actor *authz.Actor, rec map[string]any, objectAPIName string) error {
	if err := s.assertObject(ctx, actor, objectAPIName, authz.ActionRead); err != nil {
		return err
	}
	ok, err := s.canViewRecord(ctx, actor, rec, objectAPIName)
	if err != nil {
		return err
	}
	if !ok {
		return errForbidden("not allowed")
	}
	return nil
}

func (s *Service) assertModifyRecord(ctx context.Context, actor *authz.Actor, rec map[string]any, objectAPIName string) error {
	if err := s.assertObject(ctx, actor, objectAPIName, authz.ActionUpdate); err != nil {
		return err
	}
	ok, err := s.canModifyRecord(ctx, actor, rec, objectAPIName)
	if err != nil {
		return err
	}
	if !ok {
		return errForbidden("not allowed")
	}
	return nil
}

func (s *Service) canViewRecord(ctx context.Context, actor *authz.Actor, rec map[string]any, objectAPIName string) (bool, error) {
	viewAll := map[string]struct{}{}
	if s.ObjectAz != nil {
		var err error
		viewAll, err = s.ObjectAz.GetViewAllObjects(ctx, actor)
		if err != nil {
			return false, err
		}
	}
	id, _ := rec["Id"].(string)
	ownerID, _ := rec["OwnerId"].(string)
	createdByID, _ := rec["CreatedById"].(string)
	if s.RecordAccess != nil {
		return s.RecordAccess.CanViewRecordFull(ctx, actor, id, ownerID, createdByID, objectAPIName, viewAll, true)
	}
	return authz.CanViewRecord(actor, ownerID, createdByID, objectAPIName, viewAll), nil
}

func (s *Service) canModifyRecord(ctx context.Context, actor *authz.Actor, rec map[string]any, objectAPIName string) (bool, error) {
	modifyAll := map[string]struct{}{}
	if s.ObjectAz != nil {
		var err error
		modifyAll, err = s.ObjectAz.GetModifyAllObjects(ctx, actor)
		if err != nil {
			return false, err
		}
	}
	id, _ := rec["Id"].(string)
	ownerID, _ := rec["OwnerId"].(string)
	createdByID, _ := rec["CreatedById"].(string)
	if s.RecordAccess != nil {
		return s.RecordAccess.CanModifyRecordFull(ctx, actor, id, ownerID, createdByID, objectAPIName, modifyAll, true)
	}
	return authz.CanModifyRecord(actor, ownerID, createdByID, objectAPIName, modifyAll), nil
}
