"use client";
import { useEffect, useState } from "react";

export function ThemeToggle() {
  const [dark, setDark] = useState(false);
  useEffect(() => {
    const stored = localStorage.getItem("vs-theme");
    const preferDark = window.matchMedia("(prefers-color-scheme: dark)").matches;
    const isDark = stored ? stored === "dark" : preferDark;
    setDark(isDark);
    document.documentElement.dataset.theme = isDark ? "dark" : "light";
  }, []);
  function toggle() {
    const next = !dark;
    setDark(next);
    document.documentElement.dataset.theme = next ? "dark" : "light";
    localStorage.setItem("vs-theme", next ? "dark" : "light");
  }
  return (
    <button
      onClick={toggle}
      className="rounded-full border border-ink/20 px-4 py-2 text-sm font-medium hover:bg-card"
      aria-pressed={dark}
    >
      {dark ? "Light mode" : "Dark mode"}
    </button>
  );
}