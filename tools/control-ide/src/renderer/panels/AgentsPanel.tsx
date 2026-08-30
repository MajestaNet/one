import { useCallback, useEffect, useMemo, useState } from "react";
import type { AppBridge } from "../App";
import {
  listAgentHarnesses,
  PRIMARY_SECTIONS,
  SECTION_INSTRUCTION_STUBS,
  sectionLabel,
  sectionTagline,
  slugApiName,
  type AgentHarness,
} from "../agents/sections";
import { mirrorPlaybookYaml } from "../metadataMirror";
import type { AppSection } from "../workspace/types";
import { Button, EmptyState, PanelHeader, SearchField, StatusBadge, ToolSurface, ToolToolbar } from "../ui";
import { IconChevronLeft, IconAgents, modeIcon } from "../icons/Icons";

export type AgentPlaybook = {
  apiName: string;
  label: string;
  goalTemplate?: string;
  instructions?: string;
  primarySection?: string;
  harnessId?: string;
  harnessVersion?: string;
  allowedTools?: string[];
  objectScopes?: string[];
  allowedSkills?: string[];
  requireApproval?: boolean;
  active?: boolean;
  ownership?: string;
  packageName?: string;
};

const TOOL_OPTIONS = ["sobjects.read", "sobjects.write", "query"];

type WizardStep = "section" | "harness" | "identity" | "behavior" | "review";

const WIZARD_STEPS: WizardStep[] = ["section", "harness", "identity", "behavior", "review"];

function sortPlaybooks(rows: AgentPlaybook[]): AgentPlaybook[] {
  return [...rows].sort((a, b) => {
    const la = (a.label || a.apiName).toLowerCase();
    const lb = (b.label || b.apiName).toLowerCase();
    return la.localeCompare(lb);
  });
}

function emptyWizardForm() {
  return {
    primarySection: "" as AppSection | "",
    label: "",
    apiName: "",
    goalTemplate: "",
    instructions: "",
    requireApproval: true,
    allowedTools: "" as string,
    apiNameTouched: false,
  };
}

