package dataengine

// QueryLimits mirrors packages/shared QUERY_LIMITS.
type QueryLimits struct {
	MaxFieldsPerObject    int
	MaxFilterConditions   int
	MaxJoins              int
	MaxChildRelationships int
	MaxChildRowsPerParent int
	MaxChildRowsPerQuery  int
	MaxSortFields         int
	MaxInListSize         int
	StandardMaxRows       int
	DefaultPageSize       int
	LocatorMaxRows        int
	LocatorPageSize       int
	MaxSelectFields       int
}

// Limits is the platform default query ceiling set.
var Limits = QueryLimits{
	MaxFieldsPerObject:    2000,
	MaxFilterConditions:   200,
	MaxJoins:              30,
	MaxChildRelationships: 20,
	MaxChildRowsPerParent: 2000,
	MaxChildRowsPerQuery:  100_000,
	MaxSortFields:         20,
	MaxInListSize:         10_000,
	StandardMaxRows:       10_000,
	DefaultPageSize:       50,
	LocatorMaxRows:        50_000_000,
	LocatorPageSize:       2000,
	MaxSelectFields:       1000,
}

// HighVolumeLocatorMaxRows is the product ceiling for HV locator sessions (ADR-013).
// Prefer time-bounded locators over raising this toward LocatorMaxRows.
const HighVolumeLocatorMaxRows = 5_000_000
