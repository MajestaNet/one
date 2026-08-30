import { useCallback, useEffect, useMemo, useState } from "react";
import type { AppBridge } from "../App";
import { mirrorFieldYaml, mirrorObjectYaml } from "../metadataMirror";
import { Button, EmptyState, PanelHeader, StatusBadge, SearchField, ToolSurface, ToolToolbar } from "../ui";
import { IconChevronLeft, IconMetadata } from "../icons/Icons";

type MetaObject = {
  apiName: string;
  label: string;
  pluralLabel?: string;
  storageMode?: string;
  ownership?: string;
  features?: Record<string, boolean>;
};

type MetaField = {
  apiName: string;
  label: string;
  fieldType: string;
  required?: boolean;
  uniqueField?: boolean;
  indexed?: boolean;
  ownership?: string;
  objectApiName?: string;
  length?: number;
  referenceTo?: string;
  relationshipName?: string;
  picklistValues?: string[];
  autonumberFormat?: string;
  autonumberStart?: number;
};

type FieldTypeInfo = {
  apiName: string;
  label: string;
  requiresReferenceTo?: boolean;
  supportsPicklistValues?: boolean;
  supportsAutonumber?: boolean;
};

type DescribePayload = MetaObject & { fields?: MetaField[] };

const FALLBACK_FIELD_TYPES: FieldTypeInfo[] = [
  { apiName: "text", label: "Text" },
  { apiName: "textarea", label: "Text Area" },
  { apiName: "email", label: "Email" },
  { apiName: "phone", label: "Phone" },
  { apiName: "url", label: "URL" },
  { apiName: "boolean", label: "Checkbox" },
  { apiName: "integer", label: "Integer" },
  { apiName: "number", label: "Number" },
  { apiName: "currency", label: "Currency" },
  { apiName: "percent", label: "Percent" },
  { apiName: "date", label: "Date" },
  { apiName: "datetime", label: "Date/Time" },
  { apiName: "time", label: "Time" },
  { apiName: "picklist", label: "Picklist", supportsPicklistValues: true },
  { apiName: "lookup", label: "Lookup", requiresReferenceTo: true },
  { apiName: "master_detail", label: "Master-Detail", requiresReferenceTo: true },
  { apiName: "json", label: "JSON" },
  { apiName: "autonumber", label: "Auto Number", supportsAutonumber: true },
  { apiName: "richtext", label: "Rich Text" },
  { apiName: "address", label: "Address" },
  { apiName: "geolocation", label: "Geolocation" },
];

function sortObjects(rows: MetaObject[]): MetaObject[] {
  return [...rows].sort((a, b) => {
    const la = (a.label || a.apiName).toLowerCase();
    const lb = (b.label || b.apiName).toLowerCase();
    return la.localeCompare(lb);
  });
}

const emptyFieldForm = {
  apiName: "",
  label: "",
  fieldType: "text",
  required: false,
  referenceTo: "",
  relationshipName: "",
  picklistValues: "",
  autonumberFormat: "{00000}",
  autonumberStart: "1",
};

