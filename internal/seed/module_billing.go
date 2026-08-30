package seed

import (
	"github.com/MajestaNet/ide/internal/packages"
)

const BillingPackageVersion = "1.0.0"

func registerBillingModule() {
	packages.Register(packages.Module{
		Name:              "billing",
		Version:           BillingPackageVersion,
		Label:             "Billing",
		Description:       "Order from accepted Quote: Order, OrderLine (Invoice later)",
		DependsOn:         []string{"catalog", "sales"},
		Optional:          true,
		DocumentationPath: "docs/modules/billing.md",
		FieldExtensions: []packages.FieldExtension{
			{
				ObjectAPIName: "Quote",
				Fields: []packages.FieldDef{
					{APIName: "OrderId", Label: "Order", FieldType: "lookup", ReferenceTo: strPtr("Order"), RelationshipName: strPtr("Quotes"), Filterable: true, Sortable: true, Indexed: true},
				},
			},
		},
		Objects: []packages.ObjectDef{
			{
				APIName: "Order", Label: "Order", PluralLabel: "Orders",
				Features: map[string]bool{"history": true},
				Fields: append([]packages.FieldDef{
					{APIName: "Name", Label: "Name", FieldType: "text", Required: true, Length: intPtr(255), Filterable: true, Sortable: true, Indexed: true, Searchable: true},
					{APIName: "OrderNumber", Label: "Order Number", FieldType: "autonumber", Filterable: true, Sortable: true, Indexed: true, Searchable: true, AutonumberFormat: strPtr("ORD-{00000}"), AutonumberStart: intPtr(1)},
					{APIName: "Status", Label: "Status", FieldType: "picklist", Required: true, PicklistValues: []string{"Draft", "Activated", "Fulfilled", "Cancelled"}, Filterable: true, Sortable: true, Indexed: true},
					{APIName: "AccountId", Label: "Account", FieldType: "lookup", ReferenceTo: strPtr("Account"), RelationshipName: strPtr("Orders"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "ContactId", Label: "Contact", FieldType: "lookup", ReferenceTo: strPtr("Contact"), RelationshipName: strPtr("Orders"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "OpportunityId", Label: "Opportunity", FieldType: "lookup", ReferenceTo: strPtr("Opportunity"), RelationshipName: strPtr("Orders"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "QuoteId", Label: "Quote", FieldType: "lookup", ReferenceTo: strPtr("Quote"), RelationshipName: strPtr("Orders"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "PriceListId", Label: "Price List", FieldType: "lookup", ReferenceTo: strPtr("PriceList"), RelationshipName: strPtr("Orders"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "CurrencyCode", Label: "Currency Code", FieldType: "text", Length: intPtr(3), Filterable: true, Sortable: true},
					{APIName: "Subtotal", Label: "Subtotal", FieldType: "currency", Filterable: true, Sortable: true},
					{APIName: "TaxAmount", Label: "Tax Amount", FieldType: "currency", Filterable: true, Sortable: true},
					{APIName: "ShippingAmount", Label: "Shipping Amount", FieldType: "currency", Filterable: true, Sortable: true},
					{APIName: "TotalAmount", Label: "Total Amount", FieldType: "currency", Filterable: true, Sortable: true},
					{APIName: "EffectiveDate", Label: "Effective Date", FieldType: "date", Filterable: true, Sortable: true, Indexed: true},
					{APIName: "ActivatedAt", Label: "Activated At", FieldType: "datetime", Filterable: true, Sortable: true},
					{APIName: "Description", Label: "Description", FieldType: "textarea", Length: intPtr(32000), Filterable: false, Sortable: false},
				}, append(streetAddressFields("Billing", "Billing"), streetAddressFields("Shipping", "Shipping")...)...),
			},
			{
				APIName: "OrderLine", Label: "Order Line", PluralLabel: "Order Lines",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					{APIName: "OrderId", Label: "Order", FieldType: "master_detail", Required: true, ReferenceTo: strPtr("Order"), RelationshipName: strPtr("OrderLines"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "QuoteLineId", Label: "Quote Line", FieldType: "lookup", ReferenceTo: strPtr("QuoteLine"), RelationshipName: strPtr("OrderLines"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "ProductId", Label: "Product", FieldType: "lookup", Required: true, ReferenceTo: strPtr("Product"), RelationshipName: strPtr("OrderLines"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "PriceListEntryId", Label: "Price List Entry", FieldType: "lookup", ReferenceTo: strPtr("PriceListEntry"), RelationshipName: strPtr("OrderLines"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "UnitId", Label: "Unit", FieldType: "lookup", ReferenceTo: strPtr("Unit"), RelationshipName: strPtr("OrderLines"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "LineNumber", Label: "Line Number", FieldType: "number", Filterable: true, Sortable: true},
					{APIName: "Quantity", Label: "Quantity", FieldType: "number", Required: true, Filterable: true, Sortable: true},
					{APIName: "ListPrice", Label: "List Price", FieldType: "currency", Filterable: true, Sortable: true},
					{APIName: "UnitPrice", Label: "Unit Price", FieldType: "currency", Filterable: true, Sortable: true},
					{APIName: "DiscountPercent", Label: "Discount Percent", FieldType: "number", Filterable: true, Sortable: true},
					{APIName: "Amount", Label: "Amount", FieldType: "currency", Filterable: true, Sortable: true},
					{APIName: "Description", Label: "Description", FieldType: "textarea", Length: intPtr(32000), Filterable: false, Sortable: false},
					{APIName: "PriceSource", Label: "Price Source", FieldType: "picklist", PicklistValues: []string{"PriceList", "Manual", "Contract", "CpqRule", "External"}, Filterable: true, Sortable: true},
				},
			},
		},
	})
}
