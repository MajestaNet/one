package seed

import "github.com/MajestaNet/ide/internal/packages"

const NonprofitPackageVersion = "1.0.0"

func registerNonprofitModule() {
	packages.Register(packages.Module{
		Name:              "nonprofit",
		Version:           NonprofitPackageVersion,
		Label:             "Nonprofit",
		Description:       "Nonprofit: donor commitments, designations, awards, disbursements",
		DependsOn:         []string{"core"},
		Optional:          true,
		DocumentationPath: "docs/modules/nonprofit.md",
		Objects: []packages.ObjectDef{
			{
				APIName: "Designation", Label: "Designation", PluralLabel: "Designations",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					textField("Code", "Code", 50, true),
					statusField(),
					descriptionField(),
				},
			},
			{
				APIName: "DonorCommitment", Label: "Donor Commitment", PluralLabel: "Donor Commitments",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					currencyField("Amount", "Amount"),
					statusField("Pledged", "Active", "Fulfilled", "Cancelled"),
					dateField("PledgeDate", "Pledge Date"),
					accountLookup("DonorCommitments"),
					contactLookup("DonorCommitments"),
					lookupField("DesignationId", "Designation", "Designation", "DonorCommitments", false),
					descriptionField(),
				},
			},
			{
				APIName: "Award", Label: "Award", PluralLabel: "Awards",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					currencyField("Amount", "Amount"),
					statusField("Proposed", "Approved", "Disbursed", "Closed"),
					dateField("AwardDate", "Award Date"),
					accountLookup("Awards"),
					contactLookup("Awards"),
					lookupField("DesignationId", "Designation", "Designation", "Awards", false),
					descriptionField(),
				},
			},
			{
				APIName: "Disbursement", Label: "Disbursement", PluralLabel: "Disbursements",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					currencyField("Amount", "Amount"),
					dateField("DisbursementDate", "Disbursement Date"),
					statusField("Scheduled", "Paid", "Cancelled"),
					lookupField("AwardId", "Award", "Award", "Disbursements", false),
					lookupField("DonorCommitmentId", "Donor Commitment", "DonorCommitment", "Disbursements", false),
					accountLookup("Disbursements"),
					descriptionField(),
				},
			},
			{
				APIName: "BenefitRecipient", Label: "Benefit Recipient", PluralLabel: "Benefit Recipients",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					statusField(),
					accountLookup("BenefitRecipients"),
					contactLookup("BenefitRecipients"),
					lookupField("AwardId", "Award", "Award", "BenefitRecipients", false),
					descriptionField(),
				},
			},
			{
				APIName: "DeliveryFramework", Label: "Delivery Framework", PluralLabel: "Delivery Frameworks",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					statusField("Planned", "Active", "Completed"),
					dateField("StartDate", "Start Date"),
					dateField("EndDate", "End Date"),
					accountLookup("DeliveryFrameworks"),
					descriptionField(),
				},
			},
			{
				APIName: "Indicator", Label: "Indicator", PluralLabel: "Indicators",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					textField("Code", "Code", 50, true),
					numberField("TargetValue", "Target Value"),
					numberField("ActualValue", "Actual Value"),
					lookupField("DeliveryFrameworkId", "Delivery Framework", "DeliveryFramework", "Indicators", false),
					statusField(),
					descriptionField(),
				},
			},
			{
				APIName: "Budget", Label: "Budget", PluralLabel: "Budgets",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					currencyField("Amount", "Amount"),
					dateField("FiscalYearStart", "Fiscal Year Start"),
					dateField("FiscalYearEnd", "Fiscal Year End"),
					lookupField("DesignationId", "Designation", "Designation", "Budgets", false),
					statusField("Draft", "Approved", "Closed"),
					descriptionField(),
				},
			},
		},
	})
}
