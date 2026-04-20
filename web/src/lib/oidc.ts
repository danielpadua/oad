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

  // Disable iframe-based session monitoring: on localhost the SPA (port 5173) and Keycloak
  // (port 8081) are different origins, so SameSite=Lax cookies are not sent in the hidden
  // check-session iframe. The monitor would fire SessionChanged → removeUser() on every
  // page load, dropping the session immediately after F5.
  monitorSession: false,

  // Session state in localStorage survives F5 and tab reopening — acceptable for dev.
  userStore: new WebStorageStateStore({ store: window.localStorage }),

  filterProtocolClaims: true,
  loadUserInfo: false,
};

export const userManager = new UserManager(settings);
