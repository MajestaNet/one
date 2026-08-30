package seed

import (
	"github.com/MajestaNet/ide/internal/packages"
)

const SalesPackageVersion = "2.2.0"

func registerSalesModule() {
	packages.Register(packages.Module{
		Name:              "sales",
		Version:           SalesPackageVersion,
		Label:             "Sales",
		Description:       "Pipeline and quoting: Opportunity, Quote, QuoteLine, Competitor (no Lead)",
		DependsOn:         []string{"core", "catalog"},
		Optional:          true,
		DocumentationPath: "docs/modules/sales.md",
		Actions: []packages.ActionDef{
			quoteAcceptActionDef(),
		},
		Objects: []packages.ObjectDef{
			{
				APIName: "Opportunity", Label: "Opportunity", PluralLabel: "Opportunities",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					{APIName: "Name", Label: "Name", FieldType: "text", Required: true, Length: intPtr(255), Filterable: true, Sortable: true, Indexed: true, Searchable: true},
					{APIName: "StageName", Label: "Stage", FieldType: "picklist", Required: true, PicklistValues: []string{"Prospecting", "Qualification", "Proposal", "Negotiation", "Closed Won", "Closed Lost"}, Filterable: true, Sortable: true, Indexed: true},
					{APIName: "CloseDate", Label: "Close Date", FieldType: "date", Required: true, Filterable: true, Sortable: true, Indexed: true},
					{APIName: "Amount", Label: "Amount", FieldType: "currency", Filterable: true, Sortable: true},
					{APIName: "Probability", Label: "Probability", FieldType: "number", Filterable: true, Sortable: true},
					{APIName: "IsClosed", Label: "Closed", FieldType: "boolean", Filterable: true, Sortable: true},
					{APIName: "IsWon", Label: "Won", FieldType: "boolean", Filterable: true, Sortable: true},
					{APIName: "Type", Label: "Type", FieldType: "picklist", PicklistValues: []string{"New Business", "Existing Business", "Renewal"}, Filterable: true, Sortable: true},
					{APIName: "LeadSource", Label: "Lead Source", FieldType: "picklist", PicklistValues: []string{"Web", "Phone", "Partner", "Campaign", "Other"}, Filterable: true, Sortable: true},
					{APIName: "NextStep", Label: "Next Step", FieldType: "text", Length: intPtr(255), Filterable: true, Sortable: true},
					{APIName: "Description", Label: "Description", FieldType: "textarea", Length: intPtr(32000), Filterable: false, Sortable: false},
					{APIName: "AccountId", Label: "Account", FieldType: "lookup", ReferenceTo: strPtr("Account"), RelationshipName: strPtr("Opportunities"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "ContactId", Label: "Contact", FieldType: "lookup", ReferenceTo: strPtr("Contact"), RelationshipName: strPtr("Opportunities"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "PrimaryQuoteId", Label: "Primary Quote", FieldType: "lookup", ReferenceTo: strPtr("Quote"), RelationshipName: strPtr("PrimaryForOpportunities"), Filterable: true, Sortable: true, Indexed: true},
				},
			},
			{
				APIName: "OpportunityContactRole", Label: "Opportunity Contact Role", PluralLabel: "Opportunity Contact Roles",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					{APIName: "OpportunityId", Label: "Opportunity", FieldType: "lookup", Required: true, ReferenceTo: strPtr("Opportunity"), RelationshipName: strPtr("OpportunityContactRoles"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "ContactId", Label: "Contact", FieldType: "lookup", Required: true, ReferenceTo: strPtr("Contact"), RelationshipName: strPtr("OpportunityContactRoles"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "Role", Label: "Role", FieldType: "picklist", PicklistValues: []string{"Decision Maker", "Influencer", "Evaluator", "Other"}, Filterable: true, Sortable: true},
					{APIName: "IsPrimary", Label: "Primary", FieldType: "boolean", Filterable: true, Sortable: true},
				},
			},
			{
				APIName: "Quote", Label: "Quote", PluralLabel: "Quotes",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					{APIName: "Name", Label: "Name", FieldType: "text", Required: true, Length: intPtr(255), Filterable: true, Sortable: true, Indexed: true, Searchable: true},
					{APIName: "Status", Label: "Status", FieldType: "picklist", Required: true, PicklistValues: []string{"Draft", "Presented", "Accepted", "Rejected", "Expired"}, Filterable: true, Sortable: true, Indexed: true},
					{APIName: "ExpirationDate", Label: "Expiration Date", FieldType: "date", Filterable: true, Sortable: true},
					{APIName: "BillingName", Label: "Billing Name", FieldType: "text", Length: intPtr(255), Filterable: true, Sortable: true},
					{APIName: "BillingStreet", Label: "Billing Street", FieldType: "text", Length: intPtr(255), Filterable: true, Sortable: true},
					{APIName: "BillingCity", Label: "Billing City", FieldType: "text", Length: intPtr(100), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "BillingState", Label: "Billing State/Province", FieldType: "text", Length: intPtr(100), Filterable: true, Sortable: true},
					{APIName: "BillingPostalCode", Label: "Billing Postal Code", FieldType: "text", Length: intPtr(50), Filterable: true, Sortable: true},
					{APIName: "BillingCountry", Label: "Billing Country", FieldType: "text", Length: intPtr(100), Filterable: true, Sortable: true},
					{APIName: "ShippingStreet", Label: "Shipping Street", FieldType: "text", Length: intPtr(255), Filterable: true, Sortable: true},
					{APIName: "ShippingCity", Label: "Shipping City", FieldType: "text", Length: intPtr(100), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "ShippingState", Label: "Shipping State/Province", FieldType: "text", Length: intPtr(100), Filterable: true, Sortable: true},
					{APIName: "ShippingPostalCode", Label: "Shipping Postal Code", FieldType: "text", Length: intPtr(50), Filterable: true, Sortable: true},
					{APIName: "ShippingCountry", Label: "Shipping Country", FieldType: "text", Length: intPtr(100), Filterable: true, Sortable: true},
					{APIName: "Subtotal", Label: "Subtotal", FieldType: "currency", Filterable: true, Sortable: true},
					{APIName: "TaxAmount", Label: "Tax Amount", FieldType: "currency", Filterable: true, Sortable: true},
					{APIName: "ShippingAmount", Label: "Shipping Amount", FieldType: "currency", Filterable: true, Sortable: true},
					{APIName: "TotalAmount", Label: "Total Amount", FieldType: "currency", Filterable: true, Sortable: true},
					{APIName: "CurrencyCode", Label: "Currency Code", FieldType: "text", Length: intPtr(3), Filterable: true, Sortable: true},
					{APIName: "AcceptedAt", Label: "Accepted At", FieldType: "datetime", Filterable: true, Sortable: true},
					{APIName: "Description", Label: "Description", FieldType: "textarea", Length: intPtr(32000), Filterable: false, Sortable: false},
					{APIName: "AccountId", Label: "Account", FieldType: "lookup", ReferenceTo: strPtr("Account"), RelationshipName: strPtr("Quotes"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "ContactId", Label: "Contact", FieldType: "lookup", ReferenceTo: strPtr("Contact"), RelationshipName: strPtr("Quotes"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "OpportunityId", Label: "Opportunity", FieldType: "lookup", ReferenceTo: strPtr("Opportunity"), RelationshipName: strPtr("Quotes"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "PriceListId", Label: "Price List", FieldType: "lookup", ReferenceTo: strPtr("PriceList"), RelationshipName: strPtr("Quotes"), Filterable: true, Sortable: true, Indexed: true},
				},
			},
			{
				APIName: "QuoteLine", Label: "Quote Line", PluralLabel: "Quote Lines",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					{APIName: "QuoteId", Label: "Quote", FieldType: "master_detail", Required: true, ReferenceTo: strPtr("Quote"), RelationshipName: strPtr("QuoteLines"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "ProductId", Label: "Product", FieldType: "lookup", Required: true, ReferenceTo: strPtr("Product"), RelationshipName: strPtr("QuoteLines"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "PriceListEntryId", Label: "Price List Entry", FieldType: "lookup", ReferenceTo: strPtr("PriceListEntry"), RelationshipName: strPtr("QuoteLines"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "UnitId", Label: "Unit", FieldType: "lookup", ReferenceTo: strPtr("Unit"), RelationshipName: strPtr("QuoteLines"), Filterable: true, Sortable: true, Indexed: true},
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
			{
				APIName: "Competitor", Label: "Competitor", PluralLabel: "Competitors",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					{APIName: "Name", Label: "Name", FieldType: "text", Required: true, Length: intPtr(255), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "Website", Label: "Website", FieldType: "url", Length: intPtr(255), Filterable: true, Sortable: true},
					{APIName: "Strengths", Label: "Strengths", FieldType: "textarea", Length: intPtr(32000), Filterable: false, Sortable: false},
					{APIName: "Weaknesses", Label: "Weaknesses", FieldType: "textarea", Length: intPtr(32000), Filterable: false, Sortable: false},
					{APIName: "AccountId", Label: "Account", FieldType: "lookup", ReferenceTo: strPtr("Account"), RelationshipName: strPtr("Competitors"), Filterable: true, Sortable: true, Indexed: true},
				},
			},
		},
		CanvasSpecTemplates: []packages.CanvasSpecTemplate{
			{
				APIName:     "Sales_Open_Pipeline",
				Label:       "Open Pipeline",
				Description: "Opportunity pipeline board by stage with staged follow-ups.",
				LayoutJSON: `{
  "mode": "sections",
  "sections": [
    {"id": "summary", "title": "Summary", "nodeIds": ["hdr", "stat-open", "actions"]},
    {"id": "board", "title": "By stage", "nodeIds": ["lane-prospect", "lane-qual", "lane-proposal", "mutations"]}
  ]
}`,
				NodesJSON: `[
  {"id":"hdr","kind":"sectionHeader","title":"Open pipeline","props":{"subtitle":"Sales package CanvasSpec"}},
  {"id":"stat-open","kind":"stat","title":"Open opportunities","bindingId":"bind-opps","props":{"value":0,"label":"Open opps"}},
  {"id":"actions","kind":"actionChipGroup","title":"Next steps","props":{"actions":[
    {"label":"Refresh board","prompt":"Rerun the open pipeline canvas with fresh Opportunity data"},
    {"label":"Stage follow-ups","prompt":"Propose follow-up updates for the top open Opportunities on this canvas"}
  ]}},
  {"id":"lane-prospect","kind":"pipelineLane","title":"Prospecting","props":{"stage":"Prospecting","cards":[]}},
  {"id":"lane-qual","kind":"pipelineLane","title":"Qualification","props":{"stage":"Qualification","cards":[]}},
  {"id":"lane-proposal","kind":"pipelineLane","title":"Proposal","props":{"stage":"Proposal","cards":[]}},
  {"id":"mutations","kind":"mutationProposal","title":"Staged updates","props":{"status":"pending","operations":[]}}
]`,
				BindingsJSON: `[
  {"id":"bind-opps","objectApiName":"Opportunity","query":{"filters":[{"field":"IsClosed","op":"eq","value":false}],"limit":50}}
]`,
			},
		},
	})
}

func quoteAcceptActionDef() packages.ActionDef {
	return packages.ActionDef{
		APIName:          "quote.accept",
		Label:            "Accept Quote",
		Description:      "Accept a Quote and optionally create an Order snapshot when billing is enabled.",
		RequiresPackages: []string{"sales", "catalog"},
		OptionalPackages: []string{"billing"},
		Objects:          []string{"Quote", "QuoteLine", "Order", "OrderLine"},
		SyncSafe:         true,
		InputJSONSchema: `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "additionalProperties": false,
  "required": ["quoteId"],
  "properties": {
    "quoteId": {"type": "string", "minLength": 1},
    "createOrder": {"type": "boolean"}
  }
}`,
		OutputJSONSchema: `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "additionalProperties": false,
  "required": ["quoteId", "alreadyAccepted"],
  "properties": {
    "quoteId": {"type": "string"},
    "orderId": {"type": "string"},
    "alreadyAccepted": {"type": "boolean"}
  }
}`,
	}
}
