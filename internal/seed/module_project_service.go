package seed

import "github.com/MajestaNet/ide/internal/packages"

const ProjectServicePackageVersion = "1.0.0"

func registerProjectServiceModule() {
	packages.Register(packages.Module{
		Name:              "project_service",
		Version:           ProjectServicePackageVersion,
		Label:             "Project Service",
		Description:       "Project service: projects, resources, time, expense, estimates",
		DependsOn:         []string{"core"},
		Optional:          true,
		DocumentationPath: "docs/modules/project-service.md",
		Objects: []packages.ObjectDef{
			{
				APIName: "Project", Label: "Project", PluralLabel: "Projects",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					statusField("Proposed", "Active", "On Hold", "Completed", "Cancelled"),
					dateField("StartDate", "Start Date"),
					dateField("EndDate", "End Date"),
					currencyField("BudgetAmount", "Budget Amount"),
					accountLookup("Projects"),
					contactLookup("Projects"),
					descriptionField(),
				},
			},
			{
				APIName: "ProjectTask", Label: "Project Task", PluralLabel: "Project Tasks",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					lookupField("ProjectId", "Project", "Project", "ProjectTasks", true),
					statusField("Not Started", "In Progress", "Completed", "Cancelled"),
					dateField("StartDate", "Start Date"),
					dateField("DueDate", "Due Date"),
					numberField("PercentComplete", "Percent Complete"),
					descriptionField(),
				},
			},
			{
				APIName: "Characteristic", Label: "Characteristic", PluralLabel: "Characteristics",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					picklistField("CharacteristicType", "Type", []string{"Skill", "Certification", "Role", "Other"}),
					statusField(),
					descriptionField(),
				},
			},
			{
				APIName: "BookableResource", Label: "Bookable Resource", PluralLabel: "Bookable Resources",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					picklistField("ResourceType", "Resource Type", []string{"User", "Contact", "Equipment", "Account", "Crew"}),
					statusField(),
					contactLookup("BookableResources"),
					accountLookup("BookableResources"),
					descriptionField(),
				},
			},
			{
				APIName: "TimeEntry", Label: "Time Entry", PluralLabel: "Time Entries",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					lookupField("ProjectId", "Project", "Project", "TimeEntries", true),
					lookupField("ProjectTaskId", "Project Task", "ProjectTask", "TimeEntries", false),
					lookupField("BookableResourceId", "Bookable Resource", "BookableResource", "TimeEntries", false),
					dateField("EntryDate", "Entry Date"),
					numberField("Hours", "Hours"),
					statusField("Draft", "Submitted", "Approved", "Rejected"),
					descriptionField(),
				},
			},
			{
				APIName: "Expense", Label: "Expense", PluralLabel: "Expenses",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					lookupField("ProjectId", "Project", "Project", "Expenses", true),
					currencyField("Amount", "Amount"),
					dateField("ExpenseDate", "Expense Date"),
					picklistField("Category", "Category", []string{"Travel", "Meals", "Supplies", "Other"}),
					statusField("Draft", "Submitted", "Approved", "Rejected", "Paid"),
					descriptionField(),
				},
			},
			{
				APIName: "Estimate", Label: "Estimate", PluralLabel: "Estimates",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					lookupField("ProjectId", "Project", "Project", "Estimates", true),
					currencyField("Amount", "Amount"),
					numberField("Hours", "Hours"),
					statusField("Draft", "Submitted", "Approved", "Rejected"),
					descriptionField(),
				},
			},
		},
	})
}
