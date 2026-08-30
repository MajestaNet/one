package scim

import "testing"

func TestParseFilter(t *testing.T) {
	f, err := ParseFilter(`userName eq "jdoe" and active eq true`)
	if err != nil {
		t.Fatal(err)
	}
	if f.UserName != "jdoe" || f.Active == nil || !*f.Active {
		t.Fatalf("unexpected filter %+v", f)
	}
	f, err = ParseFilter(`emails.value eq "a@b.com" and urn:ietf:params:scim:schemas:extension:one:2.0:Principal:principalType eq "service"`)
	if err != nil {
		t.Fatal(err)
	}
	if f.Email != "a@b.com" || f.PrincipalType != "service" {
		t.Fatalf("unexpected filter %+v", f)
	}
}

func TestApplyPatchActive(t *testing.T) {
	active := true
	u := &User{Active: &active, UserName: "x"}
	ops := []PatchOperation{{
		Op:    "replace",
		Path:  "active",
		Value: []byte("false"),
	}}
	if err := ApplyPatch(u, ops); err != nil {
		t.Fatal(err)
	}
	if u.Active == nil || *u.Active {
		t.Fatal("expected active=false")
	}
}

func TestApplyPatchUserCustomPreservesFieldCase(t *testing.T) {
	u := &User{UserName: "x"}
	ops := []PatchOperation{{
		Op:    "add",
		Path:  SchemaUserCustom + ":CostCenter__c",
		Value: []byte(`"CC-1"`),
	}}
	if err := ApplyPatch(u, ops); err != nil {
		t.Fatal(err)
	}
	if u.Custom["CostCenter__c"] != "CC-1" {
		t.Fatalf("custom=%v", u.Custom)
	}
}

func TestToCreateInputSplitsEmployeeNumber(t *testing.T) {
	in, err := ToCreateInput(User{
		UserName:   "jdoe",
		ExternalID: "fed-1",
		Emails:     []Email{{Value: "j@example.com", Primary: true}},
		Enterprise: &EnterpriseUser{EmployeeNumber: "E-9", Department: "Sales"},
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if in.ExternalID != "fed-1" {
		t.Fatalf("externalId=%q", in.ExternalID)
	}
	if in.EmployeeNumber != "E-9" {
		t.Fatalf("employeeNumber=%q", in.EmployeeNumber)
	}
	if in.Department != "Sales" {
		t.Fatalf("department=%q", in.Department)
	}
	if len(in.RoleAPINames) != 1 || in.RoleAPINames[0] != "StandardUser" {
		t.Fatalf("roles=%v", in.RoleAPINames)
	}
}

func TestParseGroupFilter(t *testing.T) {
	f, err := ParseGroupFilter(`displayName eq "Sales" and externalId eq "okta-grp-01"`)
	if err != nil {
		t.Fatal(err)
	}
	if f.DisplayName != "Sales" || f.ExternalID != "okta-grp-01" {
		t.Fatalf("unexpected %+v", f)
	}
	if _, err := ParseGroupFilter(`members.value eq "x"`); err == nil {
		t.Fatal("R1 should reject members.value filter")
	}
}

func TestPatchTouchesGroups(t *testing.T) {
	if !PatchTouchesGroups([]PatchOperation{{Op: "replace", Path: "groups", Value: []byte(`[]`)}}) {
		t.Fatal("path groups")
	}
	if !PatchTouchesGroups([]PatchOperation{{Op: "add", Path: "", Value: []byte(`{"groups":[]}`)}}) {
		t.Fatal("overlay groups")
	}
	if PatchTouchesGroups([]PatchOperation{{Op: "replace", Path: "displayName", Value: []byte(`"x"`)}}) {
		t.Fatal("displayName should not touch groups")
	}
}

func TestApplyGroupPatchMembers(t *testing.T) {
	g := &Group{DisplayName: "Sales", Members: []GroupMember{{Value: "a", Type: "User"}}}
	if err := ApplyGroupPatch(g, []PatchOperation{
		{Op: "add", Path: "members", Value: []byte(`{"value":"b","type":"User"}`)},
		{Op: "remove", Path: `members[value eq "a"]`},
	}); err != nil {
		t.Fatal(err)
	}
	if len(g.Members) != 1 || g.Members[0].Value != "b" {
		t.Fatalf("members=%v", g.Members)
	}
	ids, err := g.MemberUserIDs()
	if err != nil || len(ids) != 1 || ids[0] != "b" {
		t.Fatalf("ids=%v err=%v", ids, err)
	}
	if _, err := (&Group{Members: []GroupMember{{Value: "x", Type: "Group"}}}).MemberUserIDs(); err == nil {
		t.Fatal("nested Group member should fail")
	}
}

func TestResourceTypesIncludeGroup(t *testing.T) {
	types := ResourceTypes()
	if len(types) != 2 {
		t.Fatalf("ResourceTypes=%d", len(types))
	}
	cfg := ServiceProviderConfig()
	bulk, _ := cfg["bulk"].(map[string]any)
	if bulk["supported"] != false {
		t.Fatalf("bulk.supported=%v", bulk["supported"])
	}
	if _, ok := FindResourceType("Group"); !ok {
		t.Fatal("missing Group resource type")
	}
	if _, ok := FindSchema(SchemaCoreGroup, nil); !ok {
		t.Fatal("missing Group schema")
	}
}
