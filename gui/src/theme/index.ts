export type ThemeName = "dark";

const storageKey = "solomon.theme";

export function savedTheme(): ThemeName {
  const value = window.localStorage.getItem(storageKey);
  return isThemeName(value) ? value : "dark";
}

export function applyTheme(theme: ThemeName): ThemeName {
  document.documentElement.dataset.theme = theme;
  document.documentElement.style.colorScheme = "dark";
  return theme;
}

export function saveTheme(theme: ThemeName): ThemeName {
  window.localStorage.setItem(storageKey, theme);
  return applyTheme(theme);
}

function isThemeName(value: string | null): value is ThemeName {
  return value === "dark";
}
