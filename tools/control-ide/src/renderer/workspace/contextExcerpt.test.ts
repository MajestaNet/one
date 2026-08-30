import { describe, expect, it } from "vitest";
import {
  formatExcerptText,
  excerptFromPlainText,
  parseContextExcerpt,
  rowsToContextExcerpt,
  serializeContextExcerpt,
  isContextExcerptDrag,
} from "./contextExcerpt";

describe("contextExcerpt", () => {
  it("formats rows as tabular text", () => {
    const text = formatExcerptText(
      [{ Name: "Acme", id: "1" }, { Name: "Beta", id: "2" }],
      [{ key: "Name", label: "Name" }, { key: "id", label: "Id" }],
    );
    expect(text).toContain("Name | id");
    expect(text).toContain("Acme");
    expect(text).toContain("Beta");
  });

  it("builds excerpt from rows with object label", () => {
    const excerpt = rowsToContextExcerpt({
      rows: [{ Name: "Acme" }],
      objectApiName: "Account",
      columns: [{ key: "Name", label: "Name" }],
    });
    expect(excerpt.label).toMatch(/1 Account/);
    expect(excerpt.structured?.objectApiName).toBe("Account");
  });

  it("round-trips serialize/parse", () => {
    const excerpt = rowsToContextExcerpt({ rows: [{ a: 1 }], label: "test" });
    const parsed = parseContextExcerpt(serializeContextExcerpt(excerpt));
    expect(parsed?.label).toBe(excerpt.label);
    expect(parsed?.text).toBe(excerpt.text);
  });

  it("excerptFromPlainText trims and labels", () => {
    const ex = excerptFromPlainText("  hello world  ");
    expect(ex.text).toBe("hello world");
    expect(ex.source).toBe("selection");
  });

  it("isContextExcerptDrag detects mime types", () => {
    const dt = {
      types: ["application/x-one-context-excerpt"],
    } as DataTransfer;
    expect(isContextExcerptDrag(dt)).toBe(true);
  });
});
