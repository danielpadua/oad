import { useTranslation } from "react-i18next";
import { setLocale, SUPPORTED_LOCALES, type Locale } from "@/lib/i18n";
import { cn } from "@/lib/utils";

const LOCALE_LABELS: Record<Locale, string> = {
  en: "EN",
  "pt-BR": "PT",
};

export function LocaleSwitcher({ className }: { className?: string }) {
  const { i18n } = useTranslation();
  const current = i18n.language as Locale;

  return (
    <div
      className={cn("flex items-center gap-0.5 rounded-lg border border-border bg-muted/40 p-0.5", className)}
      role="group"
      aria-label="Language selector"
    >
      {SUPPORTED_LOCALES.map((locale) => (
        <button
          key={locale}
          onClick={() => setLocale(locale)}
          aria-pressed={current === locale}
          className={cn(
            "cursor-pointer rounded-md px-2 py-0.5 text-xs font-medium transition-colors",
            current === locale
              ? "bg-background text-foreground shadow-sm"
              : "text-muted-foreground hover:text-foreground",
          )}
        >
          {LOCALE_LABELS[locale]}
        </button>
      ))}
    </div>
  );
}
