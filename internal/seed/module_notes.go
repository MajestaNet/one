package seed

import (
	"github.com/MajestaNet/ide/internal/packages"
)

const NotesPackageVersion = "1.1.0"

func registerNotesModule() {
	packages.Register(packages.Module{
		Name:              "notes",
		Version:           NotesPackageVersion,
		Label:             "Notes",
		Description:       "Optional Note object with optional Account lookup",
		DependsOn:         []string{"core"},
		Optional:          true,
		DocumentationPath: "docs/modules/notes.md",
		Objects: []packages.ObjectDef{
			{
				APIName: "Note", Label: "Note", PluralLabel: "Notes",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					{APIName: "Title", Label: "Title", FieldType: "text", Required: true, Length: intPtr(255), Filterable: true, Sortable: true, Indexed: true, Searchable: true},
					{APIName: "Body", Label: "Body", FieldType: "textarea", Length: intPtr(32000), Filterable: false, Sortable: false},
					{APIName: "AccountId", Label: "Account", FieldType: "lookup", ReferenceTo: strPtr("Account"), RelationshipName: strPtr("Notes"), Filterable: true, Sortable: true, Indexed: true},
				},
			},
		},
	})
}
