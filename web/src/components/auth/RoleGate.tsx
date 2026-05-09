import { type ReactNode } from "react";
import { useAuth } from "@/contexts/AuthContext";

// ─── Hook ─────────────────────────────────────────────────────────────────────

export function useRole() {
  const { identity } = useAuth();
  const isPlatformAdmin = identity?.isPlatformAdmin ?? false;
  const groups = identity?.groups ?? [];

  return {
    hasRole: (role: string) =>
      isPlatformAdmin ||
      groups.includes(`oad:${role}`),
    hasAnyRole: (r: string[]) =>
      isPlatformAdmin ||
      r.some((role) => groups.includes(`oad:${role}`)),
    isAdmin: isPlatformAdmin,
    isEditor: isPlatformAdmin || groups.includes("oad:editor"),
    isViewer:
      isPlatformAdmin ||
      groups.includes("oad:editor") ||
      groups.includes("oad:viewer"),
    /** Can create or update records (platform admin or editor group). */
    canWrite: isPlatformAdmin || groups.includes("oad:editor"),
    /** Can delete records (platform admin only). */
    canDelete: isPlatformAdmin,
    isPlatformAdmin,
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
