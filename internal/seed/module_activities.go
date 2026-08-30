package seed

import (
	"github.com/MajestaNet/ide/internal/packages"
)

const ActivitiesPackageVersion = "1.1.0"

func activityCommonFields(relSuffix string) []packages.FieldDef {
	return []packages.FieldDef{
		{APIName: "Subject", Label: "Subject", FieldType: "text", Required: true, Length: intPtr(255), Filterable: true, Sortable: true, Indexed: true, Searchable: true},
		{APIName: "Status", Label: "Status", FieldType: "picklist", PicklistValues: []string{"Open", "Completed", "Canceled"}, Filterable: true, Sortable: true, Indexed: true},
		{APIName: "Priority", Label: "Priority", FieldType: "picklist", PicklistValues: []string{"Low", "Normal", "High"}, Filterable: true, Sortable: true},
		{APIName: "ScheduledStart", Label: "Scheduled Start", FieldType: "datetime", Filterable: true, Sortable: true},
		{APIName: "ScheduledEnd", Label: "Scheduled End", FieldType: "datetime", Filterable: true, Sortable: true},
		{APIName: "Description", Label: "Description", FieldType: "textarea", Length: intPtr(32000), Filterable: false, Sortable: false},
		{APIName: "RegardingAccountId", Label: "Regarding Account", FieldType: "lookup", ReferenceTo: strPtr("Account"), RelationshipName: strPtr(relSuffix + "RegardingAccount"), Filterable: true, Sortable: true, Indexed: true},
		{APIName: "RegardingContactId", Label: "Regarding Contact", FieldType: "lookup", ReferenceTo: strPtr("Contact"), RelationshipName: strPtr(relSuffix + "RegardingContact"), Filterable: true, Sortable: true, Indexed: true},
	}
}

func registerActivitiesModule() {
	packages.Register(packages.Module{
		Name:              "activities",
		Version:           ActivitiesPackageVersion,
		Label:             "Activities",
		Description:       "Task, Appointment, PhoneCall, and Email flexible CRM work items (Activity Feed; not high_volume)",
		DependsOn:         []string{"core"},
		Optional:          true,
		DocumentationPath: "docs/modules/activities.md",
		Objects: []packages.ObjectDef{
			{
				APIName: "Task", Label: "Task", PluralLabel: "Tasks",
				Features: map[string]bool{"history": true},
				Fields: append(activityCommonFields("Tasks"),
					packages.FieldDef{APIName: "DueDate", Label: "Due Date", FieldType: "date", Filterable: true, Sortable: true, Indexed: true},
					packages.FieldDef{APIName: "PercentComplete", Label: "Percent Complete", FieldType: "number", Filterable: true, Sortable: true},
				),
			},
			{
				APIName: "Appointment", Label: "Appointment", PluralLabel: "Appointments",
				Features: map[string]bool{"history": true},
				Fields: append(activityCommonFields("Appointments"),
					packages.FieldDef{APIName: "Location", Label: "Location", FieldType: "text", Length: intPtr(255), Filterable: true, Sortable: true},
				),
			},
			{
				APIName: "PhoneCall", Label: "Phone Call", PluralLabel: "Phone Calls",
				Features: map[string]bool{"history": true},
				Fields: append(activityCommonFields("PhoneCalls"),
					packages.FieldDef{APIName: "PhoneNumber", Label: "Phone Number", FieldType: "phone", Length: intPtr(50), Filterable: true, Sortable: true},
					packages.FieldDef{APIName: "Direction", Label: "Direction", FieldType: "picklist", PicklistValues: []string{"Inbound", "Outbound"}, Filterable: true, Sortable: true},
				),
			},
			{
				APIName: "Email", Label: "Email", PluralLabel: "Emails",
				Features: map[string]bool{"history": true},
				Fields: append(activityCommonFields("Emails"),
					packages.FieldDef{APIName: "FromAddress", Label: "From", FieldType: "email", Length: intPtr(255), Filterable: true, Sortable: true},
					packages.FieldDef{APIName: "ToAddress", Label: "To", FieldType: "email", Length: intPtr(255), Filterable: true, Sortable: true},
					packages.FieldDef{APIName: "CcAddress", Label: "Cc", FieldType: "text", Length: intPtr(1000), Filterable: true, Sortable: true},
				),
			},
		},
	})
}
