import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import type { User } from "oidc-client-ts";
import { getUserManager, setActiveProviderName } from "@/lib/oidc";
import { http, setTokenGetter, setUnauthorizedHandler, setActiveSystemId } from "@/lib/http-client";
import type { MeResponse } from "@/lib/types";

// ─── Types ────────────────────────────────────────────────────────────────────

export interface AuthIdentity {
  sub: string;
  email?: string;
  name?: string;
  isPlatformAdmin: boolean;
  /** Built-in group external IDs (e.g. "oad:admin", "oad:editor", "oad:viewer"). */
  groups: string[];
  allowedSystems: string[];
  /** Currently active system UUID; null means no system scope selected. */
  activeSystemId: string | null;
}

interface AuthContextValue {
  isAuthenticated: boolean;
  isLoading: boolean;
  identity: AuthIdentity | null;
  setActiveSystem: (id: string | null) => void;
  login: (returnTo?: string, providerName?: string) => Promise<void>;
  logout: () => Promise<void>;
}

// ─── Context ──────────────────────────────────────────────────────────────────

const AuthContext = createContext<AuthContextValue | null>(null);

async function fetchMeIdentity(user: User): Promise<AuthIdentity | null> {
  try {
    const me = await http.get<MeResponse>("/api/v1/me", { token: user.access_token });
    const activeSystemId = me.allowed_systems[0] ?? null;
    return {
      sub: me.sub,
      isPlatformAdmin: me.is_platform_admin,
      groups: me.groups,
      allowedSystems: me.allowed_systems,
      activeSystemId,
    };
  } catch {
    return null;
  }
}

// ─── Provider ─────────────────────────────────────────────────────────────────

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [identity, setIdentity] = useState<AuthIdentity | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  // Keep the http-client token getter in sync with the current user.
  useEffect(() => {
    setTokenGetter(() => user?.access_token ?? null);
  }, [user]);

  // Fetch /api/v1/me after user is available and update identity + active system.
  useEffect(() => {
    if (user && !user.expired) {
      fetchMeIdentity(user).then((id) => {
        setIdentity(id);
        setActiveSystemId(id?.activeSystemId ?? null);
      });
    } else {
      setIdentity(null);
      setActiveSystemId(null);
    }
  }, [user]);

  // Redirect to /login on 401 — clears OIDC session before redirecting.
  useEffect(() => {
    setUnauthorizedHandler(() => {
      getUserManager().removeUser().catch(() => {});
      window.location.replace("/login");
    });
  }, []);

  // Load user from sessionStorage on mount, then subscribe to OIDC events.
  useEffect(() => {
    getUserManager()
      .getUser()
      .then((u) => setUser(u && !u.expired ? u : null))
      .catch(() => setUser(null))
      .finally(() => setIsLoading(false));

    const onUserLoaded = (u: User) => setUser(u);
    const onUserUnloaded = () => setUser(null);
    const onTokenExpired = () => {
      setUser(null);
      window.location.replace("/login");
    };
    const onSilentRenewError = () => {
      setUser(null);
      window.location.replace("/login");
    };

    getUserManager().events.addUserLoaded(onUserLoaded);
    getUserManager().events.addUserUnloaded(onUserUnloaded);
    getUserManager().events.addAccessTokenExpired(onTokenExpired);
    getUserManager().events.addSilentRenewError(onSilentRenewError);

    return () => {
      getUserManager().events.removeUserLoaded(onUserLoaded);
      getUserManager().events.removeUserUnloaded(onUserUnloaded);
      getUserManager().events.removeAccessTokenExpired(onTokenExpired);
      getUserManager().events.removeSilentRenewError(onSilentRenewError);
    };
  }, []);

  const login = async (returnTo?: string, providerName?: string) => {
    if (providerName) setActiveProviderName(providerName);
    await getUserManager().signinRedirect({
      state: returnTo ? { returnTo } : undefined,
    });
  };

  const logout = async () => {
    await getUserManager().signoutRedirect();
  };

  const setActiveSystem = (id: string | null) => {
    setIdentity((prev) => (prev ? { ...prev, activeSystemId: id } : null));
    setActiveSystemId(id);
  };

  return (
    <AuthContext.Provider
      value={{
        isAuthenticated: !!user,
        isLoading,
        identity,
        setActiveSystem,
        login,
        logout,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

// ─── Hook ─────────────────────────────────────────────────────────────────────

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used inside <AuthProvider>");
  return ctx;
}
