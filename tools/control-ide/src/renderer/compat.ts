/** Control IDE client compat manifest + handshake helpers (ADR-025 / BP-025). */

export type ApiRevisionWindow = {
  min: number;
  current: number;
};

export type HttpApiFamilies = {
  client: string;
  metadata: string;
  deploy: string;
  ops: string;
  auth: string;
};

export type IdeCompatManifest = {
  ideVersion: string;
  preferredApiRevision: number;
  minApiRevision: number;
  targetProductVersion: string;
  supportedProductMinors: number;
  httpApi: HttpApiFamilies;
};

export type CompatStatus = "ok" | "warn" | "block" | "overridden";

export type CompatEvaluation = {
  status: CompatStatus;
  code?: string;
  message?: string;
};

export const IDE_COMPAT_MANIFEST: IdeCompatManifest = {
  ideVersion: "0.1.0",
  preferredApiRevision: 1,
  minApiRevision: 1,
  targetProductVersion: "0.1.0",
  supportedProductMinors: 2,
  httpApi: {
    client: "v1",
    metadata: "v1",
    deploy: "v1",
    ops: "v1",
    auth: "v1",
  },
};

const semverPrefixRe = /^(\d+)\.(\d+)\.(\d+)/;

function parseSemverPrefix(version: string): { major: number; minor: number } | null {
  const m = semverPrefixRe.exec(version.trim());
  if (!m) return null;
  return { major: Number(m[1]), minor: Number(m[2]) };
}

export function parseApiRevisionWindow(raw: unknown): ApiRevisionWindow | null {
  if (!raw || typeof raw !== "object") return null;
  const o = raw as Record<string, unknown>;
  const min = Number(o.min);
  const current = Number(o.current);
  if (!Number.isFinite(min) || !Number.isFinite(current) || min < 1 || current < 1 || min > current) {
    return null;
  }
  return { min, current };
}

export function parseVersionProbe(body: Record<string, unknown>): {
  productVersion?: string;
  apiRevision: ApiRevisionWindow | null;
} {
  const productVersion =
    typeof body.productVersion === "string" ? body.productVersion : undefined;
  const apiRevision = parseApiRevisionWindow(body.apiRevision);
  return { productVersion, apiRevision };
}

export function selectPin(
  manifest: IdeCompatManifest,
  window: ApiRevisionWindow,
): { pin: number } | { block: true; code: string; message: string } {
  const { minApiRevision, preferredApiRevision } = manifest;
  if (minApiRevision < 1 || preferredApiRevision < 1) {
    return {
      block: true,
      code: "UNPARSEABLE_REVISION",
      message: "Control IDE revision manifest is invalid",
    };
  }
  if (minApiRevision > window.current) {
    return {
      block: true,
      code: "INSTALL_REVISION_TOO_OLD",
      message: `Install API revision ${window.current} is below IDE minimum ${minApiRevision}. Upgrade the install product image.`,
    };
  }
  let preferred = preferredApiRevision;
  if (preferred > window.current) preferred = window.current;
  if (preferred < window.min) {
    return {
      block: true,
      code: "API_REVISION_UNSUPPORTED",
      message: `Install requires API revision ≥ ${window.min}; IDE preferred ${preferredApiRevision}.`,
    };
  }
  let pin = preferred;
  if (pin < window.min) pin = window.min;
  if (pin < minApiRevision) pin = minApiRevision;
  if (pin < window.min || pin > window.current) {
    return {
      block: true,
      code: "API_REVISION_UNSUPPORTED",
      message: `Pin ${pin} is outside install window [${window.min}, ${window.current}].`,
    };
  }
  return { pin };
}

export function evaluateRevision(
  pin: number,
  window: ApiRevisionWindow,
  overridden = false,
): CompatEvaluation {
  if (overridden) return { status: "overridden" };
  if (pin < window.min || pin > window.current) {
    return {
      status: "block",
      code: "API_REVISION_UNSUPPORTED",
      message: `API revision pin ${pin} is outside install window [${window.min}, ${window.current}].`,
    };
  }
  return { status: "ok" };
}

export function evaluateProductTestedAgainst(
  manifest: IdeCompatManifest,
  installProduct: string | undefined,
  loopback = false,
): CompatEvaluation {
  if (!installProduct?.trim()) {
    return loopback
      ? { status: "warn", code: "UNPARSEABLE_PRODUCT", message: "Install product version missing" }
      : { status: "warn", code: "UNPARSEABLE_PRODUCT", message: "Install product version missing" };
  }
  const install = parseSemverPrefix(installProduct);
  const target = parseSemverPrefix(manifest.targetProductVersion);
  if (!install || !target) {
    return {
      status: "warn",
      code: "UNPARSEABLE_PRODUCT",
      message: "Could not parse product version for tested-against check",
    };
  }
  const n = manifest.supportedProductMinors > 0 ? manifest.supportedProductMinors : 2;
  if (install.major !== target.major) {
    return {
      status: "warn",
      code: "PRODUCT_OUTSIDE_TESTED",
      message: `Install product ${installProduct} is outside the tested major for this IDE.`,
    };
  }
  const lower = target.minor - (n - 1);
  if (install.minor < lower || install.minor > target.minor) {
    return {
      status: "warn",
      code: "PRODUCT_OUTSIDE_TESTED",
      message: `Install product ${installProduct} is outside the IDE tested window (target ${manifest.targetProductVersion}, N=${n}).`,
    };
  }
  return { status: "ok" };
}

export function pinIntersection(
  manifest: IdeCompatManifest,
  window: ApiRevisionWindow,
): { minPin: number; maxPin: number } | null {
  const minPin = Math.max(manifest.minApiRevision, window.min);
  const maxPin = Math.min(manifest.preferredApiRevision, window.current);
  if (minPin > maxPin) return null;
  return { minPin, maxPin };
}

export function mergeCompatStatus(
  revision: CompatEvaluation,
  product: CompatEvaluation,
): CompatEvaluation {
  if (revision.status === "block") return revision;
  if (revision.status === "overridden") return revision;
  if (product.status === "warn") return product;
  return revision;
}

export function compatCta(code: string | undefined): string {
  switch (code) {
    case "API_REVISION_UNSUPPORTED":
      return "Migrate your API revision pin or update Control IDE / upgrade the install image.";
    case "INSTALL_REVISION_TOO_OLD":
      return "Upgrade the install product image (Ops) or use an older Control IDE build.";
    case "PRODUCT_OUTSIDE_TESTED":
      return "Product version skew is a soft warning — upgrade install or IDE when convenient.";
    case "UNPARSEABLE_REVISION":
      return "Fix API_REVISION_CURRENT / API_REVISION_MIN on the install.";
    case "UNPARSEABLE_PRODUCT":
      return "Fix PRODUCT_VERSION on the install.";
    default:
      return "";
  }
}
