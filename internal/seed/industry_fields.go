package seed

import "github.com/MajestaNet/ide/internal/packages"

// Shared field helpers for curated industry managed packs.

func nameRequiredField() packages.FieldDef {
	return packages.FieldDef{APIName: "Name", Label: "Name", FieldType: "text", Required: true, Length: intPtr(255), Filterable: true, Sortable: true, Indexed: true}
}

func descriptionField() packages.FieldDef {
	return packages.FieldDef{APIName: "Description", Label: "Description", FieldType: "textarea", Length: intPtr(32000), Filterable: false, Sortable: false}
}

func statusField(values ...string) packages.FieldDef {
	if len(values) == 0 {
		values = []string{"Active", "Inactive"}
	}
	return packages.FieldDef{APIName: "Status", Label: "Status", FieldType: "picklist", PicklistValues: values, Filterable: true, Sortable: true, Indexed: true}
}

func accountLookup(rel string) packages.FieldDef {
	return packages.FieldDef{APIName: "AccountId", Label: "Account", FieldType: "lookup", ReferenceTo: strPtr("Account"), RelationshipName: strPtr(rel), Filterable: true, Sortable: true, Indexed: true}
}

func contactLookup(rel string) packages.FieldDef {
	return packages.FieldDef{APIName: "ContactId", Label: "Contact", FieldType: "lookup", ReferenceTo: strPtr("Contact"), RelationshipName: strPtr(rel), Filterable: true, Sortable: true, Indexed: true}
}

func textField(api, label string, length int, indexed bool) packages.FieldDef {
	f := packages.FieldDef{APIName: api, Label: label, FieldType: "text", Length: intPtr(length), Filterable: true, Sortable: true}
	if indexed {
		f.Indexed = true
	}
	return f
}

func dateField(api, label string) packages.FieldDef {
	return packages.FieldDef{APIName: api, Label: label, FieldType: "date", Filterable: true, Sortable: true}
}

func numberField(api, label string) packages.FieldDef {
	return packages.FieldDef{APIName: api, Label: label, FieldType: "number", Filterable: true, Sortable: true}
}

func currencyField(api, label string) packages.FieldDef {
	return packages.FieldDef{APIName: api, Label: label, FieldType: "currency", Filterable: true, Sortable: true}
}

func boolField(api, label string) packages.FieldDef {
	return packages.FieldDef{APIName: api, Label: label, FieldType: "boolean", Filterable: true, Sortable: true}
}

func lookupField(api, label, ref, rel string, required bool) packages.FieldDef {
	return packages.FieldDef{
		APIName: api, Label: label, FieldType: "lookup", Required: required,
		ReferenceTo: strPtr(ref), RelationshipName: strPtr(rel),
		Filterable: true, Sortable: true, Indexed: true,
	}
}

func picklistField(api, label string, values []string) packages.FieldDef {
	return packages.FieldDef{APIName: api, Label: label, FieldType: "picklist", PicklistValues: values, Filterable: true, Sortable: true}
}

func streetAddressFields(prefix, labelPrefix string) []packages.FieldDef {
	return []packages.FieldDef{
		textField(prefix+"Street", labelPrefix+" Street", 255, false),
		textField(prefix+"City", labelPrefix+" City", 100, true),
		textField(prefix+"State", labelPrefix+" State/Province", 100, false),
		textField(prefix+"PostalCode", labelPrefix+" Postal Code", 50, false),
		textField(prefix+"Country", labelPrefix+" Country", 100, false),
	}
}
