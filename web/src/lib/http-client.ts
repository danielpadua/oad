import { v4 as uuidv4 } from "uuid";
import { env } from "./env";
import { userManager } from "./oidc";

/** Shape of error responses returned by the OAD API (apierr package). */
export interface ApiError {
  code: string;
  message: string;
  details?: unknown;
}

export class HttpError extends Error {
  readonly status: number;
  readonly body: ApiError;

  constructor(status: number, body: ApiError) {
    super(body.message);
    this.name = "HttpError";
    this.status = status;
    this.body = body;
  }
}

export class NetworkError extends Error {
  constructor(cause: unknown) {
    super("Network request failed");
    this.name = "NetworkError";
    this.cause = cause;
  }
}

type RequestOptions = Omit<RequestInit, "body"> & {
  body?: unknown;
  /** Override the bearer token (used by auth flows). */
  token?: string;
};

// ─── Token injection ─────────────────────────────────────────────────────────

// Registered by AuthProvider so every request carries the current access token.
// Optional override — when null, requests fall back to the sync localStorage read.
let tokenGetter: (() => string | null) | null = null;

export function setTokenGetter(fn: () => string | null): void {
  tokenGetter = fn;
}

/**
 * Synchronous fallback that reads the access token directly from oidc-client-ts's
 * user store (localStorage). Required to avoid a race on full-page reload (F5):
 * children's `useEffect` fires before the parent AuthProvider's `useEffect`
 * updates `tokenGetter` from React state, so the first round of API calls would
 * otherwise go out unauthenticated → 401 → forced logout.
 */
function readStoredAccessToken(): string | null {
  try {
    const { authority, client_id } = userManager.settings;
    const raw = window.localStorage.getItem(`oidc.user:${authority}:${client_id}`);
    if (!raw) return null;
    const data = JSON.parse(raw) as { access_token?: string; expires_at?: number };
    if (typeof data.expires_at === "number" && data.expires_at < Date.now() / 1000) {
      return null;
    }
    return typeof data.access_token === "string" ? data.access_token : null;
  } catch {
    return null;
  }
}

// ─── 401 handler ─────────────────────────────────────────────────────────────

// Registered by AuthProvider to redirect to /login on session expiry.
let unauthorizedHandler: (() => void) | null = null;
let handlingUnauthorized = false;

export function setUnauthorizedHandler(fn: () => void): void {
  unauthorizedHandler = () => {
    if (!handlingUnauthorized) {
      handlingUnauthorized = true;
      fn();
    }
  };
}

// ─── Core fetch wrapper ───────────────────────────────────────────────────────

/**
 * Core fetch wrapper. Callers should use the typed helpers (get, post, put, patch, del)
 * rather than calling this directly.
 *
 * Responsibilities:
 * - Prepends VITE_API_BASE_URL to relative paths.
 * - Injects a `X-Correlation-ID` header (UUID v4) for distributed tracing.
 * - Serialises the request body to JSON and sets Content-Type.
 * - Injects the current Bearer token from AuthContext (via tokenGetter).
 * - Throws HttpError for non-2xx responses with the OAD apierr body.
 * - Triggers the 401 handler on session expiry.
 * - Throws NetworkError for transport-level failures.
 */
async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { body, token, ...init } = options;

  const url = path.startsWith("http") ? path : `${env.VITE_API_BASE_URL}${path}`;

  const headers = new Headers(init.headers);
  headers.set("X-Correlation-ID", uuidv4());
  headers.set("Accept", "application/json");

  if (body !== undefined) {
    headers.set("Content-Type", "application/json");
  }

  const resolvedToken = token ?? tokenGetter?.() ?? readStoredAccessToken() ?? undefined;
  if (resolvedToken) {
    headers.set("Authorization", `Bearer ${resolvedToken}`);
  }

  let response: Response;
  try {
    response = await fetch(url, {
      ...init,
      headers,
      body: body !== undefined ? JSON.stringify(body) : undefined,
    });
  } catch (err) {
    throw new NetworkError(err);
  }

  if (!response.ok) {
    let apiError: ApiError;
    try {
      apiError = (await response.json()) as ApiError;
    } catch {
      apiError = { code: "UNKNOWN", message: response.statusText };
    }

    if (response.status === 401) {
      unauthorizedHandler?.();
    }

    throw new HttpError(response.status, apiError);
  }

  // 204 No Content
  if (response.status === 204) {
    return undefined as T;
  }

  return response.json() as Promise<T>;
}

export const http = {
  get: <T>(path: string, options?: RequestOptions) =>
    request<T>(path, { ...options, method: "GET" }),

  post: <T>(path: string, body: unknown, options?: RequestOptions) =>
    request<T>(path, { ...options, method: "POST", body }),

  put: <T>(path: string, body: unknown, options?: RequestOptions) =>
    request<T>(path, { ...options, method: "PUT", body }),

  patch: <T>(path: string, body: unknown, options?: RequestOptions) =>
    request<T>(path, { ...options, method: "PATCH", body }),

  del: <T>(path: string, options?: RequestOptions) =>
    request<T>(path, { ...options, method: "DELETE" }),
};
