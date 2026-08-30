import { useCallback, useEffect, useRef, useState } from "react";
import type { AppBridge } from "../App";
import { createInferenceTestChat } from "../agents/runs";
import { EmptyState, PanelHeader, ToolSurface } from "../ui";
import { IconSettings } from "../icons/Icons";

type InferenceStatus = {
  activeSource?: string;
  doEnabled?: boolean;
  doMode?: string | null;
  doModelId?: string | null;
  doTokenConfigured?: boolean;
  doModeModels?: Record<string, string>;
  billingNotice?: string;
  prepaid?: boolean;
  defaultProviderApiName?: string;
  providers?: ProviderRow[];
};

type ProviderRow = {
  apiName: string;
  label: string;
  baseUrl: string;
  defaultModel: string;
  active: boolean;
  hasSecret?: boolean;
};

const PRESETS: { id: string; label: string; baseUrl: string; defaultModel?: string; apiKeyHint?: string }[] = [
  { id: "openai", label: "OpenAI", baseUrl: "https://api.openai.com/v1", defaultModel: "gpt-4o-mini" },
  { id: "openrouter", label: "OpenRouter", baseUrl: "https://openrouter.ai/api/v1" },
  { id: "groq", label: "Groq", baseUrl: "https://api.groq.com/openai/v1" },
  {
    id: "ollama",
    label: "Ollama (local)",
    baseUrl: "http://127.0.0.1:11434/v1",
    defaultModel: "llama3.2",
    apiKeyHint: "ollama",
  },
  { id: "custom", label: "Custom (OpenAI-compatible)", baseUrl: "" },
];

/**
 * Settings → Inference: BYO OpenAI-compatible providers + Native DigitalOcean Inference (BP-052).
 */
