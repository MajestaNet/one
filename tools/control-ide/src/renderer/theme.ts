export type Theme = "dark" | "light";

export const THEME_STORAGE_KEY = "one.control.theme";

export function detectPreferredTheme(): Theme {
  if (typeof window === "undefined") return "dark";
  try {
    if (window.matchMedia("(prefers-color-scheme: light)").matches) return "light";
  } catch {
    /* jsdom may not implement matchMedia fully */
  }
  return "dark";
}

export function loadStoredTheme(): Theme | null {
  if (typeof window === "undefined") return null;
  try {
    const v = window.localStorage.getItem(THEME_STORAGE_KEY);
    if (v === "dark" || v === "light") return v;
  } catch {
    /* private mode / denied */
  }
  return null;
}

export function resolveInitialTheme(): Theme {
  return loadStoredTheme() ?? detectPreferredTheme();
}

export function persistTheme(theme: Theme): void {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(THEME_STORAGE_KEY, theme);
  } catch {
    /* ignore */
  }
}

export function applyTheme(theme: Theme): void {
  if (typeof document === "undefined") return;
  document.documentElement.dataset.theme = theme;
  document.documentElement.style.colorScheme = theme;
}

export function toggleTheme(current: Theme): Theme {
  return current === "dark" ? "light" : "dark";
}

export function monacoThemeFor(theme: Theme): "vs-dark" | "vs" {
  return theme === "dark" ? "vs-dark" : "vs";
}
