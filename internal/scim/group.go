package scim

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/MajestaNet/ide/internal/db"
)

// Group is a SCIM Group resource backed by directory_tags.
type Group struct {
	Schemas     []string      `json:"schemas"`
	ID          string        `json:"id,omitempty"`
	ExternalID  string        `json:"externalId,omitempty"`
	DisplayName string        `json:"displayName,omitempty"`
	Members     []GroupMember `json:"members,omitempty"`
	Meta        *Meta         `json:"meta,omitempty"`
}

// GroupMember is a SCIM Group members[] entry (User only).
type GroupMember struct {
	Value   string `json:"value,omitempty"`
	Display string `json:"display,omitempty"`
	Type    string `json:"type,omitempty"`
	Ref     string `json:"$ref,omitempty"`
}

// GroupListResponse is a SCIM ListResponse of Groups.
type GroupListResponse struct {
	Schemas      []string `json:"schemas"`
	TotalResults int      `json:"totalResults"`
	StartIndex   int      `json:"startIndex"`
	ItemsPerPage int      `json:"itemsPerPage"`
	Resources    []Group  `json:"Resources"`
}

const groupMemberTypeUser = "User"

// ToGroup maps a directory tag and members to a SCIM Group.
func ToGroup(tag *db.DirectoryTag, members []db.DirectoryTagMember, locationBase string) Group {
	locBase := strings.TrimRight(locationBase, "/")
	out := Group{
		Schemas:     []string{SchemaCoreGroup},
		ID:          tag.ID,
		DisplayName: tag.DisplayName,
		Meta: &Meta{
			ResourceType: "Group",
			Created:      tag.CreatedAt.UTC().Format(time.RFC3339),
			LastModified: tag.UpdatedAt.UTC().Format(time.RFC3339),
			Location:     locBase + "/Groups/" + tag.ID,
		},
	}
	if tag.ExternalID != nil {
		out.ExternalID = *tag.ExternalID
	}
	if len(members) > 0 {
		out.Members = make([]GroupMember, 0, len(members))
		for _, m := range members {
			out.Members = append(out.Members, GroupMember{
				Value:   m.UserID,
				Display: m.DisplayName,
				Type:    groupMemberTypeUser,
				Ref:     locBase + "/Users/" + m.UserID,
			})
		}
	}
	return out
}

// MemberUserIDs returns unique member values, rejecting nested Group types.
func (g Group) MemberUserIDs() ([]string, error) {
	var ids []string
	seen := map[string]struct{}{}
	for _, m := range g.Members {
		typ := strings.TrimSpace(m.Type)
		if typ != "" && !strings.EqualFold(typ, groupMemberTypeUser) {
			return nil, fmt.Errorf("member type %q is not supported", m.Type)
		}
		id := strings.TrimSpace(m.Value)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, nil
}

// ApplyGroupPatch applies SCIM patch operations onto a Group resource (in memory).
func ApplyGroupPatch(g *Group, ops []PatchOperation) error {
	if g == nil {
		return fmt.Errorf("nil group")
	}
	for i, op := range ops {
		opName := strings.ToLower(strings.TrimSpace(op.Op))
		path := strings.TrimSpace(op.Path)
		switch opName {
		case "replace", "add":
			if err := applyGroupReplaceAdd(g, opName, path, op.Value); err != nil {
				return fmt.Errorf("operations[%d]: %w", i, err)
			}
		case "remove":
			if err := applyGroupRemove(g, path, op.Value); err != nil {
				return fmt.Errorf("operations[%d]: %w", i, err)
			}
		default:
			return fmt.Errorf("operations[%d]: unsupported op %q", i, op.Op)
		}
	}
	return nil
}

func applyGroupReplaceAdd(g *Group, op, path string, raw json.RawMessage) error {
	if path == "" {
		var overlay Group
		if err := json.Unmarshal(raw, &overlay); err != nil {
			return err
		}
		if overlay.DisplayName != "" {
			g.DisplayName = overlay.DisplayName
		}
		if overlay.ExternalID != "" {
			g.ExternalID = overlay.ExternalID
		}
		if overlay.Members != nil {
			if op == "add" {
				g.Members = appendMembers(g.Members, overlay.Members)
			} else {
				g.Members = overlay.Members
			}
		}
		return nil
	}
	norm := normalizePath(path)
	switch norm {
	case "displayname":
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return err
		}
		g.DisplayName = s
	case "externalid":
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return err
		}
		g.ExternalID = s
	case "members":
		members, err := parseMembersValue(raw)
		if err != nil {
			return err
		}
		if op == "add" {
			g.Members = appendMembers(g.Members, members)
		} else {
			g.Members = members
		}
	default:
		return fmt.Errorf("unsupported path %q", path)
	}
	return nil
}

