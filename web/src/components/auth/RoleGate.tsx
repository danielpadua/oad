import { type ReactNode } from "react";
import { useAuth } from "@/contexts/AuthContext";

// ─── Hook ─────────────────────────────────────────────────────────────────────

export function useRole() {
  const { identity } = useAuth();
  const roles = identity?.roles ?? [];

  return {
    hasRole: (role: string) => roles.includes(role),
    hasAnyRole: (r: string[]) => r.some((role) => roles.includes(role)),
    isAdmin: roles.includes("admin"),
    isEditor: roles.includes("editor"),
    isViewer: roles.includes("viewer"),
    /** Can create or update records (admin or editor). */
    canWrite: roles.includes("admin") || roles.includes("editor"),
    /** Can delete records (admin only). */
    canDelete: roles.includes("admin"),
    /** JWT carries no system scope — unrestricted platform access. */
    isPlatformAdmin: identity !== null && identity.systemId === null,
  };
}

// ─── Guards ───────────────────────────────────────────────────────────────────

interface RequireRoleProps {
  role: string;
  children: ReactNode;
  /** Rendered when the role check fails. Defaults to null (invisible). */
  fallback?: ReactNode;
}

/** Renders children only when the current user has the specified role. */
export function RequireRole({ role, children, fallback = null }: RequireRoleProps) {
  const { hasRole } = useRole();
  return hasRole(role) ? <>{children}</> : <>{fallback}</>;
}

interface RequireAnyRoleProps {
  roles: string[];
  children: ReactNode;
  fallback?: ReactNode;
}

/** Renders children when the current user has at least one of the specified roles. */
export function RequireAnyRole({ roles, children, fallback = null }: RequireAnyRoleProps) {
  const { hasAnyRole } = useRole();
  return hasAnyRole(roles) ? <>{children}</> : <>{fallback}</>;
}
