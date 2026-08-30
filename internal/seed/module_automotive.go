package seed

import "github.com/MajestaNet/ide/internal/packages"

const AutomotivePackageVersion = "1.0.0"

func registerAutomotiveModule() {
	packages.Register(packages.Module{
		Name:              "automotive",
		Version:           AutomotivePackageVersion,
		Label:             "Automotive",
		Description:       "Automotive: devices, deals, facilities, inspections",
		DependsOn:         []string{"core"},
		Optional:          true,
		DocumentationPath: "docs/modules/automotive.md",
		Objects: []packages.ObjectDef{
			{
				APIName: "DeviceBrand", Label: "Device Brand", PluralLabel: "Device Brands",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					statusField(),
					descriptionField(),
				},
			},
			{
				APIName: "DeviceModel", Label: "Device Model", PluralLabel: "Device Models",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					textField("ModelCode", "Model Code", 50, true),
					lookupField("DeviceBrandId", "Device Brand", "DeviceBrand", "DeviceModels", true),
					numberField("ModelYear", "Model Year"),
					statusField(),
					descriptionField(),
				},
			},
			{
				APIName: "Device", Label: "Device", PluralLabel: "Devices",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					textField("VIN", "VIN", 50, true),
					lookupField("DeviceModelId", "Device Model", "DeviceModel", "Devices", false),
					lookupField("DeviceBrandId", "Device Brand", "DeviceBrand", "Devices", false),
					statusField("Available", "Sold", "In Service", "Scrapped"),
					accountLookup("Devices"),
					contactLookup("Devices"),
					descriptionField(),
				},
			},
			{
				APIName: "BusinessFacility", Label: "Business Facility", PluralLabel: "Business Facilities",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					picklistField("FacilityType", "Facility Type", []string{"Dealership", "Service Center", "Warehouse", "Other"}),
					textField("City", "City", 100, true),
					accountLookup("BusinessFacilities"),
					statusField(),
					descriptionField(),
				},
			},
			{
				APIName: "Deal", Label: "Deal", PluralLabel: "Deals",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					statusField("Draft", "Quoted", "Accepted", "Delivered", "Cancelled"),
					currencyField("Amount", "Amount"),
					dateField("CloseDate", "Close Date"),
					accountLookup("Deals"),
					contactLookup("Deals"),
					lookupField("BusinessFacilityId", "Business Facility", "BusinessFacility", "Deals", false),
					descriptionField(),
				},
			},
			{
				APIName: "DealCustomer", Label: "Deal Customer", PluralLabel: "Deal Customers",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					lookupField("DealId", "Deal", "Deal", "DealCustomers", true),
					accountLookup("DealCustomers"),
					contactLookup("DealCustomers"),
					picklistField("Role", "Role", []string{"Buyer", "Co-Buyer", "Guarantor", "Other"}),
					boolField("IsPrimary", "Primary"),
				},
			},
			{
				APIName: "DealDevice", Label: "Deal Device", PluralLabel: "Deal Devices",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					lookupField("DealId", "Deal", "Deal", "DealDevices", true),
					lookupField("DeviceId", "Device", "Device", "DealDevices", true),
					currencyField("Price", "Price"),
				},
			},
			{
				APIName: "DeviceInspection", Label: "Device Inspection", PluralLabel: "Device Inspections",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					lookupField("DeviceId", "Device", "Device", "DeviceInspections", true),
					dateField("InspectionDate", "Inspection Date"),
					statusField("Passed", "Failed", "Conditional"),
					descriptionField(),
				},
			},
		},
	})
}
