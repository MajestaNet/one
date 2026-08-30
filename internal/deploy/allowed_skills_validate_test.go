package deploy

import "testing"

func TestUnknownAllowedSkillIssues(t *testing.T) {
	pb := SnapshotAgentPlaybook{APIName: "SkillAgent__c", AllowedSkills: []string{"GrantMe__c"}}
	issues := unknownAllowedSkillIssues(pb, map[string]struct{}{})
	if len(issues) != 1 || issues[0].Code != "UNKNOWN_SKILL" || issues[0].Severity != "error" {
		t.Fatalf("want one UNKNOWN_SKILL error, got %+v", issues)
	}

	issues = unknownAllowedSkillIssues(pb, map[string]struct{}{"GrantMe__c": {}})
	if len(issues) != 0 {
		t.Fatalf("bundle/install name must pass, got %+v", issues)
	}

	empty := SnapshotAgentPlaybook{APIName: "EmptySkills__c", AllowedSkills: []string{}}
	if got := unknownAllowedSkillIssues(empty, nil); len(got) != 0 {
		t.Fatalf("empty allowedSkills is valid, got %+v", got)
	}

	blank := SnapshotAgentPlaybook{APIName: "Blank__c", AllowedSkills: []string{"  "}}
	issues = unknownAllowedSkillIssues(blank, map[string]struct{}{})
	if len(issues) != 1 || issues[0].Code != "UNKNOWN_SKILL" {
		t.Fatalf("blank entry must error, got %+v", issues)
	}
}
