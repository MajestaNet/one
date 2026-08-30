package httpapi

import (
	"context"
	"errors"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/metadata"
)

// filterDescribeGlobal keeps only objects the actor can read.
func filterDescribeGlobal(ctx context.Context, objectAz *authz.ObjectAuthz, actor *authz.Actor, desc *metadata.GlobalDescribe) (*metadata.GlobalDescribe, error) {
	if desc == nil {
		return nil, nil
	}
	if actor == nil || actor.IsAdmin || objectAz == nil {
		return desc, nil
	}
	out := *desc
	out.SObjects = make([]metadata.GlobalSObjectRef, 0, len(desc.SObjects))
	for _, ref := range desc.SObjects {
		if err := objectAz.AssertObjectAccess(ctx, actor, ref.Name, authz.ActionRead); err != nil {
			if errors.Is(err, authz.ErrForbidden) {
				continue
			}
			return nil, err
		}
		out.SObjects = append(out.SObjects, ref)
	}
	return &out, nil
}

// filterDescribeObject denies describe when the actor cannot read the object,
// and strips fields the actor cannot read (deny-by-default FLS).
func filterDescribeObject(
	ctx context.Context,
	objectAz *authz.ObjectAuthz,
	fieldAz *authz.FieldAuthz,
	actor *authz.Actor,
	desc *metadata.DescribeObject,
) (*metadata.DescribeObject, error) {
	if desc == nil {
		return nil, nil
	}
	if actor != nil && actor.IsAdmin {
		return desc, nil
	}
	if objectAz != nil {
		if err := objectAz.AssertObjectAccess(ctx, actor, desc.APIName, authz.ActionRead); err != nil {
			return nil, err
		}
	}
	if fieldAz == nil || actor == nil {
		return desc, nil
	}
	out := *desc
	fields := make([]metadata.FieldDefinition, 0, len(desc.Fields))
	for _, f := range desc.Fields {
		ok, err := fieldAz.FieldReadable(ctx, actor, desc.APIName, f.APIName)
		if err != nil {
			return nil, err
		}
		if ok {
			fields = append(fields, f)
		}
	}
	out.Fields = fields
	return &out, nil
}
