import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import HttpBackend from "i18next-http-backend";

export const SUPPORTED_LOCALES = ["en", "pt-BR"] as const;
export type Locale = (typeof SUPPORTED_LOCALES)[number];

export const DEFAULT_LOCALE: Locale = "en";

const detectedLocale = (): Locale => {
  const stored = localStorage.getItem("oad-locale");
  if (stored && SUPPORTED_LOCALES.includes(stored as Locale)) {
    return stored as Locale;
  }
  const browser = navigator.language;
  if (browser.startsWith("pt")) return "pt-BR";
  return DEFAULT_LOCALE;
};

i18n
  .use(HttpBackend)
  .use(initReactI18next)
  .init({
    lng: detectedLocale(),
    fallbackLng: DEFAULT_LOCALE,
    supportedLngs: SUPPORTED_LOCALES,

    // Namespaces loaded on-demand via i18next-http-backend
    ns: ["common"],
    defaultNS: "common",

    backend: {
      loadPath: "/locales/{{lng}}/{{ns}}.json",
    },

    interpolation: {
      escapeValue: false,
    },

    // Do not suspend rendering — components that use t() render immediately
    // with fallback keys while the bundle loads.
    react: {
      useSuspense: false,
    },
  });

export function setLocale(locale: Locale): void {
  localStorage.setItem("oad-locale", locale);
  void i18n.changeLanguage(locale);
}

export { i18n };
export default i18n;
