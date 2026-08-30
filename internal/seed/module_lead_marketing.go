package seed

import (
	"github.com/MajestaNet/ide/internal/packages"
)

const LeadMarketingPackageVersion = "1.2.0"

func registerLeadMarketingModule() {
	packages.Register(packages.Module{
		Name:              "lead_marketing",
		Version:           LeadMarketingPackageVersion,
		Label:             "Lead & Marketing",
		Description:       "Lead, Campaign, and MarketingList (not part of sales)",
		DependsOn:         []string{"core"},
		Optional:          true,
		DocumentationPath: "docs/modules/lead-marketing.md",
		Actions: []packages.ActionDef{
			leadConvertActionDef(),
		},
		Objects: []packages.ObjectDef{
			{
				APIName: "Lead", Label: "Lead", PluralLabel: "Leads",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					{APIName: "FirstName", Label: "First Name", FieldType: "text", Length: intPtr(100), Filterable: true, Sortable: true, Searchable: true},
					{APIName: "LastName", Label: "Last Name", FieldType: "text", Required: true, Length: intPtr(100), Filterable: true, Sortable: true, Indexed: true, Searchable: true},
					{APIName: "Company", Label: "Company", FieldType: "text", Length: intPtr(255), Filterable: true, Sortable: true, Indexed: true, Searchable: true},
					{APIName: "Email", Label: "Email", FieldType: "email", Length: intPtr(255), Filterable: true, Sortable: true, Indexed: true, Searchable: true},
					{APIName: "Phone", Label: "Phone", FieldType: "phone", Length: intPtr(50), Filterable: true, Sortable: true},
					{APIName: "Status", Label: "Status", FieldType: "picklist", PicklistValues: []string{"New", "Contacted", "Qualified", "Unqualified", "Converted"}, Filterable: true, Sortable: true, Indexed: true},
					{APIName: "Source", Label: "Source", FieldType: "picklist", PicklistValues: []string{"Web", "Phone", "Partner", "Campaign", "Other"}, Filterable: true, Sortable: true},
					{APIName: "Description", Label: "Description", FieldType: "textarea", Length: intPtr(32000), Filterable: false, Sortable: false},
					{APIName: "AccountId", Label: "Account", FieldType: "lookup", ReferenceTo: strPtr("Account"), RelationshipName: strPtr("Leads"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "ContactId", Label: "Contact", FieldType: "lookup", ReferenceTo: strPtr("Contact"), RelationshipName: strPtr("Leads"), Filterable: true, Sortable: true, Indexed: true},
				},
			},
			{
				APIName: "Campaign", Label: "Campaign", PluralLabel: "Campaigns",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					{APIName: "Name", Label: "Name", FieldType: "text", Required: true, Length: intPtr(255), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "Status", Label: "Status", FieldType: "picklist", PicklistValues: []string{"Proposed", "Ready", "In Progress", "Completed", "Canceled"}, Filterable: true, Sortable: true, Indexed: true},
					{APIName: "Type", Label: "Type", FieldType: "picklist", PicklistValues: []string{"Advertisement", "Email", "Event", "Other"}, Filterable: true, Sortable: true},
					{APIName: "StartDate", Label: "Start Date", FieldType: "date", Filterable: true, Sortable: true},
					{APIName: "EndDate", Label: "End Date", FieldType: "date", Filterable: true, Sortable: true},
					{APIName: "Description", Label: "Description", FieldType: "textarea", Length: intPtr(32000), Filterable: false, Sortable: false},
				},
			},
			{
				APIName: "MarketingList", Label: "Marketing List", PluralLabel: "Marketing Lists",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					{APIName: "Name", Label: "Name", FieldType: "text", Required: true, Length: intPtr(255), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "Type", Label: "Type", FieldType: "picklist", PicklistValues: []string{"Static", "Dynamic"}, Filterable: true, Sortable: true},
					{APIName: "MemberType", Label: "Member Type", FieldType: "picklist", PicklistValues: []string{"Account", "Contact", "Lead"}, Filterable: true, Sortable: true},
					{APIName: "Description", Label: "Description", FieldType: "textarea", Length: intPtr(32000), Filterable: false, Sortable: false},
				},
			},
			{
				APIName: "MarketingListMember", Label: "Marketing List Member", PluralLabel: "Marketing List Members",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					{APIName: "MarketingListId", Label: "Marketing List", FieldType: "master_detail", Required: true, ReferenceTo: strPtr("MarketingList"), RelationshipName: strPtr("MarketingListMembers"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "AccountId", Label: "Account", FieldType: "lookup", ReferenceTo: strPtr("Account"), RelationshipName: strPtr("MarketingListMembers"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "ContactId", Label: "Contact", FieldType: "lookup", ReferenceTo: strPtr("Contact"), RelationshipName: strPtr("MarketingListMembers"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "LeadId", Label: "Lead", FieldType: "lookup", ReferenceTo: strPtr("Lead"), RelationshipName: strPtr("MarketingListMembers"), Filterable: true, Sortable: true, Indexed: true},
				},
			},
		},
	})
}

func leadConvertActionDef() packages.ActionDef {
	return packages.ActionDef{
		APIName:          "lead.convert",
		Label:            "Convert Lead",
		Description:      "Convert a Lead into Account and Contact, optionally creating an Opportunity when sales is enabled.",
		RequiresPackages: []string{"lead_marketing"},
		OptionalPackages: []string{"sales"},
		Objects:          []string{"Lead", "Account", "Contact", "Opportunity"},
		SyncSafe:         true,
		InputJSONSchema: `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "additionalProperties": false,
  "required": ["leadId"],
  "properties": {
    "leadId": {"type": "string", "minLength": 1},
    "accountId": {"type": "string"},
    "contactId": {"type": "string"},
    "createOpportunity": {"type": "boolean"},
    "opportunityName": {"type": "string"},
    "opportunityCloseDate": {"type": "string", "format": "date"},
    "convertedStatus": {"type": "string"}
  }
}`,
		OutputJSONSchema: `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "additionalProperties": false,
  "required": ["leadId", "accountId", "contactId", "alreadyConverted"],
  "properties": {
    "leadId": {"type": "string"},
    "accountId": {"type": "string"},
    "contactId": {"type": "string"},
    "opportunityId": {"type": "string"},
    "alreadyConverted": {"type": "boolean"}
  }
}`,
	}
}
