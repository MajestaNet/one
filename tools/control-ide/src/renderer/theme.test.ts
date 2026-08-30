import { afterEach, describe, expect, it, vi } from "vitest";
import {
  THEME_STORAGE_KEY,
  applyTheme,
  detectPreferredTheme,
  loadStoredTheme,
  monacoThemeFor,
  persistTheme,
  resolveInitialTheme,
  toggleTheme,
} from "./theme";

afterEach(() => {
  window.localStorage.removeItem(THEME_STORAGE_KEY);
  document.documentElement.removeAttribute("data-theme");
  vi.restoreAllMocks();
});

describe("theme", () => {
  it("persists and loads theme from localStorage", () => {
    persistTheme("light");
    expect(loadStoredTheme()).toBe("light");
    expect(resolveInitialTheme()).toBe("light");
  });

  it("falls back to preferred scheme when unset", () => {
    window.localStorage.removeItem(THEME_STORAGE_KEY);
    vi.spyOn(window, "matchMedia").mockImplementation((query: string) => {
      return {
        matches: query.includes("light"),
        media: query,
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      } as MediaQueryList;
    });
    expect(detectPreferredTheme()).toBe("light");
    expect(resolveInitialTheme()).toBe("light");
  });

  it("applies data-theme on the document element", () => {
    applyTheme("light");
    expect(document.documentElement.dataset.theme).toBe("light");
    expect(document.documentElement.style.colorScheme).toBe("light");
  });

  it("toggles and maps monaco themes", () => {
    expect(toggleTheme("dark")).toBe("light");
    expect(toggleTheme("light")).toBe("dark");
    expect(monacoThemeFor("dark")).toBe("vs-dark");
    expect(monacoThemeFor("light")).toBe("vs");
  });
});
