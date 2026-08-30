import { describe, expect, it } from "vitest";
import {
  IDE_COMPAT_MANIFEST,
  evaluateProductTestedAgainst,
  evaluateRevision,
  mergeCompatStatus,
  parseApiRevisionWindow,
  selectPin,
} from "./compat";

describe("compat", () => {
  const window = { min: 12, current: 14 };

  it("parses api revision window", () => {
    expect(parseApiRevisionWindow({ min: 12, current: 14 })).toEqual(window);
    expect(parseApiRevisionWindow({ min: 15, current: 14 })).toBeNull();
  });

  it("selectPin caps preferred to install current", () => {
    const manifest = { ...IDE_COMPAT_MANIFEST, minApiRevision: 12, preferredApiRevision: 20 };
    const result = selectPin(manifest, window);
    expect(result).toEqual({ pin: 14 });
  });

  it("selectPin blocks when install too old for IDE", () => {
    const manifest = { ...IDE_COMPAT_MANIFEST, minApiRevision: 15, preferredApiRevision: 15 };
    const result = selectPin(manifest, window);
    expect(result).toMatchObject({ block: true, code: "INSTALL_REVISION_TOO_OLD" });
  });

  it("evaluateRevision blocks out-of-window pin", () => {
    expect(evaluateRevision(11, window).status).toBe("block");
    expect(evaluateRevision(12, window).status).toBe("ok");
  });

  it("product tested-against is warn-only", () => {
    const manifest = {
      ...IDE_COMPAT_MANIFEST,
      targetProductVersion: "0.4.2",
      supportedProductMinors: 2,
    };
    expect(evaluateProductTestedAgainst(manifest, "0.4.1").status).toBe("ok");
    expect(evaluateProductTestedAgainst(manifest, "0.3.9").status).toBe("ok");
    expect(evaluateProductTestedAgainst(manifest, "0.2.0").status).toBe("warn");
    expect(evaluateProductTestedAgainst(manifest, "1.0.0").status).toBe("warn");
  });

  it("mergeCompatStatus keeps revision block over product warn", () => {
    const merged = mergeCompatStatus(
      { status: "block", code: "API_REVISION_UNSUPPORTED" },
      { status: "warn", code: "PRODUCT_OUTSIDE_TESTED" },
    );
    expect(merged.status).toBe("block");
  });
});
