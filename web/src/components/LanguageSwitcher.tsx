"use client";

import { useState, useRef, useEffect } from "react";
import { useLocale } from "next-intl";
import { usePathname, useRouter } from "@/i18n/navigation";
import { routing, localeNames } from "@/i18n/routing";

const localeFlags: Record<string, string> = {
  en: "🇺🇸",
  zh: "🇨🇳",
  ja: "🇯🇵",
  ko: "🇰🇷",
  vi: "🇻🇳",
  tr: "🇹🇷",
  ru: "🇷🇺",
  es: "🇪🇸",
  pt: "🇧🇷",
  fr: "🇫🇷",
  de: "🇩🇪",
  id: "🇮🇩",
  th: "🇹🇭",
  hi: "🇮🇳",
};

export default function LanguageSwitcher() {
  const locale = useLocale();
  const router = useRouter();
  const pathname = usePathname();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  // Close dropdown on outside click
  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, []);

  function switchLocale(next: string) {
    setOpen(false);
    router.replace(pathname, { locale: next as typeof locale });
  }

  return (
    <div className="relative" ref={ref}>
      <button
        onClick={() => setOpen((o) => !o)}
        className="flex items-center gap-1.5 rounded-lg border border-[#1e1e2e] px-2.5 py-1.5 text-sm text-[#94a3b8] transition-colors hover:border-[#8b5cf6] hover:text-white"
        aria-label="Switch language"
      >
        <span>{localeFlags[locale] ?? "🌐"}</span>
        <span className="hidden sm:inline">
          {localeNames[locale as keyof typeof localeNames] ?? locale}
        </span>
        <svg
          className={`h-3 w-3 transition-transform ${open ? "rotate-180" : ""}`}
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M19 9l-7 7-7-7"
          />
        </svg>
      </button>

      {open && (
        <div className="absolute right-0 z-50 mt-2 max-h-80 w-48 overflow-y-auto rounded-xl border border-[#1e1e2e] bg-[#0a0a12] shadow-xl">
          {routing.locales.map((l) => (
            <button
              key={l}
              onClick={() => switchLocale(l)}
              className={`flex w-full items-center gap-2.5 px-3.5 py-2 text-left text-sm transition-colors hover:bg-[#1e1e2e] ${
                l === locale
                  ? "bg-[#1e1e2e]/50 text-[#8b5cf6]"
                  : "text-[#94a3b8]"
              }`}
            >
              <span className="text-base">{localeFlags[l] ?? "🌐"}</span>
              <span>{localeNames[l as keyof typeof localeNames] ?? l}</span>
              {l === locale && (
                <svg
                  className="ml-auto h-4 w-4 text-[#8b5cf6]"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M5 13l4 4L19 7"
                  />
                </svg>
              )}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
