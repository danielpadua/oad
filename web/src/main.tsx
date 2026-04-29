import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import React from "react";
import ReactDOM from "react-dom";
import "./index.css";
import "./lib/i18n"; // initialize i18n before rendering
import { App } from "./App";
import { fetchConfig } from "./lib/config";
import { initUserManager } from "./lib/oidc";

const rootEl = document.getElementById("root");
if (!rootEl) throw new Error("Root element #root not found");

// axe-core accessibility auditing — dev only, never ships to production
if (import.meta.env.DEV) {
  const { default: axe } = await import("@axe-core/react");
  axe(React, ReactDOM, 1000);
}

const cfg = await fetchConfig();
initUserManager(cfg);

createRoot(rootEl).render(
  <StrictMode>
    <App />
  </StrictMode>
);
