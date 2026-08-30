package seed

import (
	"github.com/MajestaNet/ide/internal/packages"
)

const AddressPackageVersion = "1.1.0"

func registerAddressModule() {
	packages.Register(packages.Module{
		Name:              "address",
		Version:           AddressPackageVersion,
		Label:             "Address",
		Description:       "Multi-address rows for Account and Contact",
		DependsOn:         []string{"core"},
		Optional:          true,
		DocumentationPath: "docs/modules/address.md",
		Objects: []packages.ObjectDef{
			{
				APIName: "Address", Label: "Address", PluralLabel: "Addresses",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					{APIName: "Name", Label: "Name", FieldType: "text", Required: true, Length: intPtr(255), Filterable: true, Sortable: true, Indexed: true, Searchable: true},
					{APIName: "Street", Label: "Street", FieldType: "text", Length: intPtr(255), Filterable: true, Sortable: true},
					{APIName: "City", Label: "City", FieldType: "text", Length: intPtr(100), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "State", Label: "State/Province", FieldType: "text", Length: intPtr(100), Filterable: true, Sortable: true},
					{APIName: "PostalCode", Label: "Postal Code", FieldType: "text", Length: intPtr(50), Filterable: true, Sortable: true, Indexed: true, Searchable: true},
					{APIName: "Country", Label: "Country", FieldType: "text", Length: intPtr(100), Filterable: true, Sortable: true},
					{APIName: "AddressType", Label: "Address Type", FieldType: "picklist", PicklistValues: []string{"Billing", "Shipping", "Mailing", "Other"}, Filterable: true, Sortable: true, Indexed: true},
					{APIName: "IsPrimary", Label: "Primary", FieldType: "boolean", Filterable: true, Sortable: true},
					{APIName: "AccountId", Label: "Account", FieldType: "lookup", ReferenceTo: strPtr("Account"), RelationshipName: strPtr("Addresses"), Filterable: true, Sortable: true, Indexed: true},
					{APIName: "ContactId", Label: "Contact", FieldType: "lookup", ReferenceTo: strPtr("Contact"), RelationshipName: strPtr("Addresses"), Filterable: true, Sortable: true, Indexed: true},
				},
			},
		},
	})
}
