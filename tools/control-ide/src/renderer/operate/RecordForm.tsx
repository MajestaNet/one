import type { DescribeField } from "./types";
import { SYSTEM_FIELD_SKIP } from "./types";

function isEditableField(f: DescribeField): boolean {
  if (!f.apiName || SYSTEM_FIELD_SKIP.has(f.apiName)) return false;
  return true;
}

export function editableFields(fields: DescribeField[]): DescribeField[] {
  return fields.filter(isEditableField);
}

export function RecordForm({
  fields,
  values,
  onChange,
  mode,
}: {
  fields: DescribeField[];
  values: Record<string, unknown>;
  onChange: (next: Record<string, unknown>) => void;
  mode: "create" | "edit";
}) {
  const visible = editableFields(fields);
  if (visible.length === 0) {
    return <p className="muted">No editable fields from describe.</p>;
  }

  return (
    <div className="crm-record-form" data-testid="crm-record-form">
      {visible.map((f) => {
        const key = f.apiName;
        const val = values[key] ?? "";
        const label = f.label ?? key;
        const required = Boolean(f.required) && mode === "create";
        const type = f.fieldType ?? "text";

        if (type === "picklist" && f.picklistValues?.length) {
          return (
            <label key={key} className="composer-label">
              {label}
              {required ? " *" : ""}
              <select
                value={String(val)}
                aria-label={label}
                onChange={(e) => onChange({ ...values, [key]: e.target.value || null })}
              >
                <option value="">—</option>
                {f.picklistValues.map((opt) => (
                  <option key={opt} value={opt}>
                    {opt}
                  </option>
                ))}
              </select>
            </label>
          );
        }

        if (type === "boolean" || type === "checkbox") {
          return (
            <label key={key} className="composer-label crm-checkbox">
              <input
                type="checkbox"
                checked={Boolean(val)}
                aria-label={label}
                onChange={(e) => onChange({ ...values, [key]: e.target.checked })}
              />
              {label}
            </label>
          );
        }

        if (type === "autonumber") {
          return (
            <label key={key} className="composer-label">
              {label}
              <input type="text" value={String(val)} aria-label={label} readOnly disabled />
            </label>
          );
        }

        if (type === "richtext" || type === "textarea" || type === "longtext") {
          return (
            <label key={key} className="composer-label">
              {label}
              {required ? " *" : ""}
              <textarea
                rows={type === "richtext" ? 5 : 3}
                value={String(val)}
                aria-label={label}
                onChange={(e) => onChange({ ...values, [key]: e.target.value })}
              />
            </label>
          );
        }

        if (type === "json" || type === "address" || type === "geolocation") {
          const text = typeof val === "string" ? val : val == null || val === "" ? "" : JSON.stringify(val);
          return (
            <label key={key} className="composer-label">
              {label}
              {required ? " *" : ""}
              <textarea
                rows={3}
                value={text}
                aria-label={label}
                placeholder={type === "geolocation" ? '{"latitude":0,"longitude":0}' : type === "address" ? '{"street":"","city":"","state":"","postalCode":"","country":""}' : "{}"}
                onChange={(e) => {
                  const raw = e.target.value.trim();
                  if (!raw) {
                    onChange({ ...values, [key]: null });
                    return;
                  }
                  try {
                    onChange({ ...values, [key]: JSON.parse(raw) });
                  } catch {
                    onChange({ ...values, [key]: raw });
                  }
                }}
              />
            </label>
          );
        }

        const inputType =
          type === "number" ||
          type === "currency" ||
          type === "percent" ||
          type === "integer" ||
          type === "double" ||
          type === "int"
            ? "number"
            : type === "email"
              ? "email"
              : type === "url"
                ? "url"
                : type === "time"
                  ? "time"
                  : type === "date" || type === "datetime"
                    ? type === "date"
                      ? "date"
                      : "datetime-local"
                    : "text";

        return (
          <label key={key} className="composer-label">
            {label}
            {required ? " *" : ""}
            <input
              type={inputType}
              value={val == null ? "" : String(val)}
              aria-label={label}
              onChange={(e) => {
                const raw = e.target.value;
                if (inputType === "number") {
                  onChange({ ...values, [key]: raw === "" ? null : Number(raw) });
                } else {
                  onChange({ ...values, [key]: raw });
                }
              }}
            />
          </label>
        );
      })}
    </div>
  );
}

export function requiredMissing(fields: DescribeField[], values: Record<string, unknown>): string[] {
  return editableFields(fields)
    .filter((f) => f.required)
    .filter((f) => {
      const v = values[f.apiName];
      return v == null || v === "";
    })
    .map((f) => f.apiName);
}
