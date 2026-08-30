package actions

import (
	"context"
	"sort"

	"github.com/MajestaNet/ide/internal/packages"
)

// CatalogItem is one invocable action for GET /client/v1/actions.
type CatalogItem struct {
	APIName          string   `json:"apiName"`
	Label            string   `json:"label"`
	Description      string   `json:"description,omitempty"`
	OwningPackage    string   `json:"owningPackage"`
	RequiresPackages []string `json:"requiresPackages"`
	OptionalPackages []string `json:"optionalPackages,omitempty"`
	Objects          []string `json:"objects,omitempty"`
	SyncSafe         bool     `json:"syncSafe"`
}

// DescribeResult is GET /client/v1/actions/{apiName}.
type DescribeResult struct {
	CatalogItem
	InputSchema  any `json:"inputSchema,omitempty"`
	OutputSchema any `json:"outputSchema,omitempty"`
}

func catalogItem(reg packages.RegisteredAction) CatalogItem {
	return CatalogItem{
		APIName:          reg.Def.APIName,
		Label:            reg.Def.Label,
		Description:      reg.Def.Description,
		OwningPackage:    reg.Module,
		RequiresPackages: append([]string{}, reg.Def.RequiresPackages...),
		OptionalPackages: append([]string{}, reg.Def.OptionalPackages...),
		Objects:          append([]string{}, reg.Def.Objects...),
		SyncSafe:         reg.Def.SyncSafe,
	}
}

func describeResult(reg packages.RegisteredAction) DescribeResult {
	return DescribeResult{
		CatalogItem:  catalogItem(reg),
		InputSchema:  parseSchemaJSON(reg.Def.InputJSONSchema),
		OutputSchema: parseSchemaJSON(reg.Def.OutputJSONSchema),
	}
}

func requiredPackagesEnabled(def packages.ActionDef, enabled map[string]bool) (string, bool) {
	for _, name := range def.RequiresPackages {
		if !enabled[name] {
			return name, false
		}
	}
	return "", true
}

func (s *Service) enabledPackages(ctx context.Context) (map[string]bool, error) {
	enabled := map[string]bool{}
	if s == nil || s.Meta == nil {
		return enabled, nil
	}
	if _, on, err := s.Meta.GetPackageInstall(ctx, "core"); err != nil {
		return nil, err
	} else if on {
		enabled["core"] = true
	}
	names, err := s.Meta.ListEnabledPackageInstalls(ctx)
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		enabled[name] = true
	}
	return enabled, nil
}

func lookupAction(apiName string) (packages.RegisteredAction, error) {
	all, err := packages.ActionsByName()
	if err != nil {
		return packages.RegisteredAction{}, err
	}
	reg, ok := all[apiName]
	if !ok {
		return packages.RegisteredAction{}, errNotFound(apiName)
	}
	return reg, nil
}

func sortCatalog(items []CatalogItem) {
	sort.Slice(items, func(i, j int) bool { return items[i].APIName < items[j].APIName })
}