export function AgentsPanel({
  bridge,
  refreshKey = 0,
  onCatalogChanged,
}: {
  bridge: AppBridge;
  refreshKey?: number;
  /** Notify shell to reload live playbook catalog (dock). */
  onCatalogChanged?: () => void;
}) {
  const [playbooks, setPlaybooks] = useState<AgentPlaybook[]>([]);
  const [harnesses, setHarnesses] = useState<AgentHarness[]>([]);
  const [selected, setSelected] = useState<string | null>(null);
  const [detail, setDetail] = useState<AgentPlaybook | null>(null);
  const [err, setErr] = useState("");
  const [warn, setWarn] = useState("");
  const [toast, setToast] = useState("");
  const [busy, setBusy] = useState(false);
  const [filter, setFilter] = useState("");
  const [sectionFilter, setSectionFilter] = useState<AppSection | "all">("all");
  const [showNew, setShowNew] = useState(false);
  const [wizardStep, setWizardStep] = useState<WizardStep>("section");
  const [form, setForm] = useState(emptyWizardForm);
  const [moveSection, setMoveSection] = useState<AppSection | "">("");
  const [confirmMove, setConfirmMove] = useState(false);

  const loadList = useCallback(async () => {
    if (!bridge.session?.token) {
      setPlaybooks([]);
      return;
    }
    setBusy(true);
    setErr("");
    try {
      const row = (await bridge.fetch("/metadata/v1/agents/playbooks")) as {
        playbooks?: AgentPlaybook[];
      };
      setPlaybooks(sortPlaybooks(row.playbooks ?? []));
    } catch (e) {
      setErr(String(e));
      setPlaybooks([]);
    } finally {
      setBusy(false);
    }
  }, [bridge]);

  const loadHarnesses = useCallback(async () => {
    if (!bridge.session?.token) {
      setHarnesses([]);
      return;
    }
    try {
      setHarnesses(await listAgentHarnesses(bridge.fetch));
    } catch {
      setHarnesses([]);
    }
  }, [bridge]);

  const loadDetail = useCallback(
    async (apiName: string) => {
      setBusy(true);
      setErr("");
      try {
        const desc = (await bridge.fetch(
          `/metadata/v1/agents/playbooks/${encodeURIComponent(apiName)}`,
        )) as AgentPlaybook;
        setDetail(desc);
        setSelected(apiName);
        setShowNew(false);
        setMoveSection((desc.primarySection as AppSection) || "");
        setConfirmMove(false);
      } catch (e) {
        setErr(String(e));
        setDetail(null);
      } finally {
        setBusy(false);
      }
    },
    [bridge],
  );

  const backToList = () => {
    setSelected(null);
    setDetail(null);
    setWarn("");
    setToast("");
  };

  const resetWizard = () => {
    setShowNew(false);
    setWizardStep("section");
    setForm(emptyWizardForm());
  };

  useEffect(() => {
    void loadList();
    void loadHarnesses();
    setSelected(null);
    setDetail(null);
    setWarn("");
    setToast("");
  }, [loadList, loadHarnesses, refreshKey]);

  const selectedHarness = useMemo(() => {
    if (!form.primarySection) return null;
    return harnesses.find((h) => h.section === form.primarySection) ?? null;
  }, [form.primarySection, harnesses]);

  const stepIndex = WIZARD_STEPS.indexOf(wizardStep);

  const canAdvance = (): boolean => {
    switch (wizardStep) {
      case "section":
        return Boolean(form.primarySection);
      case "harness":
        return Boolean(form.primarySection);
      case "identity":
        return Boolean(form.label.trim() && form.apiName.trim());
      case "behavior":
        return true;
      case "review":
        return Boolean(form.primarySection && form.label.trim() && form.apiName.trim());
      default:
        return false;
    }
  };

  const pickSection = (section: AppSection) => {
    const harness = harnesses.find((h) => h.section === section);
    setForm((f) => ({
      ...f,
      primarySection: section,
      requireApproval: harness?.requireApprovalDefault !== false,
      instructions: f.instructions.trim() ? f.instructions : SECTION_INSTRUCTION_STUBS[section],
      allowedTools: f.allowedTools.trim()
        ? f.allowedTools
        : (harness?.toolFloor ?? ["sobjects.read", "query"]).join(", "),
      goalTemplate: f.goalTemplate.trim()
        ? f.goalTemplate
        : `Help with {{focus}} in ${sectionLabel(section)}`,
    }));
  };

  const createPlaybook = async () => {
    if (!form.primarySection) {
      setErr("primarySection is required");
      return;
    }
    setErr("");
    setWarn("");
    setToast("");
    setBusy(true);
    try {
      const tools = form.allowedTools
        .split(",")
        .map((t) => t.trim())
        .filter(Boolean);
      const body = {
        apiName: form.apiName.trim(),
        label: form.label.trim(),
        goalTemplate: form.goalTemplate.trim(),
        instructions: form.instructions.trim(),
        primarySection: form.primarySection,
        requireApproval: form.requireApproval,
        allowedTools: tools,
        objectScopes: [] as string[],
      };
      const created = (await bridge.fetch("/metadata/v1/agents/playbooks", {
        method: "POST",
        body: JSON.stringify(body),
      })) as AgentPlaybook;
      const mirrorWarn = await mirrorPlaybookYaml(bridge.session?.repoPath, {
        apiName: created.apiName || body.apiName,
        label: created.label || body.label,
        goalTemplate: created.goalTemplate || body.goalTemplate,
        instructions: created.instructions || body.instructions,
        primarySection: created.primarySection || body.primarySection,
        harnessId: created.harnessId,
        harnessVersion: created.harnessVersion,
        allowedTools: created.allowedTools || body.allowedTools,
        objectScopes: created.objectScopes || [],
        requireApproval: created.requireApproval ?? body.requireApproval,
        active: created.active ?? true,
        ownership: created.ownership || "custom",
        packageName: created.packageName || "customer.default",
      });
      if (mirrorWarn) setWarn(mirrorWarn);
      const home = sectionLabel((created.primarySection || body.primarySection) as AppSection);
      setToast(`Appears in ${home} dock`);
      resetWizard();
      await loadList();
      onCatalogChanged?.();
      await loadDetail(created.apiName || body.apiName);
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const savePlaybook = async () => {
    if (!detail?.apiName || detail.ownership !== "custom") return;
    setErr("");
    setWarn("");
    setBusy(true);
    try {
      const patched = (await bridge.fetch(
        `/metadata/v1/agents/playbooks/${encodeURIComponent(detail.apiName)}`,
        {
          method: "PATCH",
          body: JSON.stringify({
            label: detail.label,
            goalTemplate: detail.goalTemplate,
            instructions: detail.instructions,
            requireApproval: detail.requireApproval,
            active: detail.active,
            allowedTools: detail.allowedTools,
            objectScopes: detail.objectScopes,
          }),
        },
      )) as AgentPlaybook;
      const mirrorWarn = await mirrorPlaybookYaml(bridge.session?.repoPath, {
        apiName: patched.apiName || detail.apiName,
        label: patched.label || detail.label,
        goalTemplate: patched.goalTemplate ?? detail.goalTemplate,
        instructions: patched.instructions ?? detail.instructions,
        primarySection: patched.primarySection ?? detail.primarySection,
        harnessId: patched.harnessId ?? detail.harnessId,
        harnessVersion: patched.harnessVersion ?? detail.harnessVersion,
        allowedTools: patched.allowedTools || detail.allowedTools,
        objectScopes: patched.objectScopes || detail.objectScopes,
        requireApproval: patched.requireApproval ?? detail.requireApproval,
        active: patched.active ?? detail.active,
        ownership: patched.ownership || detail.ownership,
        packageName: patched.packageName || detail.packageName,
      });
      if (mirrorWarn) setWarn(mirrorWarn);
      await loadList();
      onCatalogChanged?.();
      await loadDetail(detail.apiName);
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const movePrimarySection = async () => {
    if (!detail?.apiName || detail.ownership !== "custom" || !moveSection) return;
    if (moveSection === detail.primarySection) return;
    if (!confirmMove) {
      setErr("Confirm move to re-apply the section harness floor.");
      return;
    }
    setErr("");
    setWarn("");
    setToast("");
    setBusy(true);
    try {
      const patched = (await bridge.fetch(
        `/metadata/v1/agents/playbooks/${encodeURIComponent(detail.apiName)}`,
        {
          method: "PATCH",
          body: JSON.stringify({ primarySection: moveSection }),
        },
      )) as AgentPlaybook;
      const mirrorWarn = await mirrorPlaybookYaml(bridge.session?.repoPath, {
        apiName: patched.apiName || detail.apiName,
        label: patched.label || detail.label,
        goalTemplate: patched.goalTemplate ?? detail.goalTemplate,
        instructions: patched.instructions ?? detail.instructions,
        primarySection: patched.primarySection || moveSection,
        harnessId: patched.harnessId,
        harnessVersion: patched.harnessVersion,
        allowedTools: patched.allowedTools || detail.allowedTools,
        objectScopes: patched.objectScopes || detail.objectScopes,
        requireApproval: patched.requireApproval ?? detail.requireApproval,
        active: patched.active ?? detail.active,
        ownership: patched.ownership || detail.ownership,
        packageName: patched.packageName || detail.packageName,
      });
      if (mirrorWarn) setWarn(mirrorWarn);
      setToast(`Moved to ${sectionLabel(moveSection)} dock · harness re-applied`);
      setConfirmMove(false);
      await loadList();
      onCatalogChanged?.();
      await loadDetail(detail.apiName);
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const filtered = useMemo(() => {
    const q = filter.trim().toLowerCase();
    return playbooks.filter((p) => {
      if (sectionFilter !== "all" && (p.primarySection || "") !== sectionFilter) return false;
      if (!q) return true;
      return (
        p.apiName.toLowerCase().includes(q) ||
        (p.label || "").toLowerCase().includes(q) ||
        (p.goalTemplate || "").toLowerCase().includes(q) ||
        (p.primarySection || "").toLowerCase().includes(q)
      );
    });
  }, [playbooks, filter, sectionFilter]);

  if (!bridge.session?.token) {
    return (
      <ToolSurface testId="agents-panel">
        <PanelHeader
          title="Agents"
          subtitle="Define AgentSpecs (playbooks) on the active environment — declarative Metadata."
        />
        <EmptyState
          icon={<IconAgents size={28} />}
          title="Connect an environment"
          description="Agents reads and writes Metadata playbooks on the active install. Use the top-bar env switcher."
        />
      </ToolSurface>
    );
  }

  return (
    <ToolSurface className="agents-panel" testId="agents-panel">
      <PanelHeader
        title="Agents"
        subtitle="Declarative AgentSpecs bound to one IDE section harness. Dual-writes one YAML when a local repo path is set."
      />
      {err && <p className="err">{err}</p>}
      {warn && <p className="muted" data-testid="mirror-warn">{warn}</p>}
      {toast && (
        <p className="muted" data-testid="agents-create-toast">
          {toast}
        </p>
      )}
      {!(selected && detail) ? (
        <ToolToolbar
          actions={
            <>
              <Button
                variant="primary"
                onClick={() => {
                  if (showNew) {
                    resetWizard();
                  } else {
                    setShowNew(true);
                    setWizardStep("section");
                    setForm(emptyWizardForm());
                    void loadHarnesses();
                  }
                }}
                data-testid="agents-new"
              >
                {showNew ? "Cancel" : "New agent"}
              </Button>
              <Button variant="secondary" busy={busy} onClick={() => void loadList()}>
                Refresh
              </Button>
            </>
          }
          meta={
            <span className="muted om-count">
              {filtered.length === playbooks.length
                ? `${playbooks.length} agent${playbooks.length === 1 ? "" : "s"}`
                : `${filtered.length} of ${playbooks.length}`}
            </span>
          }
          search={
            <SearchField
              value={filter}
              onChange={setFilter}
              placeholder="Search agents…"
              label="Search agents"
              testId="agents-search"
            />
          }
        />
      ) : null}

      {selected && detail ? (
        <div className="om-detail-view" data-testid="agents-detail">
          <div className="om-detail-nav">
            <Button variant="ghost" onClick={backToList} data-testid="agents-back">
              <IconChevronLeft size={14} aria-hidden /> All agents
            </Button>
          </div>
          <div className="om-form">
            <h3>{detail.apiName}</h3>
            <div className="row" style={{ gap: "0.5rem", marginBottom: "0.75rem" }}>
              {detail.primarySection ? (
                <StatusBadge tone="accent">{sectionLabel(detail.primarySection as AppSection)}</StatusBadge>
              ) : null}
              {detail.harnessId ? <StatusBadge tone="neutral">{detail.harnessId}</StatusBadge> : null}
            </div>
            <label>
              Label
              <input
                value={detail.label || ""}
                onChange={(e) => setDetail({ ...detail, label: e.target.value })}
                disabled={detail.ownership !== "custom"}
              />
            </label>
            <label>
              Goal template
              <input
                value={detail.goalTemplate || ""}
                onChange={(e) => setDetail({ ...detail, goalTemplate: e.target.value })}
                disabled={detail.ownership !== "custom"}
              />
            </label>
            <label>
              Instructions
              <textarea
                value={detail.instructions || ""}
                onChange={(e) => setDetail({ ...detail, instructions: e.target.value })}
                disabled={detail.ownership !== "custom"}
                rows={6}
              />
            </label>
            <label>
              Allowed tools (comma-separated)
              <input
                value={(detail.allowedTools ?? []).join(", ")}
                onChange={(e) =>
                  setDetail({
                    ...detail,
                    allowedTools: e.target.value
                      .split(",")
                      .map((t) => t.trim())
                      .filter(Boolean),
                  })
                }
                disabled={detail.ownership !== "custom"}
                placeholder={TOOL_OPTIONS.join(", ")}
              />
            </label>
            <label className="row">
              <input
                type="checkbox"
                checked={Boolean(detail.requireApproval)}
                onChange={(e) => setDetail({ ...detail, requireApproval: e.target.checked })}
                disabled={detail.ownership !== "custom"}
              />
              Require approval before tool runs
            </label>
            <label className="row">
              <input
                type="checkbox"
                checked={detail.active !== false}
                onChange={(e) => setDetail({ ...detail, active: e.target.checked })}
                disabled={detail.ownership !== "custom"}
              />
              Active
            </label>
            {detail.ownership === "custom" ? (
              <div className="agents-move-section" data-testid="agents-move-section">
                <h4>Move section</h4>
                <p className="muted">
                  Changing primary section re-applies that section’s Majesta One harness floor (tools + approval
                  default). The agent will appear only in the new section dock.
                </p>
                <label>
                  Primary section
                  <select
                    value={moveSection}
                    onChange={(e) => {
                      setMoveSection(e.target.value as AppSection);
                      setConfirmMove(false);
                    }}
                    data-testid="agents-move-select"
                  >
                    {PRIMARY_SECTIONS.map((s) => (
                      <option key={s} value={s}>
                        {sectionLabel(s)}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="row">
                  <input
                    type="checkbox"
                    checked={confirmMove}
                    onChange={(e) => setConfirmMove(e.target.checked)}
                    data-testid="agents-move-confirm"
                    disabled={!moveSection || moveSection === detail.primarySection}
                  />
                  I understand the harness floor will be re-applied
                </label>
                <Button
                  variant="secondary"
                  busy={busy}
                  disabled={!moveSection || moveSection === detail.primarySection || !confirmMove}
                  onClick={() => void movePrimarySection()}
                  data-testid="agents-move-submit"
                >
                  Move section…
                </Button>
              </div>
            ) : (
              <p className="muted">Primary section is read-only for managed AgentSpecs.</p>
            )}
            <div className="row">
              <StatusBadge tone="neutral">{detail.ownership || "—"}</StatusBadge>
              <StatusBadge tone={detail.active !== false ? "accent" : "neutral"}>
                {detail.active !== false ? "active" : "inactive"}
              </StatusBadge>
              {detail.ownership === "custom" ? (
                <Button variant="secondary" busy={busy} onClick={() => void savePlaybook()}>
                  Save agent
                </Button>
              ) : (
                <p className="muted">Managed AgentSpecs are read-only here.</p>
              )}
            </div>
          </div>
        </div>
      ) : (
        <div className="om-list-view" data-testid="agents-list-view">
          <div className="agents-section-filters" data-testid="agents-section-filters">
            <button
              type="button"
              className={`agents-section-chip ${sectionFilter === "all" ? "active" : ""}`}
              onClick={() => setSectionFilter("all")}
            >
              All
            </button>
            {PRIMARY_SECTIONS.map((s) => (
              <button
                key={s}
                type="button"
                className={`agents-section-chip ${sectionFilter === s ? "active" : ""}`}
                onClick={() => setSectionFilter(s)}
                data-testid={`agents-filter-${s}`}
              >
                {sectionLabel(s)}
              </button>
            ))}
          </div>

          {showNew ? (
            <div className="om-form om-new-object-form agents-wizard" data-testid="agents-new-form">
              <div className="agents-wizard-steps" data-testid="agents-wizard-steps">
                {WIZARD_STEPS.map((s, i) => (
                  <span
                    key={s}
                    className={`agents-wizard-step ${s === wizardStep ? "active" : ""} ${
                      i < stepIndex ? "done" : ""
                    }`}
                  >
                    {i + 1}. {s}
                  </span>
                ))}
              </div>

              {wizardStep === "section" ? (
                <div data-testid="agents-wizard-section">
                  <p className="muted">Pick the primary IDE section. Majesta One applies that section’s harness.</p>
                  <div className="starter-pack-grid agents-section-grid">
                    {PRIMARY_SECTIONS.map((s) => (
                      <button
                        key={s}
                        type="button"
                        className={`starter-pack-card agents-section-card ${
                          form.primarySection === s ? "selected" : ""
                        }`}
                        data-testid={`agents-section-${s}`}
                        onClick={() => pickSection(s)}
                      >
                        {modeIcon(s, { size: 18 })}
                        <h4>{sectionLabel(s)}</h4>
                        <p>{sectionTagline(s)}</p>
                      </button>
                    ))}
                  </div>
                </div>
              ) : null}

              {wizardStep === "harness" && form.primarySection ? (
                <div data-testid="agents-wizard-harness">
                  <h4>
                    {selectedHarness?.label || sectionLabel(form.primarySection)} harness
                  </h4>
                  <p className="muted">{selectedHarness?.job || "Section harness floor"}</p>
                  <ul className="agents-harness-preview">
                    <li>
                      <strong>Id:</strong> {selectedHarness?.id || "—"}
                    </li>
                    <li>
                      <strong>Tool floor:</strong>{" "}
                      {(selectedHarness?.toolFloor ?? ["sobjects.read", "query"]).join(", ")}
                    </li>
                    <li>
                      <strong>Approval default:</strong>{" "}
                      {selectedHarness?.requireApprovalDefault === false ? "optional" : "required"}
                    </li>
                    {selectedHarness?.contextPackHints?.length ? (
                      <li>
                        <strong>Context:</strong> {selectedHarness.contextPackHints.join(", ")}
                      </li>
                    ) : null}
                  </ul>
                </div>
              ) : null}

              {wizardStep === "identity" ? (
                <div data-testid="agents-wizard-identity">
                  <label>
                    Label
                    <input
                      value={form.label}
                      onChange={(e) => {
                        const label = e.target.value;
                        setForm((f) => ({
                          ...f,
                          label,
                          apiName: f.apiNameTouched ? f.apiName : slugApiName(label),
                        }));
                      }}
                      placeholder="Query assistant"
                      data-testid="agents-wizard-label"
                    />
                  </label>
                  <label>
                    API name
                    <input
                      value={form.apiName}
                      onChange={(e) =>
                        setForm((f) => ({ ...f, apiName: e.target.value, apiNameTouched: true }))
                      }
                      placeholder="QueryAssistant__c"
                      data-testid="agents-wizard-apiName"
                    />
                  </label>
                  <label style={{ gridColumn: "1 / -1" }}>
                    Goal template
                    <input
                      value={form.goalTemplate}
                      onChange={(e) => setForm((f) => ({ ...f, goalTemplate: e.target.value }))}
                      placeholder="Answer questions about {{focus}}"
                    />
                  </label>
                </div>
              ) : null}

              {wizardStep === "behavior" ? (
                <div data-testid="agents-wizard-behavior">
                  <label style={{ gridColumn: "1 / -1" }}>
                    Instructions
                    <textarea
                      value={form.instructions}
                      onChange={(e) => setForm((f) => ({ ...f, instructions: e.target.value }))}
                      rows={5}
                    />
                  </label>
                  <label style={{ gridColumn: "1 / -1" }}>
                    Allowed tools (widen above harness floor)
                    <input
                      value={form.allowedTools}
                      onChange={(e) => setForm((f) => ({ ...f, allowedTools: e.target.value }))}
                    />
                  </label>
                  <label className="row">
                    <input
                      type="checkbox"
                      checked={form.requireApproval}
                      onChange={(e) => setForm((f) => ({ ...f, requireApproval: e.target.checked }))}
                    />
                    Require approval
                  </label>
                </div>
              ) : null}

              {wizardStep === "review" ? (
                <div data-testid="agents-wizard-review">
                  <ul className="agents-harness-preview">
                    <li>
                      <strong>Section:</strong>{" "}
                      {form.primarySection ? sectionLabel(form.primarySection) : "—"}
                    </li>
                    <li>
                      <strong>Harness:</strong> {selectedHarness?.id || "—"}
                    </li>
                    <li>
                      <strong>Label:</strong> {form.label || "—"}
                    </li>
                    <li>
                      <strong>API name:</strong> {form.apiName || "—"}
                    </li>
                    <li>
                      <strong>Tools:</strong> {form.allowedTools || "—"}
                    </li>
                    <li>
                      <strong>Approval:</strong> {form.requireApproval ? "required" : "optional"}
                    </li>
                  </ul>
                </div>
              ) : null}

              <div className="row agents-wizard-nav">
                <Button
                  variant="secondary"
                  disabled={stepIndex <= 0}
                  onClick={() => setWizardStep(WIZARD_STEPS[Math.max(0, stepIndex - 1)])}
                  data-testid="agents-wizard-back"
                >
                  Back
                </Button>
                {wizardStep !== "review" ? (
                  <Button
                    variant="primary"
                    disabled={!canAdvance()}
                    onClick={() => setWizardStep(WIZARD_STEPS[Math.min(WIZARD_STEPS.length - 1, stepIndex + 1)])}
                    data-testid="agents-wizard-next"
                  >
                    Next
                  </Button>
                ) : (
                  <Button
                    variant="primary"
                    busy={busy}
                    disabled={!canAdvance()}
                    onClick={() => void createPlaybook()}
                    data-testid="agents-wizard-create"
                  >
                    Create agent
                  </Button>
                )}
              </div>
            </div>
          ) : null}

          <div className="data-table-wrap om-table-wrap" data-testid="agents-list">
            <table className="data-table om-object-table">
              <thead>
                <tr>
                  <th>Label</th>
                  <th>Section</th>
                  <th>API name</th>
                  <th>Status</th>
                  <th>Ownership</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((p) => (
                  <tr
                    key={p.apiName}
                    className="om-object-row"
                    data-testid={`agents-row-${p.apiName}`}
                    onClick={() => void loadDetail(p.apiName)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" || e.key === " ") {
                        e.preventDefault();
                        void loadDetail(p.apiName);
                      }
                    }}
                    tabIndex={0}
                    role="button"
                    aria-label={`Open ${p.label || p.apiName}`}
                  >
                    <td>{p.label || p.apiName}</td>
                    <td>
                      {p.primarySection ? (
                        <StatusBadge tone="accent">
                          {sectionLabel(p.primarySection as AppSection)}
                        </StatusBadge>
                      ) : (
                        "—"
                      )}
                    </td>
                    <td className="mono">{p.apiName}</td>
                    <td>
                      <StatusBadge tone={p.active !== false ? "accent" : "neutral"}>
                        {p.active !== false ? "active" : "inactive"}
                      </StatusBadge>
                      {p.requireApproval ? (
                        <span className="muted" style={{ marginLeft: "0.35rem" }}>
                          approval
                        </span>
                      ) : null}
                    </td>
                    <td>
                      {p.ownership ? (
                        <StatusBadge tone={p.ownership === "custom" ? "accent" : "neutral"}>
                          {p.ownership}
                        </StatusBadge>
                      ) : (
                        "—"
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            {!filtered.length && !busy ? (
              <p className="data-table-empty muted">
                No agents yet. Create one, or enable the <code>agents_starter</code> package.
              </p>
            ) : null}
          </div>
        </div>
      )}
    </ToolSurface>
  );
}
