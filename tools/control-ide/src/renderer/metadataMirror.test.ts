import { afterEach, describe, expect, it, vi } from "vitest";
import {
  fieldToYaml,
  fieldYamlPath,
  mirrorFieldYaml,
  mirrorObjectYaml,
  objectToYaml,
  objectYamlPath,
  playbookToYaml,
  playbookYamlPath,
  toolSpecToYaml,
  toolSpecYamlPath,
  mirrorToolSpecYaml,
} from "./metadataMirror";

afterEach(() => {
  Reflect.deleteProperty(window, "one");
  vi.restoreAllMocks();
});

describe("metadataMirror", () => {
  it("emits object yaml paths and content", () => {
    expect(objectYamlPath("Order__c")).toBe("metadata/objects/Order__c.yaml");
    const y = objectToYaml({
      apiName: "Order__c",
      label: "Order",
      pluralLabel: "Orders",
      features: { activities: true },
    });
    expect(y).toContain("apiName: Order__c");
    expect(y).toContain("ownership: custom");
    expect(y).toContain("activities: true");
  });

  it("emits field yaml with picklist", () => {
    expect(fieldYamlPath("Order__c", "Status__c")).toBe("metadata/fields/Order__c/Status__c.yaml");
    const y = fieldToYaml({
      objectApiName: "Order__c",
      apiName: "Status__c",
      label: "Status",
      fieldType: "picklist",
      required: true,
      picklistValues: ["Open", "Closed"],
      referenceTo: "Account",
      length: 40,
    });
    expect(y).toContain("fieldType: picklist");
    expect(y).toContain("- Open");
    expect(y).toContain("required: true");
    expect(y).toContain("referenceTo: Account");
    expect(y).toContain("length: 40");
  });

  it("quotes labels that need escaping", () => {
    const y = objectToYaml({ apiName: "X__c", label: "A: B" });
    expect(y).toContain('label: "A: B"');
  });

  it("emits tool spec yaml paths and one.tool/v1", () => {
    expect(toolSpecYamlPath("Sales_Open_Pipeline")).toBe("metadata/tools/Sales_Open_Pipeline.yaml");
    const y = toolSpecToYaml({
      apiName: "Sales_Open_Pipeline",
      label: "Open pipeline",
      layout: { mode: "sections", sections: [] },
      nodes: [{ id: "hdr", kind: "sectionHeader", props: {} }],
      dataBindings: [],
    });
    expect(y).toContain("apiVersion: one.tool/v1");
    expect(y).toContain("kind: ToolSpec");
    expect(y).toContain("apiName: Sales_Open_Pipeline");
  });

  it("mirrorToolSpecYaml skips managed ownership", async () => {
    const writeText = vi.fn();
    window.one = {
      getSession: vi.fn(),
      setSession: vi.fn(),
      gitStatus: vi.fn(),
      listTree: vi.fn(),
      readText: vi.fn(),
      writeText,
    };
    await expect(
      mirrorToolSpecYaml("/tmp/repo", {
        apiName: "Sales_Open_Pipeline",
        label: "Pipeline",
        layout: {},
        nodes: [],
        ownership: "managed",
      }),
    ).resolves.toBeNull();
    expect(writeText).not.toHaveBeenCalled();
  });

  it("emits playbook yaml", () => {
    const y = playbookToYaml({
      apiName: "QueryAssistant__c",
      label: "Query assistant",
      goalTemplate: "Ask about {{focus}}",
      instructions: "Help",
      allowedTools: ["query"],
      requireApproval: true,
    });
    expect(playbookYamlPath("QueryAssistant__c")).toBe(
      "metadata/agents/playbooks/QueryAssistant__c.yaml",
    );
    expect(y).toContain("apiName: QueryAssistant__c");
    expect(y).toContain("- query");
    expect(y).toContain("requireApproval: true");
  });

  it("emits playbook harness fields", () => {
    const y = playbookToYaml({
      apiName: "QueryAssistant__c",
      label: "Query assistant",
      primarySection: "operate",
      harnessId: "harness.operate.query",
      harnessVersion: "1",
      allowedTools: ["query"],
    });
    expect(y).toContain("primarySection: operate");
    expect(y).toContain("harnessId: harness.operate.query");
    expect(y).toContain("harnessVersion: 1");
  });

  it("mirrorObjectYaml no-ops without repo or writeText", async () => {
    await expect(mirrorObjectYaml(undefined, { apiName: "A", label: "A" })).resolves.toBeNull();
    await expect(mirrorObjectYaml("/tmp/r", { apiName: "A", label: "A" })).resolves.toBeNull();
  });

  it("mirrorObjectYaml writes and reports failures", async () => {
    const writeText = vi.fn().mockResolvedValue(true);
    window.one = {
      getSession: vi.fn(),
      setSession: vi.fn(),
      gitStatus: vi.fn(),
      listTree: vi.fn(),
      readText: vi.fn(),
      writeText,
    };
    await expect(mirrorObjectYaml("/tmp/repo", { apiName: "A__c", label: "A" })).resolves.toBeNull();
    expect(writeText).toHaveBeenCalledWith("/tmp/repo", "metadata/objects/A__c.yaml", expect.any(String));

    writeText.mockRejectedValueOnce(new Error("disk full"));
    await expect(mirrorObjectYaml("/tmp/repo", { apiName: "A__c", label: "A" })).resolves.toMatch(
      /YAML mirror failed/,
    );
  });

  it("mirrorFieldYaml writes and reports failures", async () => {
    const writeText = vi.fn().mockResolvedValue(true);
    window.one = {
      getSession: vi.fn(),
      setSession: vi.fn(),
      gitStatus: vi.fn(),
      listTree: vi.fn(),
      readText: vi.fn(),
      writeText,
    };
    await expect(
      mirrorFieldYaml("/tmp/repo", {
        objectApiName: "A__c",
        apiName: "F__c",
        label: "F",
        fieldType: "string",
      }),
    ).resolves.toBeNull();
    expect(writeText).toHaveBeenCalledWith(
      "/tmp/repo",
      "metadata/fields/A__c/F__c.yaml",
      expect.any(String),
    );
    writeText.mockRejectedValueOnce(new Error("fail"));
    await expect(
      mirrorFieldYaml("/tmp/repo", {
        objectApiName: "A__c",
        apiName: "F__c",
        label: "F",
        fieldType: "string",
      }),
    ).resolves.toMatch(/YAML mirror failed/);
  });
});
