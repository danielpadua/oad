import "@testing-library/jest-dom";
import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import enCommon from "../../public/locales/en/common.json";
import enEntityTypes from "../../public/locales/en/entityTypes.json";
import enSystems from "../../public/locales/en/systems.json";
import enDashboard from "../../public/locales/en/dashboard.json";
import enAuth from "../../public/locales/en/auth.json";
import { server } from "./mocks/server";

// ─── i18next inline initialization for tests (no HTTP backend) ───────────────

void i18n.use(initReactI18next).init({
  lng: "en",
  fallbackLng: "en",
  ns: ["common", "entityTypes", "systems", "dashboard", "auth"],
  defaultNS: "common",
  resources: {
    en: {
      common: enCommon,
      entityTypes: enEntityTypes,
      systems: enSystems,
      dashboard: enDashboard,
      auth: enAuth,
    },
  },
  interpolation: { escapeValue: false },
  react: { useSuspense: false },
});

// ─── Browser API polyfills for jsdom ─────────────────────────────────────────

// framer-motion uses IntersectionObserver via useInView
class MockIntersectionObserver {
  observe = vi.fn();
  unobserve = vi.fn();
  disconnect = vi.fn();
  takeRecords = vi.fn(() => [] as IntersectionObserverEntry[]);
}
vi.stubGlobal("IntersectionObserver", MockIntersectionObserver);

// framer-motion uses ResizeObserver for layout animations
class MockResizeObserver {
  observe = vi.fn();
  unobserve = vi.fn();
  disconnect = vi.fn();
}
vi.stubGlobal("ResizeObserver", MockResizeObserver);

// framer-motion reads prefers-reduced-motion
Object.defineProperty(window, "matchMedia", {
  writable: true,
  value: vi.fn().mockImplementation((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })),
});

// ─── MSW lifecycle ────────────────────────────────────────────────────────────

beforeAll(() => server.listen({ onUnhandledRequest: "warn" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());
