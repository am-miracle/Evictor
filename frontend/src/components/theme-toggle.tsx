"use client";

import { useSyncExternalStore } from "react";

type ThemePreference = "light" | "dark" | "system";

const preferences: readonly ThemePreference[] = ["light", "dark", "system"];
const themeEvent = "evictor-theme-change";

function getPreference(): ThemePreference {
  const value = window.localStorage.getItem("evictor-theme");
  return value === "light" || value === "dark" ? value : "system";
}

function resolveTheme(preference: ThemePreference): "light" | "dark" {
  if (preference !== "system") return preference;
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

function applyTheme(preference: ThemePreference) {
  const root = document.documentElement;
  root.classList.remove("light", "dark");
  root.classList.add(resolveTheme(preference));
  root.dataset["themePreference"] = preference;
}

function subscribe(onStoreChange: () => void) {
  const media = window.matchMedia("(prefers-color-scheme: dark)");
  const handlePreferenceChange = () => {
    applyTheme(getPreference());
    onStoreChange();
  };

  window.addEventListener("storage", handlePreferenceChange);
  window.addEventListener(themeEvent, handlePreferenceChange);
  media.addEventListener("change", handlePreferenceChange);

  return () => {
    window.removeEventListener("storage", handlePreferenceChange);
    window.removeEventListener(themeEvent, handlePreferenceChange);
    media.removeEventListener("change", handlePreferenceChange);
  };
}

function setPreference(preference: ThemePreference) {
  window.localStorage.setItem("evictor-theme", preference);
  applyTheme(preference);
  window.dispatchEvent(new Event(themeEvent));
}

export function ThemeToggle() {
  const preference = useSyncExternalStore(subscribe, getPreference, () => "system");

  return (
    <div className="theme-toggle" role="group" aria-label="Color theme">
      {preferences.map((option) => (
        <button
          type="button"
          key={option}
          aria-pressed={preference === option}
          onClick={() => setPreference(option)}
        >
          {option}
        </button>
      ))}
    </div>
  );
}
