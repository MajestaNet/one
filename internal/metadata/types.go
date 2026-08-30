package metadata

import "encoding/json"

// QueryLimits mirrors packages/shared QUERY_LIMITS (describe surface).
type QueryLimits struct {
	MaxFieldsPerObject int `json:"maxFieldsPerObject"`
	MaxJoins           int `json:"maxJoins"`
	StandardMaxRows    int `json:"standardMaxRows"`
	LocatorMaxRows     int `json:"locatorMaxRows"`
}

// DefaultQueryLimits is the describe limits block.
var DefaultQueryLimits = QueryLimits{
	MaxFieldsPerObject: 2000,
	MaxJoins:           10,
	StandardMaxRows:    10_000,
	LocatorMaxRows:     50_000_000,
}

// ObjectDefinition is a metadata_objects row (API shape).
type ObjectDefinition struct {
	ID          string          `json:"id"`
	APIName     string          `json:"apiName"`
	Label       string          `json:"label"`
	PluralLabel string          `json:"pluralLabel"`
	StorageMode string          `json:"storageMode"`
	PackageName *string         `json:"packageName"`
	Ownership   string          `json:"ownership"`
	Features    map[string]bool `json:"features"`
}

// FieldDefinition is a metadata_fields row (API shape).
type FieldDefinition struct {
	ID                   string          `json:"id"`
	ObjectAPIName        string          `json:"objectApiName"`
	APIName              string          `json:"apiName"`
	Label                string          `json:"label"`
	FieldType            string          `json:"fieldType"`
	Required             bool            `json:"required"`
	UniqueField          bool            `json:"uniqueField"`
	ExternalID           bool            `json:"externalId"`
	Indexed              bool            `json:"indexed"`
	Filterable           bool            `json:"filterable"`
	Sortable             bool            `json:"sortable"`
	Searchable           bool            `json:"searchable"`
	DefaultValue         json.RawMessage `json:"defaultValue"`
	Length               *int            `json:"length"`
	Precision            *int            `json:"precision"`
	Scale                *int            `json:"scale"`
	PicklistValues       []string        `json:"picklistValues"`
	ReferenceTo          *string         `json:"referenceTo"`
	RelationshipName     *string         `json:"relationshipName"`
	PolymorphicTypeField *string         `json:"polymorphicTypeField,omitempty"`
	AutonumberFormat     *string         `json:"autonumberFormat,omitempty"`
	AutonumberStart      *int            `json:"autonumberStart,omitempty"`
	PackageName          *string         `json:"packageName"`
	Ownership            string          `json:"ownership"`
	KernelColumn         *string         `json:"kernelColumn,omitempty"`
}

// ValidationRuleDefinition is exposed on describe.
type ValidationRuleDefinition struct {
	APIName      string          `json:"apiName"`
	Label        string          `json:"label"`
	Active       bool            `json:"active"`
	ErrorMessage string          `json:"errorMessage"`
	Expression   json.RawMessage `json:"expression"`
	PackageName  *string         `json:"packageName"`
	Ownership    string          `json:"ownership"`
}

// DescribeObject is ObjectDefinition + fields + rules + limits.
type DescribeObject struct {
	ObjectDefinition
	Fields          []FieldDefinition          `json:"fields"`
	ValidationRules []ValidationRuleDefinition `json:"validationRules"`
	Limits          QueryLimits                `json:"limits"`
}

// GlobalDescribe is GET /client/v1/describe.
type GlobalDescribe struct {
	Encoding     string             `json:"encoding"`
	MaxBatchSize int                `json:"maxBatchSize"`
	Limits       QueryLimits        `json:"limits"`
	SObjects     []GlobalSObjectRef `json:"sobjects"`
}

// GlobalSObjectRef is one entry in describeGlobal.sobjects.
type GlobalSObjectRef struct {
	Name        string  `json:"name"`
	Label       string  `json:"label"`
	LabelPlural string  `json:"labelPlural"`
	Custom      bool    `json:"custom"`
	Ownership   string  `json:"ownership"`
	PackageName *string `json:"packageName"`
	StorageMode string  `json:"storageMode"`
}