export function InferencePanel({ bridge }: { bridge: AppBridge }) {
  const [status, setStatus] = useState<InferenceStatus | null>(null);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const [mode, setMode] = useState<"dev" | "standard" | "pro">("dev");
  const [preset, setPreset] = useState(PRESETS[0].id);
  const [apiName, setApiName] = useState("openai");
  const [label, setLabel] = useState("OpenAI");
  const [baseUrl, setBaseUrl] = useState(PRESETS[0].baseUrl);
  const [apiKey, setApiKey] = useState("");
  const [defaultModel, setDefaultModel] = useState("gpt-4o-mini");
  const [testPrompt, setTestPrompt] = useState("Say hello in one short sentence.");
  const [testReply, setTestReply] = useState("");
  const [testErr, setTestErr] = useState("");
  const [testBusy, setTestBusy] = useState(false);
  const testAbort = useRef<AbortController | null>(null);
  const connected = Boolean(bridge.session?.baseUrl && bridge.session?.token);

  const load = useCallback(async () => {
    if (!connected) {
      setStatus(null);
      return;
    }
    setErr("");
    setBusy(true);
    try {
      const row = (await bridge.fetch("/metadata/v1/inference/config")) as InferenceStatus;
      setStatus(row);
      if (row.doMode === "dev" || row.doMode === "standard" || row.doMode === "pro") {
        setMode(row.doMode);
      }
    } catch (e) {
      setErr(String(e));
      setStatus(null);
    } finally {
      setBusy(false);
    }
  }, [bridge, connected]);

  useEffect(() => {
    if (connected) void load();
  }, [connected, load, bridge.session?.activeInstallId]);

  useEffect(() => () => testAbort.current?.abort(), []);

  const putDO = async (enabled: boolean) => {
    setBusy(true);
    setErr("");
    try {
      const row = (await bridge.fetch("/deploy/v1/cloud/inference", {
        method: "PUT",
        body: JSON.stringify({ enabled, mode }),
      })) as InferenceStatus;
      setStatus(row);
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const saveBYO = async () => {
    setBusy(true);
    setErr("");
    try {
      await bridge.fetch("/metadata/v1/inference/providers", {
        method: "POST",
        body: JSON.stringify({
          apiName,
          label,
          baseUrl,
          apiKey: apiKey || undefined,
          defaultModel,
          setDefault: true,
        }),
      });
      setApiKey("");
      await load();
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const sendTestChat = async () => {
    const session = bridge.session;
    if (!session?.baseUrl || !session.token) return;
    testAbort.current?.abort();
    const ac = new AbortController();
    testAbort.current = ac;
    setTestBusy(true);
    setTestErr("");
    setTestReply("");
    try {
      let streamed = "";
      await createInferenceTestChat(session.baseUrl, session.token, testPrompt, {
        onToken: ({ delta }) => {
          if (!delta) return;
          streamed += delta;
          setTestReply(streamed);
        },
        onDone: ({ output }) => {
          if (streamed) return;
          if (output && typeof output === "object" && "summary" in output) {
            setTestReply(String((output as { summary?: string }).summary ?? ""));
          }
        },
      }, ac.signal);
      if (!streamed) {
        setTestReply("(model returned no tokens — check the provider URL and model id)");
      }
    } catch (e) {
      if (ac.signal.aborted) return;
      setTestErr(String(e));
    } finally {
      if (testAbort.current === ac) {
        setTestBusy(false);
      }
    }
  };

  const onPreset = (id: string) => {
    setPreset(id);
    const p = PRESETS.find((x) => x.id === id);
    if (!p) return;
    if (id !== "custom") {
      setApiName(id);
      setLabel(p.label);
      setBaseUrl(p.baseUrl);
      if (p.defaultModel) setDefaultModel(p.defaultModel);
      if (p.apiKeyHint) setApiKey(p.apiKeyHint);
    }
  };

  if (!connected) {
    return (
      <ToolSurface testId="inference-panel">
        <PanelHeader title="Inference" subtitle="Connect an install to configure model providers." />
        <EmptyState icon={<IconSettings />} title="Not connected" description="Sign in to an install to manage inference." />
      </ToolSurface>
    );
  }

  const models = status?.doModeModels ?? {};

  return (
    <ToolSurface testId="inference-panel">
      <PanelHeader
        title="Inference"
        subtitle="Route agent runs through BYO providers or Native DigitalOcean Inference."
      />
      {err ? (
        <p className="error-text" role="alert">
          {err}
        </p>
      ) : null}
      {busy && !status ? <p className="muted">Loading…</p> : null}

      <section className="stack-gap" style={{ marginBottom: "1.5rem" }}>
        <h3 className="panel-section-title">Active source</h3>
        <p className="muted">
          Current: <strong>{status?.activeSource ?? "none"}</strong>
          {status?.doModelId ? <> · model <code>{status.doModelId}</code></> : null}
          {status?.defaultProviderApiName ? <> · BYO <code>{status.defaultProviderApiName}</code></> : null}
        </p>
      </section>

      <section className="stack-gap" style={{ marginBottom: "1.5rem" }} data-testid="inference-test-chat">
        <h3 className="panel-section-title">Test chat</h3>
        <p className="muted">
          Sends one streaming prompt through <code>POST /client/v1/agents/runs</code> (same path as Operate). Use this to
          confirm the model before chatting. Operate still sends <code>approved: false</code>; this probe sets{" "}
          <code>approved: true</code> so older APIs generate instead of parking.
        </p>
        <label className="field-label">
          Prompt
          <textarea
            value={testPrompt}
            onChange={(e) => setTestPrompt(e.target.value)}
            rows={2}
            data-testid="inference-test-prompt"
          />
        </label>
        <div className="row-actions">
          <button
            type="button"
            disabled={testBusy || !testPrompt.trim()}
            onClick={() => void sendTestChat()}
            data-testid="inference-test-send"
          >
            {testBusy ? "Streaming…" : "Send test"}
          </button>
        </div>
        {testErr ? (
          <p className="error-text" role="alert" data-testid="inference-test-error">
            {testErr}
          </p>
        ) : null}
        <pre className="inference-test-transcript" data-testid="inference-test-reply">
          {testBusy && !testReply ? "Waiting for tokens…" : testReply}
        </pre>
      </section>

      <section className="stack-gap" style={{ marginBottom: "1.5rem" }} data-testid="inference-do-section">
        <h3 className="panel-section-title">Native DigitalOcean</h3>
        <p className="muted" data-testid="inference-billing-notice">
          {status?.billingNotice ??
            "Inference and hosting are billed by DigitalOcean on your account. Serverless Inference is prepaid."}
        </p>
        {!status?.doTokenConfigured ? (
          <p className="muted">Install needs <code>DIGITALOCEAN_API_TOKEN</code> before Native DO Inference can be enabled.</p>
        ) : null}
        <label className="field-label">
          Mode
          <select
            value={mode}
            onChange={(e) => setMode(e.target.value as "dev" | "standard" | "pro")}
            data-testid="inference-do-mode"
          >
            <option value="dev">Dev{models.dev ? ` (${models.dev})` : ""}</option>
            <option value="standard">Standard{models.standard ? ` (${models.standard})` : ""}</option>
            <option value="pro">Pro{models.pro ? ` (${models.pro})` : ""}</option>
          </select>
        </label>
        <div className="row-actions">
          <button type="button" disabled={busy || !status?.doTokenConfigured} onClick={() => void putDO(true)} data-testid="inference-do-enable">
            Enable Native DO
          </button>
          <button type="button" disabled={busy || !status?.doEnabled} onClick={() => void putDO(false)} data-testid="inference-do-disable">
            Disable
          </button>
        </div>
      </section>

      <section className="stack-gap" style={{ marginBottom: "1.5rem" }} data-testid="inference-byo-section">
        <h3 className="panel-section-title">BYO provider</h3>
        <p className="muted">
          OpenAI-compatible base URL + API key. Keys are stored as install secrets (never echoed back). Cloud BYO hosts
          must also be on the install egress allowlist. Local Ollama (<code>http://127.0.0.1:11434/v1</code>) works when
          the API runs with <code>APP_ENV=development</code> — use a placeholder API key (Ollama ignores it).
        </p>
        <label className="field-label">
          Preset
          <select value={preset} onChange={(e) => onPreset(e.target.value)} data-testid="inference-preset">
            {PRESETS.map((p) => (
              <option key={p.id} value={p.id}>
                {p.label}
              </option>
            ))}
          </select>
        </label>
        <label className="field-label">
          API name
          <input value={apiName} onChange={(e) => setApiName(e.target.value)} data-testid="inference-api-name" />
        </label>
        <label className="field-label">
          Label
          <input value={label} onChange={(e) => setLabel(e.target.value)} />
        </label>
        <label className="field-label">
          Base URL
          <input value={baseUrl} onChange={(e) => setBaseUrl(e.target.value)} data-testid="inference-base-url" />
        </label>
        <label className="field-label">
          API key
          <input
            type="password"
            value={apiKey}
            onChange={(e) => setApiKey(e.target.value)}
            placeholder="sk-…"
            autoComplete="off"
            data-testid="inference-api-key"
          />
        </label>
        <label className="field-label">
          Default model
          <input value={defaultModel} onChange={(e) => setDefaultModel(e.target.value)} data-testid="inference-default-model" />
        </label>
        <button type="button" disabled={busy || !apiName || !baseUrl} onClick={() => void saveBYO()} data-testid="inference-save-byo">
          Save &amp; use as active BYO
        </button>
      </section>

      {(status?.providers?.length ?? 0) > 0 ? (
        <section className="stack-gap">
          <h3 className="panel-section-title">Configured providers</h3>
          <ul className="plain-list">
            {status!.providers!.map((p) => (
              <li key={p.apiName}>
                <strong>{p.label}</strong> (<code>{p.apiName}</code>) · {p.baseUrl} · model <code>{p.defaultModel || "—"}</code>
                {p.hasSecret ? " · key set" : " · no key"}
              </li>
            ))}
          </ul>
        </section>
      ) : null}
    </ToolSurface>
  );
}
