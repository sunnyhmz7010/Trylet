import { preferences } from "./preferences";
import {
  createContext,
  useContext,
  useState,
  useLayoutEffect,
  type ReactNode,
} from "react";
export type Theme = "dark" | "light" | "green";
export function loadTheme(): Theme {
  try {
    const value = preferences.getItem("oil-git.theme");
    if (value === "light" || value === "green" || value === "dark")
      return value;
  } catch {}
  return matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}
export function applyTheme(theme: Theme) {
  const root = document.querySelector<HTMLElement>(".oil-git"); if (!root) return; root.dataset.theme = theme;
  root.style.colorScheme =
    theme === "light" ? "light" : "dark";
}
const ThemeContext = createContext<{
  theme: Theme;
  setTheme: (theme: Theme) => void;
}>({ theme: "dark", setTheme() {} });
export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setValue] = useState<Theme>(loadTheme);
  useLayoutEffect(() => applyTheme(theme), [theme]);
  const setTheme = (value: Theme) => {
    applyTheme(value);
    try {
      preferences.setItem("oil-git.theme", value);
    } catch {}
    setValue(value);
  };
  return (
    <ThemeContext.Provider value={{ theme, setTheme }}>
      {children}
    </ThemeContext.Provider>
  );
}
export const useTheme = () => useContext(ThemeContext);
