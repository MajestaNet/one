package seed

import "github.com/MajestaNet/ide/internal/packages"

const HealthcarePackageVersion = "1.0.0"

func registerHealthcareModule() {
	packages.Register(packages.Module{
		Name:              "healthcare",
		Version:           HealthcarePackageVersion,
		Label:             "Healthcare",
		Description:       "Clinical spine: Patient, Practitioner, CarePlan, Encounter, and related records",
		DependsOn:         []string{"core"},
		Optional:          true,
		DocumentationPath: "docs/modules/healthcare.md",
		Objects: []packages.ObjectDef{
			{
				APIName: "Patient", Label: "Patient", PluralLabel: "Patients",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					textField("MedicalRecordNumber", "MRN", 100, true),
					statusField("Active", "Inactive", "Deceased"),
					dateField("BirthDate", "Birth Date"),
					picklistField("Gender", "Gender", []string{"Female", "Male", "Other", "Unknown"}),
					contactLookup("Patients"),
					descriptionField(),
				},
			},
			{
				APIName: "Practitioner", Label: "Practitioner", PluralLabel: "Practitioners",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					textField("NPI", "NPI", 50, true),
					textField("Specialty", "Specialty", 100, true),
					statusField(),
					contactLookup("Practitioners"),
					descriptionField(),
				},
			},
			{
				APIName: "CarePlan", Label: "Care Plan", PluralLabel: "Care Plans",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					statusField("Draft", "Active", "On Hold", "Completed", "Cancelled"),
					dateField("StartDate", "Start Date"),
					dateField("EndDate", "End Date"),
					lookupField("PatientId", "Patient", "Patient", "CarePlans", true),
					lookupField("PractitionerId", "Practitioner", "Practitioner", "CarePlans", false),
					descriptionField(),
				},
			},
			{
				APIName: "Encounter", Label: "Encounter", PluralLabel: "Encounters",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					statusField("Planned", "In Progress", "Finished", "Cancelled"),
					picklistField("Class", "Class", []string{"Ambulatory", "Emergency", "Inpatient", "Virtual", "Other"}),
					dateField("StartDate", "Start Date"),
					dateField("EndDate", "End Date"),
					lookupField("PatientId", "Patient", "Patient", "Encounters", true),
					lookupField("PractitionerId", "Practitioner", "Practitioner", "Encounters", false),
					descriptionField(),
				},
			},
			{
				APIName: "Condition", Label: "Condition", PluralLabel: "Conditions",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					textField("Code", "Code", 100, true),
					statusField("Active", "Recurrence", "Relapse", "Inactive", "Remission", "Resolved"),
					dateField("OnsetDate", "Onset Date"),
					lookupField("PatientId", "Patient", "Patient", "Conditions", true),
					lookupField("EncounterId", "Encounter", "Encounter", "Conditions", false),
					descriptionField(),
				},
			},
			{
				APIName: "AllergyIntolerance", Label: "Allergy Intolerance", PluralLabel: "Allergy Intolerances",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					textField("Code", "Code", 100, true),
					picklistField("Criticality", "Criticality", []string{"Low", "High", "Unable to Assess"}),
					statusField("Active", "Inactive", "Resolved"),
					lookupField("PatientId", "Patient", "Patient", "AllergyIntolerances", true),
					descriptionField(),
				},
			},
			{
				APIName: "Observation", Label: "Observation", PluralLabel: "Observations",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					textField("Code", "Code", 100, true),
					textField("Value", "Value", 255, false),
					textField("Unit", "Unit", 50, false),
					statusField("Registered", "Preliminary", "Final", "Amended", "Cancelled"),
					dateField("EffectiveDate", "Effective Date"),
					lookupField("PatientId", "Patient", "Patient", "Observations", true),
					lookupField("EncounterId", "Encounter", "Encounter", "Observations", false),
				},
			},
			{
				APIName: "MedicationRequest", Label: "Medication Request", PluralLabel: "Medication Requests",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					textField("Medication", "Medication", 255, true),
					statusField("Active", "On Hold", "Cancelled", "Completed", "Stopped"),
					dateField("AuthoredOn", "Authored On"),
					lookupField("PatientId", "Patient", "Patient", "MedicationRequests", true),
					lookupField("PractitionerId", "Practitioner", "Practitioner", "MedicationRequests", false),
					lookupField("EncounterId", "Encounter", "Encounter", "MedicationRequests", false),
					descriptionField(),
				},
			},
		},
	})
}
