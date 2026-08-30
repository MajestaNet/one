package seed

import (
	"github.com/MajestaNet/ide/internal/packages"
)

const CrmBridgePackageVersion = "1.0.0"

func registerCrmBridgeModule() {
	packages.Register(packages.Module{
		Name:              "crm_bridge",
		Version:           CrmBridgePackageVersion,
		Label:             "CRM Bridge",
		Description:       "Cross-cloud lookup fields between Sales and Service (Case.OpportunityId); auto-enabled when both are on",
		DependsOn:         []string{"sales", "service"},
		Optional:          true,
		AutoEnable:        true,
		DocumentationPath: "docs/modules/crm-bridge.md",
		// No Objects — only FieldExtensions on Case (owned by service).
		FieldExtensions: []packages.FieldExtension{
			{
				ObjectAPIName: "Case",
				Fields: []packages.FieldDef{
					{APIName: "OpportunityId", Label: "Opportunity", FieldType: "lookup", ReferenceTo: strPtr("Opportunity"), RelationshipName: strPtr("Cases"), Filterable: true, Sortable: true, Indexed: true},
				},
			},
		},
	})
}
