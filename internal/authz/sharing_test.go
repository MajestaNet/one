package authz

import (
	"os"
	"regexp"
	"testing"
)

func TestSharingDoesNotReferenceAPIRoleParentColumn(t *testing.T) {
	src, err := os.ReadFile("sharing.go")
	if err != nil {
		t.Fatal(err)
	}
	if regexp.MustCompile(`(?i)JOIN\s+roles\b[\s\S]{0,200}parent_role_id`).Match(src) {
		t.Fatal("sharing evaluators must not JOIN roles.parent_role_id")
	}
}

func TestIsAncestorRole(t *testing.T) {
	ceo := "ceo"
	mgr := "mgr"
	rep := "rep"
	h := HierarchyIndex{
		ceo: nil,
		mgr: &ceo,
		rep: &mgr,
	}
	if !IsAncestorRole(h, ceo, rep) {
		t.Fatal("CEO should be ancestor of rep")
	}
	if !IsAncestorRole(h, mgr, rep) {
		t.Fatal("manager should be ancestor of rep")
	}
	if IsAncestorRole(h, rep, mgr) {
		t.Fatal("rep should not be ancestor of manager")
	}
}

func TestCanViewWithSharing(t *testing.T) {
	sh := SharingContext{RecordSharingEnabled: true, DefaultAccess: DefaultAccessPrivate}
	h := HierarchyIndex{"mgr": nil, "rep": strPtr("mgr")}

	in := RecordAccessInput{
		ActorID: "u1", ActorDataRoleID: "mgr",
		OwnerID: "u2", OwnerDataRoleID: "rep",
		HasObjectRead: true,
	}
	if !CanViewWithSharing(in, sh, h, false) {
		t.Fatal("manager should view rep-owned record")
	}

	pub := SharingContext{RecordSharingEnabled: true, DefaultAccess: DefaultAccessPublicRead}
	in2 := RecordAccessInput{ActorID: "u3", HasObjectRead: true}
	if !CanViewWithSharing(in2, pub, h, false) {
		t.Fatal("public read should allow view")
	}
}

func TestValidateSharingAccessLevel(t *testing.T) {
	if err := ValidateSharingAccessLevel("read"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSharingAccessLevel("read_write"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSharingAccessLevel("write"); err == nil {
		t.Fatal("expected error for write")
	}
}

func TestCanModifyWithSharingRequiresReadWriteGrant(t *testing.T) {
	sh := SharingContext{RecordSharingEnabled: true, DefaultAccess: DefaultAccessPrivate}
	h := HierarchyIndex{}
	in := RecordAccessInput{
		ActorID: "u1", OwnerID: "u2", HasObjectUpdate: true,
	}
	if CanModifyWithSharing(in, sh, h, false) {
		t.Fatal("read-only grant / no grant must not allow modify")
	}
	if !CanModifyWithSharing(in, sh, h, true) {
		t.Fatal("read_write grant should allow modify")
	}
}

func strPtr(s string) *string { return &s }
