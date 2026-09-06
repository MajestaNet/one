import { describe, expect, it } from "vitest";
import { parseUserDataDirFlag, shouldTakeSingleInstanceLock } from "./userDataDir";

describe("userDataDir argv", () => {
  it("parses equals and split --user-data-dir", () => {
    expect(parseUserDataDirFlag(["electron", "--user-data-dir=/tmp/sim-a"])).toBe("/tmp/sim-a");
    expect(parseUserDataDirFlag(["electron", "--user-data-dir", "/tmp/sim-b"])).toBe("/tmp/sim-b");
  });

  it("keeps the default-profile single-instance lock", () => {
    expect(shouldTakeSingleInstanceLock(["electron", "."])).toBe(true);
    expect(shouldTakeSingleInstanceLock(["electron", "--user-data-dir=/tmp/sim-a"])).toBe(false);
  });
});
