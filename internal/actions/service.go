package actions

import (
	"context"
	"errors"
	"fmt"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/automation"
	"github.com/MajestaNet/ide/internal/dataengine"
	"github.com/MajestaNet/ide/internal/metadata"
	"github.com/MajestaNet/ide/internal/packages"
)

// Handler executes one platform action inside an open write transaction when syncSafe.
type Handler func(ctx context.Context, s *Service, actor *authz.Actor, input map[string]any) (map[string]any, error)

// Service is the platform-action catalog and invoke dispatcher (ADR-029).
type Service struct {
	Meta         *metadata.Service
	Data         *dataengine.Service
	ObjectAz     *authz.ObjectAuthz
	FieldAz      *authz.FieldAuthz
	RecordAccess *authz.RecordAccessEvaluator
	handlers     map[string]Handler
}

// Options wires DataEngine + AuthZ for invoke.
type Options struct {
	Meta         *metadata.Service
	Data         *dataengine.Service
	ObjectAz     *authz.ObjectAuthz
	FieldAz      *authz.FieldAuthz
	RecordAccess *authz.RecordAccessEvaluator
}

// New constructs a Service with product handlers registered.
func New(opts Options) *Service {
	s := &Service{
		Meta:         opts.Meta,
		Data:         opts.Data,
		ObjectAz:     opts.ObjectAz,
		FieldAz:      opts.FieldAz,
		RecordAccess: opts.RecordAccess,
		handlers:     map[string]Handler{},
	}
	s.handlers["lead.convert"] = convertLead
	s.handlers["quote.accept"] = acceptQuote
	return s
}

// SetHandler overrides or adds a handler (tests).
func (s *Service) SetHandler(apiName string, h Handler) {
	if s.handlers == nil {
		s.handlers = map[string]Handler{}
	}
	s.handlers[apiName] = h
}

// Catalog returns actions whose required packages are enabled on this install.
func (s *Service) Catalog(ctx context.Context) ([]CatalogItem, error) {
	all, err := packages.ActionsByName()
	if err != nil {
		return nil, err
	}
	enabled, err := s.enabledPackages(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]CatalogItem, 0)
	for _, reg := range all {
		if _, ok := requiredPackagesEnabled(reg.Def, enabled); !ok {
			continue
		}
		out = append(out, catalogItem(reg))
	}
	sortCatalog(out)
	return out, nil
}

// Describe returns one action. Unknown names are ACTION_NOT_FOUND;
// known names with a required package disabled are PACKAGE_NOT_ENABLED.
func (s *Service) Describe(ctx context.Context, apiName string) (DescribeResult, error) {
	reg, err := lookupAction(apiName)
	if err != nil {
		return DescribeResult{}, err
	}
	enabled, err := s.enabledPackages(ctx)
	if err != nil {
		return DescribeResult{}, err
	}
	if missing, ok := requiredPackagesEnabled(reg.Def, enabled); !ok {
		return DescribeResult{}, errPackageNotEnabled(missing, "")
	}
	return describeResult(reg), nil
}

// Invoke runs a platform action as actor (package gate → schema → handler).
func (s *Service) Invoke(ctx context.Context, actor *authz.Actor, apiName string, input map[string]any) (map[string]any, error) {
	if actor == nil {
		return nil, errForbidden("authentication required")
	}
	reg, err := lookupAction(apiName)
	if err != nil {
		return nil, err
	}
	enabled, err := s.enabledPackages(ctx)
	if err != nil {
		return nil, err
	}
	if missing, ok := requiredPackagesEnabled(reg.Def, enabled); !ok {
		return nil, errPackageNotEnabled(missing, "")
	}
	if input == nil {
		input = map[string]any{}
	}
	if err := validateInputSchema(reg.Def.InputJSONSchema, input); err != nil {
		return nil, err
	}
	if automation.IsSyncGuest(ctx) && !reg.Def.SyncSafe {
		return nil, errAsyncOnly()
	}
	handler := s.handlers[apiName]
	if handler == nil {
		return nil, errNotFound(apiName)
	}
	if s.Data != nil && !dataengine.InWriteTx(ctx) {
		if err := s.ensureActionObjects(ctx, apiName, reg.Def.Objects, input); err != nil {
			return nil, err
		}
	}

	run := func(ctx context.Context) (map[string]any, error) {
		out, err := handler(ctx, s, actor, input)
		return out, mapInvokeErr(err)
	}

	if s.Data == nil {
		return run(ctx)
	}
	var result map[string]any
	err = s.Data.RunInTx(ctx, func(ctx context.Context) error {
		var herr error
		result, herr = run(ctx)
		return herr
	})
	if err != nil {
		return nil, mapInvokeErr(err)
	}
	return result, nil
}

func mapInvokeErr(err error) error {
	if err == nil {
		return nil
	}
	var ae *Error
	if errors.As(err, &ae) {
		return ae
	}
	if errors.Is(err, authz.ErrForbidden) {
		return errForbidden(err.Error())
	}
	var ve *dataengine.ValidationError
	if errors.As(err, &ve) {
		return errValidation(ve.Error())
	}
	if errors.Is(err, dataengine.ErrNotFound) {
		return errValidation(err.Error())
	}
	return err
}

func (s *Service) ensureActionObjects(ctx context.Context, apiName string, names []string, input map[string]any) error {
	if s.Data == nil {
		return fmt.Errorf("data engine not configured")
	}
	createOpp := boolVal(input["createOpportunity"])
	skipOrders := false
	if apiName == "quote.accept" {
		if _, ok := input["createOrder"]; ok {
			skipOrders = !boolVal(input["createOrder"])
		} else {
			enabled, err := s.enabledPackages(ctx)
			if err != nil {
				return err
			}
			skipOrders = !enabled["billing"]
		}
	}
	for _, name := range names {
		if name == "Opportunity" && !createOpp {
			continue
		}
		if skipOrders && (name == "Order" || name == "OrderLine") {
			continue
		}
		if s.Meta != nil {
			if _, err := s.Meta.GetObject(ctx, name); err != nil {
				continue
			}
		}
		if err := s.Data.EnsurePartitions(ctx, name); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) requirePackage(ctx context.Context, packageName, option string) error {
	enabled, err := s.enabledPackages(ctx)
	if err != nil {
		return err
	}
	if !enabled[packageName] {
		return errPackageNotEnabled(packageName, option)
	}
	return nil
}
