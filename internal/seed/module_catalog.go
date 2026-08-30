package seed

import (
	"github.com/MajestaNet/ide/internal/packages"
)

const CatalogPackageVersion = "2.2.0"

func registerCatalogModule() {
	packages.Register(packages.Module{
		Name:              "catalog",
		Version:           CatalogPackageVersion,
		Label:             "Catalog",
		Description:       "Thin shared product catalog: Product, PriceList, PriceListEntry, Unit, UnitGroup",
		DependsOn:         []string{"core"},
		Optional:          true,
		DocumentationPath: "docs/modules/catalog.md",
		Objects: []packages.ObjectDef{
			{
				APIName: "UnitGroup", Label: "Unit Group", PluralLabel: "Unit Groups",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					{APIName: "Name", Label: "Name", FieldType: "text", Required: true, Length: intPtr(255), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "Description", Label: "Description", FieldType: "textarea", Length: intPtr(32000), Filterable: false, Sortable: false},
				},
			},
			{
				APIName: "Unit", Label: "Unit", PluralLabel: "Units",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					{APIName: "Name", Label: "Name", FieldType: "text", Required: true, Length: intPtr(255), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "UnitGroupId", Label: "Unit Group", FieldType: "master_detail", Required: true, ReferenceTo: strPtr("UnitGroup"), RelationshipName: strPtr("Units"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "Quantity", Label: "Quantity", FieldType: "number", Filterable: true, Sortable: true},
					{APIName: "IsBaseUnit", Label: "Base Unit", FieldType: "boolean", Filterable: true, Sortable: true},
				},
			},
			{
				APIName: "Product", Label: "Product", PluralLabel: "Products",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					{APIName: "Name", Label: "Name", FieldType: "text", Required: true, Length: intPtr(255), Filterable: true, Sortable: true, Indexed: true, Searchable: true},
					{APIName: "ProductCode", Label: "Product Code", FieldType: "text", Length: intPtr(100), Filterable: true, Sortable: true, Indexed: true, Searchable: true},
					{APIName: "StockKeepingUnit", Label: "SKU", FieldType: "text", Length: intPtr(100), Filterable: true, Sortable: true, Indexed: true, Searchable: true},
					{APIName: "IsActive", Label: "Active", FieldType: "boolean", Filterable: true, Sortable: true},
					{APIName: "Family", Label: "Product Family", FieldType: "text", Length: intPtr(100), Filterable: true, Sortable: true},
					{APIName: "ProductType", Label: "Product Type", FieldType: "picklist", PicklistValues: []string{"Good", "Service", "Subscription"}, Filterable: true, Sortable: true},
					{APIName: "Description", Label: "Description", FieldType: "textarea", Length: intPtr(32000), Filterable: false, Sortable: false},
					{APIName: "ProductURL", Label: "Product URL", FieldType: "url", Length: intPtr(255), Filterable: true, Sortable: true},
					{APIName: "QuantityUnitOfMeasureId", Label: "Unit of Measure", FieldType: "lookup", ReferenceTo: strPtr("Unit"), RelationshipName: strPtr("Products"), Filterable: true, Sortable: true, Indexed: true},
				},
			},
			{
				APIName: "PriceList", Label: "Price List", PluralLabel: "Price Lists",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					{APIName: "Name", Label: "Name", FieldType: "text", Required: true, Length: intPtr(255), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "IsActive", Label: "Active", FieldType: "boolean", Filterable: true, Sortable: true},
					{APIName: "IsStandard", Label: "Standard", FieldType: "boolean", Filterable: true, Sortable: true},
					{APIName: "CurrencyCode", Label: "Currency Code", FieldType: "text", Length: intPtr(3), Filterable: true, Sortable: true},
					{APIName: "Description", Label: "Description", FieldType: "textarea", Length: intPtr(32000), Filterable: false, Sortable: false},
					{APIName: "BeginDate", Label: "Begin Date", FieldType: "date", Filterable: true, Sortable: true},
					{APIName: "EndDate", Label: "End Date", FieldType: "date", Filterable: true, Sortable: true},
				},
			},
			{
				APIName: "PriceListEntry", Label: "Price List Entry", PluralLabel: "Price List Entries",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					{APIName: "PriceListId", Label: "Price List", FieldType: "master_detail", Required: true, ReferenceTo: strPtr("PriceList"), RelationshipName: strPtr("PriceListEntries"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "ProductId", Label: "Product", FieldType: "lookup", Required: true, ReferenceTo: strPtr("Product"), RelationshipName: strPtr("PriceListEntries"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "UnitId", Label: "Unit", FieldType: "lookup", ReferenceTo: strPtr("Unit"), RelationshipName: strPtr("PriceListEntries"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "ListPrice", Label: "List Price", FieldType: "currency", Required: true, Filterable: true, Sortable: true},
					{APIName: "IsActive", Label: "Active", FieldType: "boolean", Filterable: true, Sortable: true},
				},
			},
		},
	})
}