export function ObjectManagerPanel({
  bridge,
  refreshKey = 0,
}: {
  bridge: AppBridge;
  /** Bump when active env changes to reload lists. */
  refreshKey?: number;
}) {
  const [objects, setObjects] = useState<MetaObject[]>([]);
  const [selected, setSelected] = useState<string | null>(null);
  const [detail, setDetail] = useState<DescribePayload | null>(null);
  const [err, setErr] = useState("");
  const [warn, setWarn] = useState("");
  const [busy, setBusy] = useState(false);
  const [filter, setFilter] = useState("");
  const [showNewObject, setShowNewObject] = useState(false);
  const [showNewField, setShowNewField] = useState(false);
  const [objForm, setObjForm] = useState({ apiName: "", label: "", pluralLabel: "" });
  const [fieldForm, setFieldForm] = useState({ ...emptyFieldForm });
  const [fieldTypes, setFieldTypes] = useState<FieldTypeInfo[]>(FALLBACK_FIELD_TYPES);

  const loadObjects = useCallback(async () => {
    if (!bridge.session?.token) {
      setObjects([]);
      return;
    }
    setBusy(true);
    setErr("");
    try {
      const row = (await bridge.fetch("/metadata/v1/objects")) as { objects?: MetaObject[] };
      setObjects(sortObjects(row.objects ?? []));
      try {
        const catalog = (await bridge.fetch("/metadata/v1/field-types")) as { fieldTypes?: FieldTypeInfo[] };
        if (catalog.fieldTypes?.length) setFieldTypes(catalog.fieldTypes);
      } catch {
        setFieldTypes(FALLBACK_FIELD_TYPES);
      }
    } catch (e) {
      setErr(String(e));
      setObjects([]);
    } finally {
      setBusy(false);
    }
  }, [bridge]);

  const loadDetail = useCallback(
    async (apiName: string) => {
      setBusy(true);
      setErr("");
      try {
        const desc = (await bridge.fetch(`/metadata/v1/objects/${encodeURIComponent(apiName)}`)) as DescribePayload;
        setDetail(desc);
        setSelected(apiName);
        setShowNewObject(false);
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
    setShowNewField(false);
    setWarn("");
  };

  useEffect(() => {
    void loadObjects();
    setSelected(null);
    setDetail(null);
    setWarn("");
  }, [loadObjects, refreshKey]);

  const createObject = async () => {
    setErr("");
    setWarn("");
    setBusy(true);
    try {
      const body = {
        apiName: objForm.apiName.trim(),
        label: objForm.label.trim(),
        pluralLabel: objForm.pluralLabel.trim() || `${objForm.label.trim()}s`,
        storageMode: "flexible",
        ownership: "custom",
      };
      const created = (await bridge.fetch("/metadata/v1/objects", {
        method: "POST",
        body: JSON.stringify(body),
      })) as MetaObject;
      const mirrorWarn = await mirrorObjectYaml(bridge.session?.repoPath, {
        apiName: created.apiName || body.apiName,
        label: created.label || body.label,
        pluralLabel: created.pluralLabel || body.pluralLabel,
        storageMode: created.storageMode || "flexible",
        ownership: created.ownership || "custom",
        features: created.features,
      });
      if (mirrorWarn) setWarn(mirrorWarn);
      setShowNewObject(false);
      setObjForm({ apiName: "", label: "", pluralLabel: "" });
      await loadObjects();
      await loadDetail(created.apiName || body.apiName);
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const saveObjectLabel = async () => {
    if (!detail?.apiName) return;
    setErr("");
    setWarn("");
    setBusy(true);
    try {
      const patched = (await bridge.fetch(`/metadata/v1/objects/${encodeURIComponent(detail.apiName)}`, {
        method: "PATCH",
        body: JSON.stringify({ label: detail.label, pluralLabel: detail.pluralLabel }),
      })) as MetaObject;
      const mirrorWarn = await mirrorObjectYaml(bridge.session?.repoPath, {
        apiName: patched.apiName || detail.apiName,
        label: patched.label || detail.label,
        pluralLabel: patched.pluralLabel || detail.pluralLabel,
        storageMode: patched.storageMode || detail.storageMode,
        ownership: patched.ownership || detail.ownership,
        features: patched.features || detail.features,
      });
      if (mirrorWarn) setWarn(mirrorWarn);
      await loadObjects();
      await loadDetail(detail.apiName);
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const createField = async () => {
    if (!selected) return;
    setErr("");
    setWarn("");
    setBusy(true);
    try {
      const body: Record<string, unknown> = {
        objectApiName: selected,
        apiName: fieldForm.apiName.trim(),
        label: fieldForm.label.trim(),
        fieldType: fieldForm.fieldType,
        required: fieldForm.required,
        ownership: "custom",
      };
      if (fieldForm.referenceTo.trim()) body.referenceTo = fieldForm.referenceTo.trim();
      if (fieldForm.relationshipName.trim()) body.relationshipName = fieldForm.relationshipName.trim();
      if (fieldForm.picklistValues.trim()) {
        body.picklistValues = fieldForm.picklistValues
          .split(",")
          .map((s) => s.trim())
          .filter(Boolean);
      }
      if (fieldForm.fieldType === "autonumber") {
        body.autonumberFormat = fieldForm.autonumberFormat.trim() || "{00000}";
        const start = Number(fieldForm.autonumberStart);
        body.autonumberStart = Number.isFinite(start) ? start : 1;
      }
      const created = (await bridge.fetch("/metadata/v1/fields", {
        method: "POST",
        body: JSON.stringify(body),
      })) as MetaField;
      const mirrorWarn = await mirrorFieldYaml(bridge.session?.repoPath, {
        objectApiName: selected,
        apiName: created.apiName || String(body.apiName),
        label: created.label || String(body.label),
        fieldType: created.fieldType || String(body.fieldType),
        required: created.required ?? Boolean(body.required),
        uniqueField: created.uniqueField,
        indexed: created.indexed,
        ownership: created.ownership || "custom",
        length: created.length,
        referenceTo: created.referenceTo || (body.referenceTo as string | undefined),
      });
      if (mirrorWarn) setWarn(mirrorWarn);
      setShowNewField(false);
      setFieldForm({ ...emptyFieldForm });
      await loadDetail(selected);
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const selectedTypeInfo = fieldTypes.find((t) => t.apiName === fieldForm.fieldType);

  const filtered = useMemo(() => {
    const q = filter.trim().toLowerCase();
    if (!q) return objects;
    return objects.filter((o) => {
      return (
        o.apiName.toLowerCase().includes(q) ||
        (o.label || "").toLowerCase().includes(q) ||
        (o.pluralLabel || "").toLowerCase().includes(q)
      );
    });
  }, [objects, filter]);

  if (!bridge.session?.token) {
    return (
      <ToolSurface testId="object-manager">
        <PanelHeader title="Objects" subtitle="Create and inspect objects and fields on the active environment." />
        <EmptyState
          icon={<IconMetadata size={28} />}
          title="Connect an environment"
          description="Objects reads and writes the Metadata API on the active install. Use the top-bar env switcher."
        />
      </ToolSurface>
    );
  }

  const listToolbar = (
    <ToolToolbar
      actions={
        <>
          <Button
            variant="primary"
            onClick={() => {
              setShowNewObject((v) => !v);
              if (selected) backToList();
            }}
          >
            {showNewObject ? "Cancel" : "New object"}
          </Button>
          <Button variant="secondary" busy={busy} onClick={() => void loadObjects()}>
            Refresh
          </Button>
        </>
      }
      meta={
        <span className="muted om-count" data-testid="om-count">
          {filtered.length === objects.length
            ? `${objects.length} object${objects.length === 1 ? "" : "s"}`
            : `${filtered.length} of ${objects.length}`}
        </span>
      }
      search={
        <SearchField
          value={filter}
          onChange={setFilter}
          placeholder="Search objects…"
          label="Search objects"
          testId="om-search"
        />
      }
    />
  );

  return (
    <ToolSurface className="object-manager" testId="object-manager">
      <PanelHeader
        title="Object Manager"
        subtitle="Browse, create, and describe objects on the active environment. Dual-writes one YAML when a local repo path is set."
      />
      {err && <p className="err">{err}</p>}
      {warn && <p className="muted" data-testid="mirror-warn">{warn}</p>}
      {!(selected && detail) ? listToolbar : null}

      {selected && detail ? (
        <div className="om-detail-view" data-testid="om-detail">
          <div className="om-detail-nav">
            <Button variant="ghost" onClick={backToList} data-testid="om-back">
              <IconChevronLeft size={14} aria-hidden /> All objects
            </Button>
          </div>
          <div className="om-form">
            <h3>{detail.apiName}</h3>
            <label>
              Label
              <input
                value={detail.label || ""}
                onChange={(e) => setDetail({ ...detail, label: e.target.value })}
              />
            </label>
            <label>
              Plural label
              <input
                value={detail.pluralLabel || ""}
                onChange={(e) => setDetail({ ...detail, pluralLabel: e.target.value })}
              />
            </label>
            <div className="row">
              <StatusBadge tone="neutral">{detail.ownership || "—"}</StatusBadge>
              <StatusBadge tone="neutral">{detail.storageMode || "flexible"}</StatusBadge>
              {detail.ownership === "custom" ? (
                <Button variant="secondary" busy={busy} onClick={() => void saveObjectLabel()}>
                  Save object
                </Button>
              ) : (
                <p className="muted">Managed objects are read-only here.</p>
              )}
            </div>
          </div>
          <div className="row" style={{ marginTop: "1rem" }}>
            <h4>Fields</h4>
            <Button variant="primary" onClick={() => setShowNewField((v) => !v)}>
              {showNewField ? "Cancel" : "New field"}
            </Button>
          </div>
          {showNewField ? (
            <div className="om-form" data-testid="om-new-field">
              <label>
                API name
                <input
                  value={fieldForm.apiName}
                  onChange={(e) => setFieldForm((f) => ({ ...f, apiName: e.target.value }))}
                  placeholder="Region__c"
                />
              </label>
              <label>
                Label
                <input
                  value={fieldForm.label}
                  onChange={(e) => setFieldForm((f) => ({ ...f, label: e.target.value }))}
                />
              </label>
              <label>
                Type
                <select
                  value={fieldForm.fieldType}
                  onChange={(e) => setFieldForm((f) => ({ ...f, fieldType: e.target.value }))}
                  aria-label="Field type"
                >
                  {fieldTypes.map((t) => (
                    <option key={t.apiName} value={t.apiName}>
                      {t.label} ({t.apiName})
                    </option>
                  ))}
                </select>
              </label>
              {selectedTypeInfo?.requiresReferenceTo ? (
                <>
                  <label>
                    Reference to
                    <input
                      value={fieldForm.referenceTo}
                      onChange={(e) => setFieldForm((f) => ({ ...f, referenceTo: e.target.value }))}
                      placeholder="Account"
                      aria-label="Reference to"
                    />
                  </label>
                  <label>
                    Relationship name
                    <input
                      value={fieldForm.relationshipName}
                      onChange={(e) => setFieldForm((f) => ({ ...f, relationshipName: e.target.value }))}
                      placeholder="CustomLookups"
                    />
                  </label>
                </>
              ) : null}
              {selectedTypeInfo?.supportsPicklistValues ? (
                <label>
                  Picklist values (comma-separated)
                  <input
                    value={fieldForm.picklistValues}
                    onChange={(e) => setFieldForm((f) => ({ ...f, picklistValues: e.target.value }))}
                    placeholder="Open, Closed"
                  />
                </label>
              ) : null}
              {selectedTypeInfo?.supportsAutonumber ? (
                <>
                  <label>
                    Autonumber format
                    <input
                      value={fieldForm.autonumberFormat}
                      onChange={(e) => setFieldForm((f) => ({ ...f, autonumberFormat: e.target.value }))}
                      placeholder="A-{00000}"
                    />
                  </label>
                  <label>
                    Start
                    <input
                      value={fieldForm.autonumberStart}
                      onChange={(e) => setFieldForm((f) => ({ ...f, autonumberStart: e.target.value }))}
                    />
                  </label>
                </>
              ) : null}
              <label className="row">
                <input
                  type="checkbox"
                  checked={fieldForm.required}
                  onChange={(e) => setFieldForm((f) => ({ ...f, required: e.target.checked }))}
                />
                Required
              </label>
              <Button
                variant="primary"
                busy={busy}
                disabled={
                  !fieldForm.apiName.trim() ||
                  !fieldForm.label.trim() ||
                  (Boolean(selectedTypeInfo?.requiresReferenceTo) && !fieldForm.referenceTo.trim())
                }
                onClick={() => void createField()}
              >
                Create field
              </Button>
            </div>
          ) : null}
          <div className="data-table-wrap om-table-wrap">
            <table className="data-table" data-testid="om-field-table">
              <thead>
                <tr>
                  <th>API name</th>
                  <th>Label</th>
                  <th>Type</th>
                  <th>Required</th>
                  <th>Ownership</th>
                </tr>
              </thead>
              <tbody>
                {(detail.fields ?? []).map((f) => (
                  <tr key={f.apiName}>
                    <td className="mono">{f.apiName}</td>
                    <td>{f.label}</td>
                    <td>{f.fieldType}</td>
                    <td>{f.required ? "yes" : ""}</td>
                    <td>{f.ownership || "—"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      ) : (
        <div className="om-list-view" data-testid="om-list-view">
          {showNewObject ? (
            <div className="om-form om-new-object-form" data-testid="om-new-object">
              <label>
                Label
                <input
                  value={objForm.label}
                  onChange={(e) => setObjForm((f) => ({ ...f, label: e.target.value }))}
                  placeholder="Order"
                />
              </label>
              <label>
                Plural label
                <input
                  value={objForm.pluralLabel}
                  onChange={(e) => setObjForm((f) => ({ ...f, pluralLabel: e.target.value }))}
                  placeholder="Orders"
                />
              </label>
              <label>
                API name
                <input
                  value={objForm.apiName}
                  onChange={(e) => setObjForm((f) => ({ ...f, apiName: e.target.value }))}
                  placeholder="MyObject__c"
                />
              </label>
              <div className="om-new-object-actions" data-testid="om-new-object-actions">
                <Button
                  variant="primary"
                  busy={busy}
                  disabled={!objForm.apiName.trim() || !objForm.label.trim()}
                  onClick={() => void createObject()}
                >
                  Create object
                </Button>
              </div>
            </div>
          ) : null}
          <div className="data-table-wrap om-table-wrap" data-testid="om-object-list">
            <table className="data-table om-object-table">
              <thead>
                <tr>
                  <th>Label</th>
                  <th>Plural label</th>
                  <th>API name</th>
                  <th>Ownership</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((o) => (
                  <tr
                    key={o.apiName}
                    className="om-object-row"
                    data-testid={`om-row-${o.apiName}`}
                    onClick={() => void loadDetail(o.apiName)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" || e.key === " ") {
                        e.preventDefault();
                        void loadDetail(o.apiName);
                      }
                    }}
                    tabIndex={0}
                    role="button"
                    aria-label={`Open ${o.label || o.apiName}`}
                  >
                    <td>{o.label || o.apiName}</td>
                    <td>{o.pluralLabel || "—"}</td>
                    <td className="mono">{o.apiName}</td>
                    <td>
                      {o.ownership ? (
                        <StatusBadge tone={o.ownership === "custom" ? "accent" : "neutral"}>
                          {o.ownership}
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
              <p className="data-table-empty muted">No objects match.</p>
            ) : null}
          </div>
        </div>
      )}
    </ToolSurface>
  );
}
