import { UserManager, WebStorageStateStore, type UserManagerSettings } from "oidc-client-ts";
import type { OADConfig, OADProvider } from "./config";

const STORAGE_KEY = "oad.active_idp";

let _managers: Map<string, UserManager> = new Map();
let _providers: OADProvider[] = [];

function makeSettings(p: OADProvider, cfg: OADConfig): UserManagerSettings {
  return {
    authority: p.authority,
    client_id: p.client_id,
    redirect_uri: cfg.redirect_uri,
    post_logout_redirect_uri: cfg.post_logout_uri,
    scope: p.scope,
    response_type: "code",

    // Attempt silent renewal via refresh token; falls back to iframe using silent_redirect_uri.
    automaticSilentRenew: true,
    silent_redirect_uri: `${window.location.origin}/silent-renew`,

    // Disable iframe-based session monitoring: on localhost the SPA and the IdP
    // are different origins, so SameSite=Lax cookies are not sent in the hidden
    // check-session iframe, which would fire SessionChanged and drop the session.
    monitorSession: false,

    userStore: new WebStorageStateStore({ store: window.localStorage }),

    filterProtocolClaims: true,
    loadUserInfo: false,
  };
}

// initUserManager must be called once at app bootstrap (before rendering)
// with the config fetched from /config.json.
export function initUserManager(cfg: OADConfig): void {
  if (cfg.providers.length === 0) {
    throw new Error("No OIDC providers configured in /config.json");
  }
  _providers = cfg.providers;
  _managers = new Map();
  for (const p of cfg.providers) {
    _managers.set(p.name, new UserManager(makeSettings(p, cfg)));
  }

  // Seed the active-provider hint only when none is set or the stored name is no longer valid.
  const stored = sessionStorage.getItem(STORAGE_KEY);
  if (!stored || !_managers.has(stored)) {
    sessionStorage.setItem(STORAGE_KEY, cfg.providers[0].name);
  }
}

export function getProviders(): OADProvider[] {
  return _providers;
}

export function getActiveProviderName(): string | null {
  return sessionStorage.getItem(STORAGE_KEY);
}

export function setActiveProviderName(name: string): void {
  sessionStorage.setItem(STORAGE_KEY, name);
}

// Returns the UserManager for the given provider name, or the active one when omitted.
export function getUserManager(providerName?: string): UserManager {
  const name = providerName ?? getActiveProviderName();
  if (!name) {
    throw new Error("No active OIDC provider — call initUserManager before rendering");
  }
  const mgr = _managers.get(name);
  if (!mgr) {
    throw new Error(`No UserManager for provider "${name}" — call initUserManager first`);
  }
  return mgr;
}

// Used by SilentRenew: the iframe has its own sessionStorage so the active-provider
// hint is not available. We try every manager — only the one whose stored state matches
// the callback URL will succeed.
export async function signinSilentCallbackAll(): Promise<void> {
  for (const mgr of _managers.values()) {
    try {
      await mgr.signinSilentCallback();
      return;
    } catch {
      // state didn't match this provider — try the next one
    }
  }
}
