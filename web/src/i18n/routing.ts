import { defineRouting } from "next-intl/routing";

export const routing = defineRouting({
  locales: [
    "en",
    "zh",
    "ja",
    "ko",
    "vi",
    "tr",
    "ru",
    "es",
    "pt",
    "fr",
    "de",
    "id",
    "th",
    "hi",
  ],
  defaultLocale: "en",
  localePrefix: "as-needed",
});

// Label map for the language switcher UI
export const localeNames: Record<string, string> = {
  en: "English",
  zh: "中文",
  ja: "日本語",
  ko: "한국어",
  vi: "Tiếng Việt",
  tr: "Türkçe",
  ru: "Русский",
  es: "Español",
  pt: "Português",
  fr: "Français",
  de: "Deutsch",
  id: "Bahasa Indonesia",
  th: "ไทย",
  hi: "हिन्दी",
};
