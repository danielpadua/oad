import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import type { User } from "oidc-client-ts";
import { getUserManager, setActiveProviderName } from "@/lib/oidc";
import { setTokenGetter, setUnauthorizedHandler } from "@/lib/http-client";

// ─── Types ────────────────────────────────────────────────────────────────────

export interface AuthIdentity {
  sub: string;
  email?: string;
  name?: string;
  /** OAD application roles extracted from the `oad_roles` JWT claim. */
  roles: string[];
  /** System UUID from `oad_system_id` claim; null means platform admin (unrestricted). */
  systemId: string | null;
}

interface AuthContextValue {
  isAuthenticated: boolean;
  isLoading: boolean;
  identity: AuthIdentity | null;
  login: (returnTo?: string, providerName?: string) => Promise<void>;
  logout: () => Promise<void>;
}

// ─── Context ──────────────────────────────────────────────────────────────────

const AuthContext = createContext<AuthContextValue | null>(null);

function userToIdentity(user: User): AuthIdentity {
  const claims = user.profile;
  return {
    sub: claims.sub,
    email: claims.email,
    name: claims.name,
    roles: (claims["oad_roles"] as string[] | undefined) ?? [],
    systemId: (claims["oad_system_id"] as string | undefined) ?? null,
  };
}

// ─── Provider ─────────────────────────────────────────────────────────────────

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  // Keep the http-client token getter in sync with the current user.
  useEffect(() => {
    setTokenGetter(() => user?.access_token ?? null);
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

  return (
    <AuthContext.Provider
      value={{
        isAuthenticated: !!user,
        isLoading,
        identity: user ? userToIdentity(user) : null,
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
