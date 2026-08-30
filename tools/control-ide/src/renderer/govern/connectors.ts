/** Thin Metadata/Auth helpers for Govern outbound connector administration. */
import type { ApiFetch } from "./integrations";

export type ConnectorAuthType = "static_bearer" | "oauth2_client_credentials" | "oauth2_authorization_code";

export type ConnectorOAuthFlow = {
  authorizationUrl?: string;
  tokenUrl?: string;
  clientId?: string;
  scopes?: string[];
  pkce?: boolean;
  authStyle?: string;
};

export type OutboundConnector = {
  apiName: string;
  label: string;
  baseUrl: string;
  secretRef?: string | null;
  allowedMethods?: string[];
  pathPrefix?: string;
  active?: boolean;
  authType: ConnectorAuthType;
  oauthFlow?: ConnectorOAuthFlow;
};

export type ConnectorStatus = {
  apiName: string;
  authType: ConnectorAuthType;
  connected: boolean;
  refreshable?: boolean;
  expiresAt?: string;
};

export type InstallSecret = { apiName: string; label?: string; hasSecret?: boolean };
export type EgressEntry = { id?: string; hostPattern: string; label?: string };

export async function listOutboundConnectors(fetchApi: ApiFetch): Promise<OutboundConnector[]> {
  const res = (await fetchApi("/metadata/v1/connectors")) as { connectors?: OutboundConnector[] };
  return res.connectors ?? [];
}

export async function createOutboundConnector(
  fetchApi: ApiFetch,
  input: OutboundConnector,
): Promise<OutboundConnector> {
  return (await fetchApi("/metadata/v1/connectors", { method: "POST", body: JSON.stringify(input) })) as OutboundConnector;
}

export async function deleteOutboundConnector(fetchApi: ApiFetch, apiName: string): Promise<void> {
  await fetchApi(`/metadata/v1/connectors/${encodeURIComponent(apiName)}`, { method: "DELETE" });
}

async function deleteInstallSecret(fetchApi: ApiFetch, apiName: string): Promise<void> {
  await fetchApi(`/metadata/v1/secrets/${encodeURIComponent(apiName)}`, { method: "DELETE" });
}

export async function getConnectorStatus(fetchApi: ApiFetch, apiName: string): Promise<ConnectorStatus> {
  return (await fetchApi(`/metadata/v1/connectors/${encodeURIComponent(apiName)}/oauth/status`)) as ConnectorStatus;
}

export async function startConnectorAuthorization(fetchApi: ApiFetch, apiName: string): Promise<string> {
  const res = (await fetchApi(`/auth/v1/connectors/${encodeURIComponent(apiName)}/authorize`, {
    method: "POST",
    body: "{}",
  })) as { authorizationUrl?: string };
  if (!res.authorizationUrl) throw new Error("Authorization URL was not returned");
  return res.authorizationUrl;
}

export async function listInstallSecrets(fetchApi: ApiFetch): Promise<InstallSecret[]> {
  const res = (await fetchApi("/metadata/v1/secrets")) as { secrets?: InstallSecret[] };
  return res.secrets ?? [];
}

export async function upsertInstallSecret(
  fetchApi: ApiFetch,
  input: { apiName: string; label: string; secret: string },
  exists = false,
): Promise<void> {
  await fetchApi(
    exists
      ? `/metadata/v1/secrets/${encodeURIComponent(input.apiName)}/rotate`
      : "/metadata/v1/secrets",
    { method: "POST", body: JSON.stringify(input) },
  );
}

export async function listEgressEntries(fetchApi: ApiFetch): Promise<EgressEntry[]> {
  const res = (await fetchApi("/metadata/v1/install/egress")) as { allowlist?: EgressEntry[] };
  return res.allowlist ?? [];
}

export async function addEgressEntry(fetchApi: ApiFetch, hostPattern: string, label: string): Promise<void> {
  await fetchApi("/metadata/v1/install/egress", {
    method: "POST",
    body: JSON.stringify({ hostPattern, label }),
  });
}

async function deleteEgressEntry(fetchApi: ApiFetch, hostPattern: string): Promise<void> {
  await fetchApi(`/metadata/v1/install/egress/${encodeURIComponent(hostPattern)}`, { method: "DELETE" });
}

