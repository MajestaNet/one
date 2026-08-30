package seed

import "github.com/MajestaNet/ide/internal/packages"

const FinancialServicesPackageVersion = "1.0.0"

func registerFinancialServicesModule() {
	packages.Register(packages.Module{
		Name:              "financial_services",
		Version:           FinancialServicesPackageVersion,
		Label:             "Financial Services",
		Description:       "Banking and insurance: Bank, Branch, FinancialProduct, Claim, Coverage, KYC",
		DependsOn:         []string{"core"},
		Optional:          true,
		DocumentationPath: "docs/modules/financial-services.md",
		Objects: []packages.ObjectDef{
			{
				APIName: "Bank", Label: "Bank", PluralLabel: "Banks",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					textField("SwiftCode", "SWIFT Code", 50, true),
					textField("Country", "Country", 100, true),
					statusField(),
					descriptionField(),
				},
			},
			{
				APIName: "Branch", Label: "Branch", PluralLabel: "Branches",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					textField("BranchCode", "Branch Code", 50, true),
					lookupField("BankId", "Bank", "Bank", "Branches", true),
					textField("City", "City", 100, true),
					statusField(),
					descriptionField(),
				},
			},
			{
				APIName: "FinancialProduct", Label: "Financial Product", PluralLabel: "Financial Products",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					textField("ProductCode", "Product Code", 100, true),
					picklistField("ProductType", "Product Type", []string{"Checking", "Savings", "Loan", "Mortgage", "Credit Card", "Investment", "Insurance", "Other"}),
					statusField(),
					accountLookup("FinancialProducts"),
					contactLookup("FinancialProducts"),
					currencyField("LimitAmount", "Limit Amount"),
					descriptionField(),
				},
			},
			{
				APIName: "Collateral", Label: "Collateral", PluralLabel: "Collateral",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					picklistField("CollateralType", "Type", []string{"Real Estate", "Vehicle", "Securities", "Cash", "Other"}),
					currencyField("Value", "Value"),
					lookupField("FinancialProductId", "Financial Product", "FinancialProduct", "Collateral", false),
					accountLookup("Collateral"),
					descriptionField(),
				},
			},
			{
				APIName: "Claim", Label: "Claim", PluralLabel: "Claims",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					textField("ClaimNumber", "Claim Number", 100, true),
					statusField("Open", "Under Review", "Approved", "Denied", "Closed"),
					dateField("LossDate", "Loss Date"),
					currencyField("ClaimedAmount", "Claimed Amount"),
					currencyField("PaidAmount", "Paid Amount"),
					accountLookup("Claims"),
					contactLookup("Claims"),
					descriptionField(),
				},
			},
			{
				APIName: "Coverage", Label: "Coverage", PluralLabel: "Coverages",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					textField("CoverageCode", "Coverage Code", 100, true),
					currencyField("LimitAmount", "Limit Amount"),
					currencyField("Deductible", "Deductible"),
					lookupField("FinancialProductId", "Financial Product", "FinancialProduct", "Coverages", false),
					statusField(),
					descriptionField(),
				},
			},
			{
				APIName: "Limit", Label: "Limit", PluralLabel: "Limits",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					currencyField("Amount", "Amount"),
					picklistField("LimitType", "Limit Type", []string{"Credit", "Exposure", "Policy", "Other"}),
					lookupField("FinancialProductId", "Financial Product", "FinancialProduct", "Limits", false),
					accountLookup("Limits"),
					statusField(),
				},
			},
			{
				APIName: "MortgageApplication", Label: "Mortgage Application", PluralLabel: "Mortgage Applications",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					statusField("Draft", "Submitted", "Underwriting", "Approved", "Rejected", "Funded"),
					currencyField("RequestedAmount", "Requested Amount"),
					numberField("TermMonths", "Term Months"),
					accountLookup("MortgageApplications"),
					contactLookup("MortgageApplications"),
					lookupField("BranchId", "Branch", "Branch", "MortgageApplications", false),
					descriptionField(),
				},
			},
			{
				APIName: "KYC", Label: "KYC", PluralLabel: "KYC Records",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					statusField("Pending", "In Review", "Approved", "Rejected", "Expired"),
					dateField("ReviewDate", "Review Date"),
					dateField("ExpiryDate", "Expiry Date"),
					picklistField("RiskRating", "Risk Rating", []string{"Low", "Medium", "High"}),
					accountLookup("KYCRecords"),
					contactLookup("KYCRecords"),
					descriptionField(),
				},
			},
		},
	})
}
