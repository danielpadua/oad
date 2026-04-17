import { UserManager, WebStorageStateStore, type UserManagerSettings } from "oidc-client-ts";
import { env } from "./env";

const settings: UserManagerSettings = {
  authority: env.VITE_OIDC_AUTHORITY,
  client_id: env.VITE_OIDC_CLIENT_ID,
  redirect_uri: env.VITE_OIDC_REDIRECT_URI,
  post_logout_redirect_uri: env.VITE_OIDC_POST_LOGOUT_URI,
  scope: env.VITE_OIDC_SCOPE,
  response_type: "code",

  // Attempt silent renewal via refresh token; falls back to iframe using silent_redirect_uri.
  automaticSilentRenew: true,
  silent_redirect_uri: `${window.location.origin}/silent-renew`,

  // Session state lives in sessionStorage — cleared on tab close, never persisted to disk.
  userStore: new WebStorageStateStore({ store: window.sessionStorage }),

  filterProtocolClaims: true,
  loadUserInfo: false,
};

export const userManager = new UserManager(settings);