export type ConnectorCatalogEntry = {
  id: string;
  label: string;
  description: string;
  baseUrl: string;
  pathPrefix: string;
  authType: ConnectorAuthType;
  allowedMethods: string[];
  oauthFlow?: Omit<ConnectorOAuthFlow, "clientId">;
};

export const CONNECTOR_CATALOG: ConnectorCatalogEntry[] = [
  {
    id: "slack",
    label: "Slack API",
    description: "Send approved messages and call Slack Web API methods with an install-scoped token.",
    baseUrl: "https://slack.com/api",
    pathPrefix: "/api",
    authType: "static_bearer",
    allowedMethods: ["POST"],
  },
  {
    id: "microsoft-graph",
    label: "Microsoft Graph",
    description: "Authorization-code connector for Microsoft 365 APIs with PKCE and refresh tokens.",
    baseUrl: "https://graph.microsoft.com/v1.0",
    pathPrefix: "/v1.0",
    authType: "oauth2_authorization_code",
    allowedMethods: ["GET", "POST", "PATCH"],
    oauthFlow: {
      authorizationUrl: "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
      tokenUrl: "https://login.microsoftonline.com/common/oauth2/v2.0/token",
      scopes: ["offline_access"],
      pkce: true,
      authStyle: "auto",
    },
  },
  {
    id: "generic-rest",
    label: "Generic REST API",
    description: "A constrained HTTPS connector for an internal or vendor API using a static bearer secret.",
    baseUrl: "https://api.example.com",
    pathPrefix: "/",
    authType: "static_bearer",
    allowedMethods: ["GET", "POST"],
  },
];

export async function installCatalogConnector(
  fetchApi: ApiFetch,
  input: {
    catalog: ConnectorCatalogEntry;
    apiName: string;
    label: string;
    baseUrl: string;
    clientId?: string;
    secret?: string;
    scopes?: string[];
  },
): Promise<OutboundConnector> {
  const secretRef = input.secret ? `connector.${input.apiName}` : undefined;
  const urls = [input.baseUrl, input.catalog.oauthFlow?.authorizationUrl, input.catalog.oauthFlow?.tokenUrl].filter(
    (value): value is string => Boolean(value),
  );
  const hosts = [...new Set(urls.map((value) => {
    const parsed = new URL(value);
    if (parsed.protocol !== "https:") throw new Error(`Connector URL must use HTTPS: ${value}`);
    return parsed.hostname;
  }))];
  const [connectors, secrets, egress] = await Promise.all([
    listOutboundConnectors(fetchApi),
    listInstallSecrets(fetchApi),
    listEgressEntries(fetchApi),
  ]);
  if (connectors.some((item) => item.apiName === input.apiName)) {
    throw new Error(`Connector ${input.apiName} already exists`);
  }

  const connectorInput: OutboundConnector = {
    apiName: input.apiName,
    label: input.label,
    baseUrl: input.baseUrl,
    secretRef,
    allowedMethods: input.catalog.allowedMethods,
    pathPrefix: input.catalog.pathPrefix,
    active: true,
    authType: input.catalog.authType,
    oauthFlow: input.catalog.oauthFlow
      ? { ...input.catalog.oauthFlow, clientId: input.clientId, scopes: input.scopes?.length ? input.scopes : input.catalog.oauthFlow.scopes }
      : undefined,
  };

  const created = await createOutboundConnector(fetchApi, connectorInput);
  const addedHosts: string[] = [];
  let createdSecret = false;
  try {
    for (const host of hosts) {
      if (!egress.some((entry) => entry.hostPattern === host)) {
        await addEgressEntry(fetchApi, host, `${input.label} connector`);
        addedHosts.push(host);
      }
    }
    if (input.secret && secretRef) {
      const secretExists = secrets.some((item) => item.apiName === secretRef);
      await upsertInstallSecret(
        fetchApi,
        { apiName: secretRef, label: `${input.label} credential`, secret: input.secret },
        secretExists,
      );
      createdSecret = !secretExists;
    }
    return created;
  } catch (error) {
    await Promise.allSettled([
      deleteOutboundConnector(fetchApi, input.apiName),
      ...addedHosts.map((host) => deleteEgressEntry(fetchApi, host)),
      ...(createdSecret && secretRef ? [deleteInstallSecret(fetchApi, secretRef)] : []),
    ]);
    throw error;
  }
}
