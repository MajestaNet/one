package dataengine

import (
	"fmt"
	"strings"

	"github.com/MajestaNet/ide/internal/authz"
)

// Visibility modes for QueryVisibility.Mode.
const (
	// VisibilityNone means no SQL visibility predicate (view-all / admin).
	VisibilityNone = ""
	// VisibilityOwnerCreator is owner_id OR created_by_id (ADR-009 baseline when sharing is off).
	VisibilityOwnerCreator = "owner_creator"
	// VisibilityLegacy is a deprecated alias for VisibilityOwnerCreator.
	VisibilityLegacy = VisibilityOwnerCreator
	// VisibilitySharing is full ADR-016 record-sharing predicate.
	VisibilitySharing = "sharing"
)

// QueryVisibility optional record visibility SQL predicate.
type QueryVisibility struct {
	// Mode is VisibilityNone, VisibilityLegacy, or VisibilitySharing.
	Mode                   string
	UserID                 string
	DefaultAccess          string
	HasObjectRead          bool
	SubordinateDataRoleIDs []string
}

// Enabled reports whether a SQL visibility clause should be applied.
func (v QueryVisibility) Enabled() bool {
	switch v.Mode {
	case VisibilityOwnerCreator, VisibilitySharing, "legacy":
		return true
	default:
		return false
	}
}

// AppendSharingVisibility adds a record visibility OR-clause when Mode is set.
// alias is the SQL table/alias qualifier (e.g. "r" or "c").
func AppendSharingVisibility(alias string, where *[]string, args *[]any, vis QueryVisibility) {
	if !vis.Enabled() || vis.UserID == "" {
		return
	}
	if alias == "" {
		alias = "r"
	}
	*args = append(*args, vis.UserID)
	userParam := fmt.Sprintf("$%d::uuid", len(*args))

	if vis.Mode == VisibilityOwnerCreator || vis.Mode == VisibilityLegacy || vis.Mode == "legacy" {
		*where = append(*where, fmt.Sprintf("(%s.owner_id = %s OR %s.created_by_id = %s)", alias, userParam, alias, userParam))
		return
	}

	parts := []string{}
	parts = append(parts, fmt.Sprintf("%s.owner_id = %s", alias, userParam))
	parts = append(parts, fmt.Sprintf("%s.created_by_id = %s", alias, userParam))
	*args = append(*args, vis.UserID)
	grantParam := fmt.Sprintf("$%d::uuid", len(*args))
	parts = append(parts, fmt.Sprintf(`EXISTS (
  SELECT 1 FROM record_access_grants g
  WHERE g.object_api_name = %s.object_api_name AND g.record_id = %s.id AND g.user_id = %s
)`, alias, alias, grantParam))
	if vis.HasObjectRead && (vis.DefaultAccess == authz.DefaultAccessPublicRead || vis.DefaultAccess == authz.DefaultAccessPublicReadWrite) {
		parts = append(parts, "TRUE")
	}
	if len(vis.SubordinateDataRoleIDs) > 0 {
		*args = append(*args, vis.SubordinateDataRoleIDs)
		rolesParam := fmt.Sprintf("$%d::uuid[]", len(*args))
		parts = append(parts, fmt.Sprintf(`EXISTS (
  SELECT 1 FROM users ou
  WHERE ou.id = %s.owner_id AND ou.data_role_id = ANY(%s)
)`, alias, rolesParam))
	}
	*where = append(*where, "("+strings.Join(parts, " OR ")+")")
}
