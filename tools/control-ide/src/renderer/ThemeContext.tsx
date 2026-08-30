import { createContext, useContext } from "react";
import type { Theme } from "./theme";

export const ThemeContext = createContext<Theme>("dark");

export function useTheme(): Theme {
  return useContext(ThemeContext);
}
