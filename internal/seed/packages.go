package seed

import (
	"context"
	"fmt"
	"strings"

	"github.com/MajestaNet/ide/internal/metadata"
	"github.com/MajestaNet/ide/internal/packages"
)

// Package versions — bump when managed object/field definitions change.
const (
	CorePackageVersion = "2.2.0"
)

func init() {
	packages.Register(packages.Module{
		Name:              "core",
		Version:           CorePackageVersion,
		Label:             "Core",
		Description:       "Always-on User identity + Account and Contact data model",
		Optional:          false,
		DocumentationPath: "docs/modules/core.md",
		Objects: []packages.ObjectDef{
			{
				APIName: "User", Label: "User", PluralLabel: "Users",
				StorageMode: "kernel",
				Fields: []packages.FieldDef{
					{APIName: "Id", Label: "User ID", FieldType: "text", Required: true, Filterable: true, Sortable: true, KernelColumn: "id"},
					{APIName: "Username", Label: "Username", FieldType: "text", UniqueField: true, Length: intPtr(255), Filterable: true, Sortable: true, Indexed: true, KernelColumn: "user_name"},
					{APIName: "Email", Label: "Email", FieldType: "email", Required: true, UniqueField: true, Length: intPtr(255), Filterable: true, Sortable: true, Indexed: true, KernelColumn: "email"},
					{APIName: "DisplayName", Label: "Display Name", FieldType: "text", Length: intPtr(255), Filterable: true, Sortable: true, KernelColumn: "display_name"},
					{APIName: "GivenName", Label: "First Name", FieldType: "text", Length: intPtr(100), Filterable: true, Sortable: true, KernelColumn: "given_name"},
					{APIName: "FamilyName", Label: "Last Name", FieldType: "text", Length: intPtr(100), Filterable: true, Sortable: true, KernelColumn: "family_name"},
					{APIName: "Phone", Label: "Phone", FieldType: "phone", Length: intPtr(50), Filterable: true, Sortable: true, KernelColumn: "phone_number"},
					{APIName: "Locale", Label: "Locale", FieldType: "text", Length: intPtr(32), Filterable: true, Sortable: true, KernelColumn: "locale"},
					{APIName: "Timezone", Label: "Timezone", FieldType: "text", Length: intPtr(64), Filterable: true, Sortable: true, KernelColumn: "timezone"},
					{APIName: "Title", Label: "Title", FieldType: "text", Length: intPtr(128), Filterable: true, Sortable: true, KernelColumn: "title"},
					{APIName: "Department", Label: "Department", FieldType: "text", Length: intPtr(100), Filterable: true, Sortable: true, KernelColumn: "department"},
					{APIName: "EmployeeNumber", Label: "Employee Number", FieldType: "text", UniqueField: true, Length: intPtr(64), Filterable: true, Sortable: true, Indexed: true, KernelColumn: "employee_number"},
					{APIName: "ExternalId", Label: "External Id", FieldType: "text", UniqueField: true, Length: intPtr(255), Filterable: true, Sortable: true, Indexed: true, KernelColumn: "external_id"},
					{APIName: "IsActive", Label: "Active", FieldType: "boolean", Filterable: true, Sortable: true, KernelColumn: "is_active"},
					{APIName: "PrincipalType", Label: "Principal Type", FieldType: "picklist", PicklistValues: []string{"user", "service", "agent"}, Filterable: true, Sortable: true, KernelColumn: "principal_type"},
					{APIName: "DataRoleId", Label: "Data Role ID", FieldType: "text", Filterable: true, Sortable: true, KernelColumn: "data_role_id"},
				},
			},
			{
				APIName: "Account", Label: "Account", PluralLabel: "Accounts",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					{APIName: "Name", Label: "Name", FieldType: "text", Required: true, Length: intPtr(255), Filterable: true, Sortable: true, Indexed: true, Searchable: true},
					{APIName: "AccountNumber", Label: "Account Number", FieldType: "text", Length: intPtr(100), Filterable: true, Sortable: true, Indexed: true, Searchable: true},
					{APIName: "Website", Label: "Website", FieldType: "url", Length: intPtr(255), Filterable: true, Sortable: true, Searchable: true},
					{APIName: "Industry", Label: "Industry", FieldType: "text", Length: intPtr(100), Filterable: true, Sortable: true},
					{APIName: "Phone", Label: "Phone", FieldType: "phone", Length: intPtr(50), Filterable: true, Sortable: true, Searchable: true},
					{APIName: "Fax", Label: "Fax", FieldType: "phone", Length: intPtr(50), Filterable: true, Sortable: true},
					{APIName: "TickerSymbol", Label: "Ticker Symbol", FieldType: "text", Length: intPtr(20), Filterable: true, Sortable: true},
					{APIName: "Type", Label: "Type", FieldType: "picklist", PicklistValues: []string{"Prospect", "Customer", "Partner"}, Filterable: true, Sortable: true},
					{APIName: "Ownership", Label: "Ownership", FieldType: "picklist", PicklistValues: []string{"Public", "Private", "Subsidiary", "Other"}, Filterable: true, Sortable: true},
					{APIName: "Description", Label: "Description", FieldType: "textarea", Length: intPtr(32000), Filterable: false, Sortable: false},
					{APIName: "ParentAccountId", Label: "Parent Account", FieldType: "lookup", ReferenceTo: strPtr("Account"), RelationshipName: strPtr("ChildAccounts"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "PrimaryContactId", Label: "Primary Contact", FieldType: "lookup", ReferenceTo: strPtr("Contact"), RelationshipName: strPtr("PrimaryForAccounts"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "BillingStreet", Label: "Billing Street", FieldType: "text", Length: intPtr(255), Filterable: true, Sortable: true},
					{APIName: "BillingCity", Label: "Billing City", FieldType: "text", Length: intPtr(100), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "BillingState", Label: "Billing State/Province", FieldType: "text", Length: intPtr(100), Filterable: true, Sortable: true},
					{APIName: "BillingPostalCode", Label: "Billing Postal Code", FieldType: "text", Length: intPtr(50), Filterable: true, Sortable: true},
					{APIName: "BillingCountry", Label: "Billing Country", FieldType: "text", Length: intPtr(100), Filterable: true, Sortable: true},
					{APIName: "ShippingStreet", Label: "Shipping Street", FieldType: "text", Length: intPtr(255), Filterable: true, Sortable: true},
					{APIName: "ShippingCity", Label: "Shipping City", FieldType: "text", Length: intPtr(100), Filterable: true, Sortable: true},
					{APIName: "ShippingState", Label: "Shipping State/Province", FieldType: "text", Length: intPtr(100), Filterable: true, Sortable: true},
					{APIName: "ShippingPostalCode", Label: "Shipping Postal Code", FieldType: "text", Length: intPtr(50), Filterable: true, Sortable: true},
					{APIName: "ShippingCountry", Label: "Shipping Country", FieldType: "text", Length: intPtr(100), Filterable: true, Sortable: true},
				},
			},
			{
				APIName: "Contact", Label: "Contact", PluralLabel: "Contacts",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					{APIName: "Salutation", Label: "Salutation", FieldType: "picklist", PicklistValues: []string{"Mr.", "Mrs.", "Ms.", "Dr.", "Prof."}, Filterable: true, Sortable: true},
					{APIName: "FirstName", Label: "First Name", FieldType: "text", Length: intPtr(100), Filterable: true, Sortable: true, Searchable: true},
					{APIName: "MiddleName", Label: "Middle Name", FieldType: "text", Length: intPtr(100), Filterable: true, Sortable: true},
					{APIName: "LastName", Label: "Last Name", FieldType: "text", Required: true, Length: intPtr(100), Filterable: true, Sortable: true, Indexed: true, Searchable: true},
					{APIName: "Email", Label: "Email", FieldType: "email", Length: intPtr(255), Filterable: true, Sortable: true, Indexed: true, Searchable: true},
					{APIName: "JobTitle", Label: "Job Title", FieldType: "text", Length: intPtr(128), Filterable: true, Sortable: true},
					{APIName: "Department", Label: "Department", FieldType: "text", Length: intPtr(100), Filterable: true, Sortable: true},
					{APIName: "MobilePhone", Label: "Mobile Phone", FieldType: "phone", Length: intPtr(50), Filterable: true, Sortable: true, Searchable: true},
					{APIName: "HomePhone", Label: "Home Phone", FieldType: "phone", Length: intPtr(50), Filterable: true, Sortable: true, Searchable: true},
					{APIName: "Fax", Label: "Fax", FieldType: "phone", Length: intPtr(50), Filterable: true, Sortable: true},
					{APIName: "Description", Label: "Description", FieldType: "textarea", Length: intPtr(32000), Filterable: false, Sortable: false},
					{APIName: "AccountId", Label: "Account", FieldType: "lookup", ReferenceTo: strPtr("Account"), RelationshipName: strPtr("Contacts"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "MailingStreet", Label: "Mailing Street", FieldType: "text", Length: intPtr(255), Filterable: true, Sortable: true},
					{APIName: "MailingCity", Label: "Mailing City", FieldType: "text", Length: intPtr(100), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "MailingState", Label: "Mailing State/Province", FieldType: "text", Length: intPtr(100), Filterable: true, Sortable: true},
					{APIName: "MailingPostalCode", Label: "Mailing Postal Code", FieldType: "text", Length: intPtr(50), Filterable: true, Sortable: true},
					{APIName: "MailingCountry", Label: "Mailing Country", FieldType: "text", Length: intPtr(100), Filterable: true, Sortable: true},
				},
			},
		},
	})
	registerNotesModule()
	registerAgentsStarterModule()
	registerAddressModule()
	registerCatalogModule()
	registerActivitiesModule()
	registerLeadMarketingModule()
	registerSalesModule()
	registerServiceModule()
	registerCrmBridgeModule()
	registerBillingModule()
	registerHealthcareModule()
	registerFinancialServicesModule()
	registerRetailModule()
	registerSustainabilityModule()
	registerEducationModule()
	registerAutomotiveModule()
	registerNonprofitModule()
	registerMarketingEventsModule()
	registerPortalsModule()
	registerProjectServiceModule()
}

// InstallCore migrates the managed core data model (User, Account, Contact) and records package version.
func InstallCore(ctx context.Context, meta *metadata.Service) error {
	m, ok := packages.Get("core")
	if !ok {
		return fmt.Errorf("core package not registered")
	}
	if err := syncModuleDefs(ctx, meta, m); err != nil {
		return err
	}
	if err := meta.RecordPackageInstall(ctx, m.Name, m.Version); err != nil {
		return fmt.Errorf("record core package install: %w", err)
	}
	return nil
}

func syncModuleDefs(ctx context.Context, meta *metadata.Service, m packages.Module) error {
	pkg := m.Name
	// Pass 1: objects only so cross-object lookups (and self-refs) resolve in pass 2.
	for _, d := range m.Objects {
		feats := d.Features
		if feats == nil {
			feats = map[string]bool{}
		}
		mode := d.StorageMode
		if mode == "" {
			mode = "flexible"
		}
		if err := meta.SyncObjectManaged(ctx, metadata.ObjectDefinition{
			APIName: d.APIName, Label: d.Label, PluralLabel: d.PluralLabel,
			StorageMode: mode, PackageName: &pkg, Ownership: "managed", Features: feats,
		}); err != nil {
			return err
		}
	}
	// Pass 2: fields on this module's objects.
	for _, d := range m.Objects {
		for _, f := range d.Fields {
			if err := syncManagedField(ctx, meta, pkg, d.APIName, f); err != nil {
				return err
			}
		}
	}
	// Pass 3: fields on objects owned by dependency packages (bridge modules).
	for _, ext := range m.FieldExtensions {
		for _, f := range ext.Fields {
			if err := syncManagedField(ctx, meta, pkg, ext.ObjectAPIName, f); err != nil {
				return err
			}
		}
	}
	// Pass 4: managed CanvasSpecs (ADR-018 Phase 5).
	if pool := meta.Pool(); pool != nil && len(m.CanvasSpecTemplates) > 0 {
		if err := SyncCanvasSpecTemplates(ctx, pool, m); err != nil {
			return err
		}
	}
	return nil
}

func syncManagedField(ctx context.Context, meta *metadata.Service, pkg, objectAPIName string, f packages.FieldDef) error {
	fd := metadata.FieldDefinition{
		ObjectAPIName:    objectAPIName,
		APIName:          f.APIName,
		Label:            f.Label,
		FieldType:        f.FieldType,
		Required:         f.Required,
		UniqueField:      f.UniqueField,
		ExternalID:       f.ExternalID,
		Indexed:          f.Indexed,
		Filterable:       f.Filterable,
		Sortable:         f.Sortable,
		Searchable:       f.Searchable,
		Length:           f.Length,
		PicklistValues:   f.PicklistValues,
		ReferenceTo:      f.ReferenceTo,
		RelationshipName: f.RelationshipName,
		PackageName:      &pkg,
		Ownership:        "managed",
		AutonumberFormat: f.AutonumberFormat,
		AutonumberStart:  f.AutonumberStart,
	}
	if col := strings.TrimSpace(f.KernelColumn); col != "" {
		fd.KernelColumn = &col
	}
	return meta.SyncFieldManaged(ctx, fd)
}
