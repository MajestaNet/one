import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { ThemeContext } from "../ThemeContext";
import { BrandMark } from "./BrandMark";

afterEach(() => cleanup());

describe("BrandMark", () => {
  it("renders a compact globe with an accessible Majesta.Net name", () => {
    render(
      <ThemeContext.Provider value="dark">
        <BrandMark variant="symbol" />
      </ThemeContext.Provider>,
    );
    expect(screen.getByTestId("brand")).toBeTruthy();
    expect(screen.getByRole("img", { name: /Majesta\.Net/i })).toBeTruthy();
    expect(screen.getByText("Control")).toBeTruthy();
  });

  it("renders a lockup at least 180px wide in CSS class terms", () => {
    render(
      <ThemeContext.Provider value="light">
        <BrandMark variant="lockup" />
      </ThemeContext.Provider>,
    );
    const mark = screen.getByTestId("brand-lockup");
    expect(mark.className).toMatch(/brand-lockup/);
    expect(screen.getByRole("img", { name: /Majesta\.Net/i })).toBeTruthy();
  });

  it("picks gold artwork on dark and navy/two-colour on light", () => {
    const { rerender } = render(
      <ThemeContext.Provider value="dark">
        <BrandMark variant="symbol" />
      </ThemeContext.Provider>,
    );
    const darkSrc = decodeURIComponent(
      (screen.getByRole("img", { name: /Majesta\.Net/i }) as HTMLImageElement).src,
    );
    expect(darkSrc).toMatch(/#F6CF55/i);

    rerender(
      <ThemeContext.Provider value="light">
        <BrandMark variant="symbol" />
      </ThemeContext.Provider>,
    );
    const lightSrc = decodeURIComponent(
      (screen.getByRole("img", { name: /Majesta\.Net/i }) as HTMLImageElement).src,
    );
    expect(lightSrc).toMatch(/#1B2E46/i);
    expect(lightSrc).not.toMatch(/#F6CF55/i);
  });
});