func applyGroupRemove(g *Group, path string, raw json.RawMessage) error {
	norm := normalizePath(path)
	switch norm {
	case "externalid":
		g.ExternalID = ""
	case "members":
		if len(raw) == 0 || string(raw) == "null" {
			g.Members = nil
			return nil
		}
		toRemove, err := parseMembersValue(raw)
		if err != nil {
			return err
		}
		g.Members = removeMembers(g.Members, toRemove)
	default:
		if value, ok := memberValueFromPath(path); ok {
			g.Members = removeMembers(g.Members, []GroupMember{{Value: value}})
			return nil
		}
		return fmt.Errorf("unsupported remove path %q", path)
	}
	return nil
}

func parseMembersValue(raw json.RawMessage) ([]GroupMember, error) {
	var arr []GroupMember
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr, nil
	}
	var one GroupMember
	if err := json.Unmarshal(raw, &one); err != nil {
		return nil, err
	}
	return []GroupMember{one}, nil
}

func appendMembers(dst, add []GroupMember) []GroupMember {
	seen := map[string]struct{}{}
	out := make([]GroupMember, 0, len(dst)+len(add))
	for _, m := range dst {
		id := strings.TrimSpace(m.Value)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, m)
	}
	for _, m := range add {
		id := strings.TrimSpace(m.Value)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, m)
	}
	return out
}

func removeMembers(dst, drop []GroupMember) []GroupMember {
	remove := map[string]struct{}{}
	for _, m := range drop {
		id := strings.TrimSpace(m.Value)
		if id != "" {
			remove[id] = struct{}{}
		}
	}
	out := make([]GroupMember, 0, len(dst))
	for _, m := range dst {
		if _, ok := remove[strings.TrimSpace(m.Value)]; ok {
			continue
		}
		out = append(out, m)
	}
	return out
}

func memberValueFromPath(path string) (string, bool) {
	norm := normalizePath(path)
	const prefix = `members[valueeq"`
	if strings.HasPrefix(norm, prefix) && strings.HasSuffix(norm, `"]`) {
		return strings.TrimSuffix(strings.TrimPrefix(norm, prefix), `"]`), true
	}
	const prefixUnquoted = `members[valueeq`
	if strings.HasPrefix(norm, prefixUnquoted) && strings.HasSuffix(norm, "]") && !strings.HasPrefix(norm, prefix) {
		inner := strings.TrimSuffix(strings.TrimPrefix(norm, prefixUnquoted), "]")
		inner = strings.Trim(inner, `"'`)
		if inner != "" {
			return inner, true
		}
	}
	return "", false
}

// PatchTouchesGroups reports whether a User PatchOp attempts to write groups.
func PatchTouchesGroups(ops []PatchOperation) bool {
	for _, op := range ops {
		path := strings.TrimSpace(op.Path)
		if path == "" {
			var overlay map[string]any
			if json.Unmarshal(op.Value, &overlay) == nil {
				if _, ok := overlay["groups"]; ok {
					return true
				}
			}
			continue
		}
		norm := normalizePath(path)
		if norm == "groups" || strings.HasPrefix(norm, "groups[") || strings.HasPrefix(norm, "groups.") {
			return true
		}
	}
	return false
}
