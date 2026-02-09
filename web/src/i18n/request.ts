import { getRequestConfig } from "next-intl/server";
import { hasLocale } from "next-intl";
import { routing } from "./routing";

const messageImports = {
  en: () => import("../../messages/en.json"),
  zh: () => import("../../messages/zh.json"),
  ja: () => import("../../messages/ja.json"),
  ko: () => import("../../messages/ko.json"),
  vi: () => import("../../messages/vi.json"),
  tr: () => import("../../messages/tr.json"),
  ru: () => import("../../messages/ru.json"),
  es: () => import("../../messages/es.json"),
  pt: () => import("../../messages/pt.json"),
  fr: () => import("../../messages/fr.json"),
  de: () => import("../../messages/de.json"),
  id: () => import("../../messages/id.json"),
  th: () => import("../../messages/th.json"),
  hi: () => import("../../messages/hi.json"),
} as const;

export default getRequestConfig(async ({ requestLocale }) => {
  const requested = await requestLocale;
  const locale = hasLocale(routing.locales, requested)
    ? requested
    : routing.defaultLocale;

  return {
    locale,
    messages: (await messageImports[locale as keyof typeof messageImports]())
      .default,
  };
});
