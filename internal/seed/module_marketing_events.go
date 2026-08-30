package seed

import "github.com/MajestaNet/ide/internal/packages"

const MarketingEventsPackageVersion = "1.0.0"

func registerMarketingEventsModule() {
	packages.Register(packages.Module{
		Name:              "marketing_events",
		Version:           MarketingEventsPackageVersion,
		Label:             "Marketing Events",
		Description:       "Marketing events and journeys (Campaign remains in lead_marketing)",
		DependsOn:         []string{"core"},
		Optional:          true,
		DocumentationPath: "docs/modules/marketing-events.md",
		Objects: []packages.ObjectDef{
			{
				APIName: "MarketingEvent", Label: "Marketing Event", PluralLabel: "Marketing Events",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					statusField("Draft", "Published", "In Progress", "Completed", "Cancelled"),
					picklistField("EventType", "Event Type", []string{"Conference", "Webinar", "Workshop", "Trade Show", "Other"}),
					dateField("StartDate", "Start Date"),
					dateField("EndDate", "End Date"),
					textField("Location", "Location", 255, false),
					descriptionField(),
				},
			},
			{
				APIName: "Building", Label: "Building", PluralLabel: "Buildings",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					textField("Address", "Address", 255, false),
					textField("City", "City", 100, true),
					statusField(),
					descriptionField(),
				},
			},
			{
				APIName: "Hotel", Label: "Hotel", PluralLabel: "Hotels",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					textField("City", "City", 100, true),
					numberField("StarRating", "Star Rating"),
					statusField(),
					descriptionField(),
				},
			},
			{
				APIName: "EventVendor", Label: "Event Vendor", PluralLabel: "Event Vendors",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					lookupField("MarketingEventId", "Marketing Event", "MarketingEvent", "EventVendors", true),
					accountLookup("EventVendors"),
					picklistField("VendorRole", "Vendor Role", []string{"Catering", "AV", "Venue", "Other"}),
					statusField(),
					descriptionField(),
				},
			},
			{
				APIName: "AttendeePass", Label: "Attendee Pass", PluralLabel: "Attendee Passes",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					lookupField("MarketingEventId", "Marketing Event", "MarketingEvent", "AttendeePasses", true),
					currencyField("Price", "Price"),
					statusField("Available", "Sold Out", "Retired"),
					descriptionField(),
				},
			},
			{
				APIName: "EventRegistration", Label: "Event Registration", PluralLabel: "Event Registrations",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					lookupField("MarketingEventId", "Marketing Event", "MarketingEvent", "EventRegistrations", true),
					lookupField("AttendeePassId", "Attendee Pass", "AttendeePass", "EventRegistrations", false),
					contactLookup("EventRegistrations"),
					accountLookup("EventRegistrations"),
					statusField("Registered", "Checked In", "Cancelled", "No Show"),
					dateField("RegistrationDate", "Registration Date"),
				},
			},
			{
				APIName: "CustomerJourney", Label: "Customer Journey", PluralLabel: "Customer Journeys",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					statusField("Draft", "Live", "Stopped", "Completed"),
					dateField("StartDate", "Start Date"),
					dateField("EndDate", "End Date"),
					descriptionField(),
				},
			},
		},
	})
}
