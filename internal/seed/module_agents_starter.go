package seed

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/MajestaNet/ide/internal/agentharness"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/metadata"
	"github.com/MajestaNet/ide/internal/packages"
)

const AgentsStarterPackageVersion = "1.3.4"

func registerAgentsStarterModule() {
	packages.Register(packages.Module{
		Name:              "agents_starter",
		Version:           AgentsStarterPackageVersion,
		Label:             "Agents starter",
		Description:       "Always-on starter AgentSpecs (AdminSetup, MetadataBuilder, RunCoach, ShipGuide, AccountGuide) cloned as customer-owned, editable playbooks",
		DependsOn:         []string{"core"},
		Optional:          false,
		DocumentationPath: "docs/modules/agents-starter.md",
		AgentSpecTemplates: []packages.AgentSpecTemplate{
			{
				APIName:        "AdminSetup",
				Label:          "Admin setup",
				GoalTemplate:   "Complete first-week Majesta One admin setup for this install using {{focus}}",
				PrimarySection: "govern",
				JobClass:       "govern",
				Instructions: `You are an AdminSetup agent operating inside a customer Majesta One install.
You may only use Majesta One Client and Metadata APIs available to your principal.
You are not a Majesta One product engineer and must not reference vendor source paths.
Help the customer admin enable packages, review describe/schema, and create sample records when granted.
Prefer describe and query before writes. High-risk writes require human approval.`,
				AllowedTools:    []string{"sobjects.read", "sobjects.write", "query", "search"},
				ObjectScopes:    []string{},
				RequireApproval: true,
			},
			{
				APIName:        "MetadataBuilder",
				Label:          "Metadata builder",
				GoalTemplate:   "Propose or apply customer metadata changes for {{focus}}",
				PrimarySection: "build",
				JobClass:       "customize",
				Instructions: `You are a MetadataBuilder agent operating inside a customer Majesta One install.
You help design customer-owned objects, fields, and validation using the Metadata API when your principal has metadata scope.
Never mutate managed package definitions. Prefer dry-run and approval for writes.
You are not a Majesta One product engineer.`,
				AllowedTools:    []string{"sobjects.read", "query"},
				ObjectScopes:    []string{},
				RequireApproval: true,
			},
			{
				APIName:      "RunCoach",
				Label:        "Run coach",
				GoalTemplate: "Organize the personal Operate graph or compose a reusable Tool for {{focus}}",
				// Stored token is `run` (jobClass operate). Four-tile docks alias run→Operate;
				// BindSpec rejects primarySection=operate with jobClass=operate (that pair is query).
				PrimarySection: "run",
				JobClass:       "operate",
				Instructions: `You are a RunCoach agent operating inside a customer Majesta One install.
Prefer graph.get, graph.pin, graph.cluster, graph.mountTool, graph.link, graph.unlink, graph.annotate, and graph.layout through the IDE bridge to organize the user's reference-only Run home.
Never put query results, record fields, rows, or hydrated cards into graph operations.
Use the Curator role for My day topology, live signal triage, insights, and stale attention. Use the Doer role to stage Client-shaped proposals for human Apply; never silently mutate CRM through graph state.
Emit graphCalls / proposal / boardHandoff inside a final oneEffects JSON fence so the IDE can apply them; never invent field maps inside graph.* inputs.
After a personal workflow succeeds repeatedly, suggest graph.publishSubgraph to turn the selected subgraph into an org ToolSpec playbook. Never imply that the personal graph itself is shared.
Help business users compose declarative Tools (one.canvas/v1 documents) for reusable pipelines, queues, and daily work.
Use tool.create, tool.update, tool.rerun, and tool.saveAsSpec through the IDE bridge when available.
Hosted /client/v1/agents/runs ignores graphCalls / proposal / boardHandoff; those stay optional-client Apply.
Never reference vendor source paths. Prefer query before writes; high-risk mutations require human approval.`,
				AllowedTools:    []string{"sobjects.read", "query"},
				ObjectScopes:    []string{},
				RequireApproval: true,
			},
			{
				APIName:      "ShipGuide",
				Label:        "Ship guide",
				GoalTemplate: "Advise on validate / deploy readiness for {{focus}}",
				// Stored token is `ship` (jobClass ship). Four-tile docks alias ship→Build.
				PrimarySection: "ship",
				JobClass:       "ship",
				Instructions: `You are a ShipGuide agent operating inside a customer Majesta One install.
Advise on change sets and promote readiness. Prefer read and explain.
Ship with one org validate then org deploy (or MCP org_validate / org_deploy / pack). Do not peer-promote between installs.
Do not execute Ops image rolls or Deploy cloud mutations via hosted agent tools.
You are not a Majesta One product engineer.`,
				AllowedTools:    []string{"sobjects.read", "query"},
				ObjectScopes:    []string{},
				RequireApproval: true,
			},
			{
				APIName:        "AccountGuide",
				Label:          "Account guide",
				GoalTemplate:   "Orient the admin on Account, Hosting, or Inference for {{focus}}",
				PrimarySection: "settings",
				JobClass:       "govern",
				Instructions: `You are an AccountGuide agent operating on this Majesta One install.
Orient operators on Account preferences, Hosting, and Inference via Metadata/Deploy HTTP or the one CLI — not an in-IDE settings host.
Prefer read-only guidance. Never echo Hosting secrets, API keys, or tokens.
Privileged changes stay on family HTTP or CLI. You are not a Majesta One product engineer.`,
				AllowedTools:    []string{"sobjects.read", "query"},
				ObjectScopes:    []string{},
				RequireApproval: true,
			},
		},
	})
}

