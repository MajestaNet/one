package seed

import "github.com/MajestaNet/ide/internal/packages"

const SustainabilityPackageVersion = "1.0.0"

func registerSustainabilityModule() {
	packages.Register(packages.Module{
		Name:              "sustainability",
		Version:           SustainabilityPackageVersion,
		Label:             "Sustainability",
		Description:       "Sustainability: facilities, emissions, materials, travel",
		DependsOn:         []string{"core"},
		Optional:          true,
		DocumentationPath: "docs/modules/sustainability.md",
		Objects: []packages.ObjectDef{
			{
				APIName: "Facility", Label: "Facility", PluralLabel: "Facilities",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					textField("FacilityType", "Facility Type", 100, true),
					textField("Country", "Country", 100, true),
					statusField(),
					accountLookup("Facilities"),
					descriptionField(),
				},
			},
			{
				APIName: "EmissionsSource", Label: "Emissions Source", PluralLabel: "Emissions Sources",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					picklistField("Scope", "Scope", []string{"Scope 1", "Scope 2", "Scope 3"}),
					statusField(),
					descriptionField(),
				},
			},
			{
				APIName: "EmissionFactor", Label: "Emission Factor", PluralLabel: "Emission Factors",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					numberField("FactorValue", "Factor Value"),
					textField("Unit", "Unit", 50, false),
					lookupField("EmissionsSourceId", "Emissions Source", "EmissionsSource", "EmissionFactors", false),
					statusField(),
					descriptionField(),
				},
			},
			{
				APIName: "Emission", Label: "Emission", PluralLabel: "Emissions",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					numberField("CO2e", "CO2e"),
					dateField("ActivityDate", "Activity Date"),
					lookupField("FacilityId", "Facility", "Facility", "Emissions", false),
					lookupField("EmissionsSourceId", "Emissions Source", "EmissionsSource", "Emissions", false),
					lookupField("EmissionFactorId", "Emission Factor", "EmissionFactor", "Emissions", false),
					accountLookup("Emissions"),
					descriptionField(),
				},
			},
			{
				APIName: "Material", Label: "Material", PluralLabel: "Materials",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					textField("MaterialType", "Material Type", 100, true),
					boolField("IsRecycled", "Recycled"),
					statusField(),
					descriptionField(),
				},
			},
			{
				APIName: "FuelType", Label: "Fuel Type", PluralLabel: "Fuel Types",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					textField("FuelCategory", "Fuel Category", 100, true),
					statusField(),
					descriptionField(),
				},
			},
			{
				APIName: "BusinessTravel", Label: "Business Travel", PluralLabel: "Business Travel",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					picklistField("TravelMode", "Travel Mode", []string{"Air", "Rail", "Car", "Hotel", "Other"}),
					dateField("TravelDate", "Travel Date"),
					numberField("Distance", "Distance"),
					contactLookup("BusinessTravel"),
					accountLookup("BusinessTravel"),
					lookupField("FacilityId", "Facility", "Facility", "BusinessTravel", false),
					descriptionField(),
				},
			},
			{
				APIName: "EmployeeCommuting", Label: "Employee Commuting", PluralLabel: "Employee Commuting",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					picklistField("CommuteMode", "Commute Mode", []string{"Car", "Transit", "Bike", "Walk", "Remote", "Other"}),
					numberField("Distance", "Distance"),
					dateField("PeriodStart", "Period Start"),
					dateField("PeriodEnd", "Period End"),
					contactLookup("EmployeeCommuting"),
					descriptionField(),
				},
			},
		},
	})
}
