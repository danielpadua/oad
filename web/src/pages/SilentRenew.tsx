import { useEffect, useRef } from "react";
import { signinSilentCallbackAll } from "@/lib/oidc";

/**
 * Lightweight page loaded inside the hidden iframe by oidc-client-ts during
 * silent token renewal. It calls `signinSilentCallback` which posts the new
 * tokens back to the parent window via postMessage and closes the iframe.
 */
export default function SilentRenew() {
  const handled = useRef(false);

  useEffect(() => {
    if (handled.current) return;
    handled.current = true;

    signinSilentCallbackAll().catch((err) => {
      console.error("[OAD] Silent renew callback error:", err);
    });
  }, []);

  return null;
}