// CloneAgentSpecTemplates inserts missing customer-owned AgentSpecs from module templates.
// Existing api_name rows are never overwritten.
func CloneAgentSpecTemplates(ctx context.Context, pool *db.Pool, m packages.Module) error {
	if pool == nil || len(m.AgentSpecTemplates) == 0 {
		return nil
	}
	pkg := "customer.default"
	for _, t := range m.AgentSpecTemplates {
		section := t.PrimarySection
		if section == "" {
			if s, ok := agentharness.StarterSection(t.APIName); ok {
				section = string(s)
			}
		}
		binding, err := agentharness.BindSpec(t.JobClass, section)
		if err != nil {
			return fmt.Errorf("clone AgentSpec %s: %w", t.APIName, err)
		}
		tools := agentharness.EnsureToolFloor(binding.ToolFloor, t.AllowedTools)
		if tools == nil {
			tools = []string{}
		}
		scopes := t.ObjectScopes
		if scopes == nil {
			scopes = []string{}
		}
		requireApproval := agentharness.EffectiveRequireApproval(binding.RequireApprovalDefault, t.RequireApproval)
		toolsJSON, _ := json.Marshal(tools)
		scopesJSON, _ := json.Marshal(scopes)
		tag, err := pool.Exec(ctx, `
INSERT INTO agent_playbooks (
  api_name, label, goal_template, instructions, allowed_tools, object_scopes,
  require_approval, active, ownership, package_name,
  primary_section, harness_id, harness_version, job_class
)
VALUES ($1,$2,$3,$4,$5::jsonb,$6::jsonb,$7,true,'custom',$8,$9,$10,$11,$12)
ON CONFLICT (api_name) DO NOTHING`,
			t.APIName, t.Label, t.GoalTemplate, t.Instructions,
			string(toolsJSON), string(scopesJSON), requireApproval, pkg,
			nullableStarterText(binding.PrimarySection), binding.HarnessID, binding.HarnessVersion,
			nullableStarterText(binding.JobClass))
		if err != nil {
			return fmt.Errorf("clone AgentSpec %s: %w", t.APIName, err)
		}
		_ = tag
	}
	return nil
}

// InstallAgentsStarter migrates the always-on agents starter templates and records package version.
// Customers can define additional AgentSpecs anytime via Metadata; this only seeds day-one playbooks.
func InstallAgentsStarter(ctx context.Context, meta *metadata.Service) error {
	m, ok := packages.Get("agents_starter")
	if !ok {
		return fmt.Errorf("agents_starter package not registered")
	}
	if err := syncModuleDefs(ctx, meta, m); err != nil {
		return err
	}
	pool := meta.Pool()
	if pool == nil {
		return fmt.Errorf("metadata pool unavailable for agents_starter clone")
	}
	if err := CloneAgentSpecTemplates(ctx, pool, m); err != nil {
		return err
	}
	if err := meta.RecordPackageInstall(ctx, m.Name, m.Version); err != nil {
		return fmt.Errorf("record agents_starter package install: %w", err)
	}
	return nil
}

// cloneAgentsStarterAfterEnable is called after package sync/record for agents_starter.
func cloneAgentsStarterAfterEnable(ctx context.Context, meta *metadata.Service, m packages.Module) error {
	if m.Name != "agents_starter" {
		return nil
	}
	pool := meta.Pool()
	if pool == nil {
		return fmt.Errorf("metadata pool unavailable for agents_starter clone")
	}
	return CloneAgentSpecTemplates(ctx, pool, m)
}

func nullableStarterText(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}
