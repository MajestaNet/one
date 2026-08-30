package seed

import (
	"github.com/MajestaNet/ide/internal/packages"
)

const ServicePackageVersion = "2.1.0"

func registerServiceModule() {
	packages.Register(packages.Module{
		Name:              "service",
		Version:           ServicePackageVersion,
		Label:             "Service",
		Description:       "Cases, assets, entitlements, service contracts, and work orders",
		DependsOn:         []string{"core", "catalog"},
		Optional:          true,
		DocumentationPath: "docs/modules/service.md",
		Objects: []packages.ObjectDef{
			{
				APIName: "Case", Label: "Case", PluralLabel: "Cases",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					{APIName: "Subject", Label: "Subject", FieldType: "text", Required: true, Length: intPtr(255), Filterable: true, Sortable: true, Indexed: true, Searchable: true},
					{APIName: "Status", Label: "Status", FieldType: "picklist", Required: true, PicklistValues: []string{"New", "Working", "Escalated", "Closed"}, Filterable: true, Sortable: true, Indexed: true},
					{APIName: "Origin", Label: "Origin", FieldType: "picklist", PicklistValues: []string{"Email", "Phone", "Web", "Chat"}, Filterable: true, Sortable: true},
					{APIName: "Priority", Label: "Priority", FieldType: "picklist", PicklistValues: []string{"Low", "Medium", "High", "Critical"}, Filterable: true, Sortable: true, Indexed: true},
					{APIName: "Type", Label: "Type", FieldType: "picklist", PicklistValues: []string{"Question", "Problem", "Request", "Other"}, Filterable: true, Sortable: true},
					{APIName: "Reason", Label: "Reason", FieldType: "text", Length: intPtr(255), Filterable: true, Sortable: true},
					{APIName: "IsEscalated", Label: "Escalated", FieldType: "boolean", Filterable: true, Sortable: true},
					{APIName: "Description", Label: "Description", FieldType: "textarea", Length: intPtr(32000), Filterable: false, Sortable: false},
					{APIName: "AccountId", Label: "Account", FieldType: "lookup", ReferenceTo: strPtr("Account"), RelationshipName: strPtr("Cases"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "ContactId", Label: "Contact", FieldType: "lookup", ReferenceTo: strPtr("Contact"), RelationshipName: strPtr("Cases"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "AssetId", Label: "Asset", FieldType: "lookup", ReferenceTo: strPtr("Asset"), RelationshipName: strPtr("Cases"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "EntitlementId", Label: "Entitlement", FieldType: "lookup", ReferenceTo: strPtr("Entitlement"), RelationshipName: strPtr("Cases"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "ParentId", Label: "Parent Case", FieldType: "lookup", ReferenceTo: strPtr("Case"), RelationshipName: strPtr("ChildCases"), Filterable: true, Sortable: true, Indexed: true},
				},
			},
			{
				APIName: "CaseComment", Label: "Case Comment", PluralLabel: "Case Comments",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					{APIName: "ParentId", Label: "Case", FieldType: "master_detail", Required: true, ReferenceTo: strPtr("Case"), RelationshipName: strPtr("CaseComments"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "Body", Label: "Body", FieldType: "textarea", Required: true, Length: intPtr(32000), Filterable: false, Sortable: false},
					{APIName: "IsPublished", Label: "Published", FieldType: "boolean", Filterable: true, Sortable: true},
				},
			},
			{
				APIName: "Asset", Label: "Asset", PluralLabel: "Assets",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					{APIName: "Name", Label: "Name", FieldType: "text", Required: true, Length: intPtr(255), Filterable: true, Sortable: true, Indexed: true, Searchable: true},
					{APIName: "Status", Label: "Status", FieldType: "picklist", PicklistValues: []string{"Purchased", "Installed", "Registered", "Obsolete"}, Filterable: true, Sortable: true, Indexed: true},
					{APIName: "SerialNumber", Label: "Serial Number", FieldType: "text", Length: intPtr(100), Filterable: true, Sortable: true, Indexed: true, Searchable: true},
					{APIName: "InstallDate", Label: "Install Date", FieldType: "date", Filterable: true, Sortable: true},
					{APIName: "PurchaseDate", Label: "Purchase Date", FieldType: "date", Filterable: true, Sortable: true},
					{APIName: "Description", Label: "Description", FieldType: "textarea", Length: intPtr(32000), Filterable: false, Sortable: false},
					{APIName: "AccountId", Label: "Account", FieldType: "lookup", Required: true, ReferenceTo: strPtr("Account"), RelationshipName: strPtr("Assets"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "ProductId", Label: "Product", FieldType: "lookup", Required: true, ReferenceTo: strPtr("Product"), RelationshipName: strPtr("Assets"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "ContactId", Label: "Contact", FieldType: "lookup", ReferenceTo: strPtr("Contact"), RelationshipName: strPtr("Assets"), Filterable: true, Sortable: true, Indexed: true},
				},
			},
			{
				APIName: "ServiceContract", Label: "Service Contract", PluralLabel: "Service Contracts",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					{APIName: "Name", Label: "Name", FieldType: "text", Required: true, Length: intPtr(255), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "AccountId", Label: "Account", FieldType: "lookup", Required: true, ReferenceTo: strPtr("Account"), RelationshipName: strPtr("ServiceContracts"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "Status", Label: "Status", FieldType: "picklist", PicklistValues: []string{"Draft", "Active", "Expired", "Cancelled"}, Filterable: true, Sortable: true, Indexed: true},
					{APIName: "StartDate", Label: "Start Date", FieldType: "date", Filterable: true, Sortable: true},
					{APIName: "EndDate", Label: "End Date", FieldType: "date", Filterable: true, Sortable: true},
				},
			},
			{
				APIName: "Entitlement", Label: "Entitlement", PluralLabel: "Entitlements",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					{APIName: "Name", Label: "Name", FieldType: "text", Required: true, Length: intPtr(255), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "AccountId", Label: "Account", FieldType: "lookup", Required: true, ReferenceTo: strPtr("Account"), RelationshipName: strPtr("Entitlements"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "StartDate", Label: "Start Date", FieldType: "date", Filterable: true, Sortable: true},
					{APIName: "EndDate", Label: "End Date", FieldType: "date", Filterable: true, Sortable: true},
					{APIName: "Status", Label: "Status", FieldType: "picklist", PicklistValues: []string{"New", "Active", "Expired"}, Filterable: true, Sortable: true, Indexed: true},
					{APIName: "AssetId", Label: "Asset", FieldType: "lookup", ReferenceTo: strPtr("Asset"), RelationshipName: strPtr("Entitlements"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "ServiceContractId", Label: "Service Contract", FieldType: "lookup", ReferenceTo: strPtr("ServiceContract"), RelationshipName: strPtr("Entitlements"), Filterable: true, Sortable: true, Indexed: true},
				},
			},
			{
				APIName: "ContractLineItem", Label: "Contract Line Item", PluralLabel: "Contract Line Items",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					{APIName: "ServiceContractId", Label: "Service Contract", FieldType: "master_detail", Required: true, ReferenceTo: strPtr("ServiceContract"), RelationshipName: strPtr("ContractLineItems"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "ProductId", Label: "Product", FieldType: "lookup", ReferenceTo: strPtr("Product"), RelationshipName: strPtr("ContractLineItems"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "AssetId", Label: "Asset", FieldType: "lookup", ReferenceTo: strPtr("Asset"), RelationshipName: strPtr("ContractLineItems"), Filterable: true, Sortable: true, Indexed: true},
				},
			},
			{
				APIName: "WorkOrder", Label: "Work Order", PluralLabel: "Work Orders",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					{APIName: "Subject", Label: "Subject", FieldType: "text", Required: true, Length: intPtr(255), Filterable: true, Sortable: true, Indexed: true, Searchable: true},
					{APIName: "Status", Label: "Status", FieldType: "picklist", Required: true, PicklistValues: []string{"New", "In Progress", "Completed", "Closed", "Canceled"}, Filterable: true, Sortable: true, Indexed: true},
					{APIName: "Priority", Label: "Priority", FieldType: "picklist", PicklistValues: []string{"Low", "Medium", "High", "Critical"}, Filterable: true, Sortable: true, Indexed: true},
					{APIName: "Description", Label: "Description", FieldType: "textarea", Length: intPtr(32000), Filterable: false, Sortable: false},
					{APIName: "AccountId", Label: "Account", FieldType: "lookup", ReferenceTo: strPtr("Account"), RelationshipName: strPtr("WorkOrders"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "ContactId", Label: "Contact", FieldType: "lookup", ReferenceTo: strPtr("Contact"), RelationshipName: strPtr("WorkOrders"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "CaseId", Label: "Case", FieldType: "lookup", ReferenceTo: strPtr("Case"), RelationshipName: strPtr("WorkOrders"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "AssetId", Label: "Asset", FieldType: "lookup", ReferenceTo: strPtr("Asset"), RelationshipName: strPtr("WorkOrders"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "EntitlementId", Label: "Entitlement", FieldType: "lookup", ReferenceTo: strPtr("Entitlement"), RelationshipName: strPtr("WorkOrders"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "ServiceContractId", Label: "Service Contract", FieldType: "lookup", ReferenceTo: strPtr("ServiceContract"), RelationshipName: strPtr("WorkOrders"), Filterable: true, Sortable: true, Indexed: true},
				},
			},
		},
		CanvasSpecTemplates: []packages.CanvasSpecTemplate{
			{
				APIName:     "Service_Case_Queue",
				Label:       "Case Queue",
				Description: "Open Case queue with priority lanes and staged status updates.",
				LayoutJSON: `{
  "mode": "sections",
  "sections": [
    {"id": "summary", "title": "Queue", "nodeIds": ["hdr", "stat-open", "actions"]},
    {"id": "board", "title": "By status", "nodeIds": ["table", "related", "mutations"]}
  ]
}`,
				NodesJSON: `[
  {"id":"hdr","kind":"sectionHeader","title":"Case queue","props":{"subtitle":"Service package CanvasSpec"}},
  {"id":"stat-open","kind":"stat","title":"Open cases","bindingId":"bind-cases","props":{"value":0,"label":"Open cases"}},
  {"id":"actions","kind":"actionChipGroup","title":"Next steps","props":{"actions":[
    {"label":"Refresh queue","prompt":"Rerun the Case queue canvas with fresh open Cases"},
    {"label":"Prioritize Critical","prompt":"Propose status and owner updates for Critical Cases on this canvas"}
  ]}},
  {"id":"table","kind":"recordTable","title":"Open Cases","bindingId":"bind-cases","props":{"columns":[
    {"key":"Subject","label":"Subject"},
    {"key":"Status","label":"Status"},
    {"key":"Priority","label":"Priority"}
  ],"rows":[]}},
  {"id":"related","kind":"relatedList","title":"Related work orders","props":{"objectApiName":"WorkOrder","relationship":"CaseId","records":[],"columns":[
    {"key":"Subject","label":"Subject"},
    {"key":"Status","label":"Status"}
  ]}},
  {"id":"mutations","kind":"mutationProposal","title":"Staged updates","props":{"status":"pending","operations":[]}}
]`,
				BindingsJSON: `[
  {"id":"bind-cases","objectApiName":"Case","query":{"filters":[{"field":"Status","op":"ne","value":"Closed"}],"sort":[{"field":"Priority","direction":"desc"}],"limit":50}}
]`,
			},
		},
	})
}
