package seed

import "github.com/MajestaNet/ide/internal/packages"

const RetailPackageVersion = "1.0.0"

func registerRetailModule() {
	packages.Register(packages.Module{
		Name:              "retail",
		Version:           RetailPackageVersion,
		Label:             "Retail",
		Description:       "Retail: loyalty, brands, customer assets, appointments, surveys",
		DependsOn:         []string{"core"},
		Optional:          true,
		DocumentationPath: "docs/modules/retail.md",
		Objects: []packages.ObjectDef{
			{
				APIName: "LoyaltyProgram", Label: "Loyalty Program", PluralLabel: "Loyalty Programs",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					statusField(),
					boolField("IsActive", "Active"),
					descriptionField(),
				},
			},
			{
				APIName: "LoyaltyAccount", Label: "Loyalty Account", PluralLabel: "Loyalty Accounts",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					textField("MembershipNumber", "Membership Number", 100, true),
					numberField("PointsBalance", "Points Balance"),
					statusField(),
					lookupField("LoyaltyProgramId", "Loyalty Program", "LoyaltyProgram", "LoyaltyAccounts", true),
					accountLookup("LoyaltyAccounts"),
					contactLookup("LoyaltyAccounts"),
				},
			},
			{
				APIName: "LoyaltyCard", Label: "Loyalty Card", PluralLabel: "Loyalty Cards",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					textField("CardNumber", "Card Number", 100, true),
					statusField("Active", "Blocked", "Expired"),
					lookupField("LoyaltyAccountId", "Loyalty Account", "LoyaltyAccount", "LoyaltyCards", true),
					dateField("ExpiryDate", "Expiry Date"),
				},
			},
			{
				APIName: "CustomerAsset", Label: "Customer Asset", PluralLabel: "Customer Assets",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					textField("SerialNumber", "Serial Number", 100, true),
					statusField("Owned", "Registered", "Returned", "Disposed"),
					dateField("PurchaseDate", "Purchase Date"),
					accountLookup("CustomerAssets"),
					contactLookup("CustomerAssets"),
					descriptionField(),
				},
			},
			{
				APIName: "ProductBrand", Label: "Product Brand", PluralLabel: "Product Brands",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					statusField(),
					descriptionField(),
				},
			},
			{
				APIName: "ProductCategory", Label: "Product Category", PluralLabel: "Product Categories",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					textField("CategoryCode", "Category Code", 100, true),
					lookupField("ParentCategoryId", "Parent Category", "ProductCategory", "ChildCategories", false),
					statusField(),
					descriptionField(),
				},
			},
			{
				APIName: "RetailAppointment", Label: "Retail Appointment", PluralLabel: "Retail Appointments",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					statusField("Scheduled", "Completed", "Cancelled", "No Show"),
					dateField("ScheduledDate", "Scheduled Date"),
					textField("Location", "Location", 255, false),
					accountLookup("RetailAppointments"),
					contactLookup("RetailAppointments"),
					descriptionField(),
				},
			},
			{
				APIName: "SurveyDefinition", Label: "Survey Definition", PluralLabel: "Survey Definitions",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					statusField("Draft", "Published", "Retired"),
					descriptionField(),
				},
			},
			{
				APIName: "SurveyResponse", Label: "Survey Response", PluralLabel: "Survey Responses",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					lookupField("SurveyDefinitionId", "Survey Definition", "SurveyDefinition", "SurveyResponses", true),
					numberField("Score", "Score"),
					dateField("ResponseDate", "Response Date"),
					accountLookup("SurveyResponses"),
					contactLookup("SurveyResponses"),
					descriptionField(),
				},
			},
		},
	})
}
