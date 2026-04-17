import { Navigate, useLocation } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";

function Spinner() {
  return (
    <div className="flex h-full items-center justify-center">
      <div className="h-8 w-8 animate-spin rounded-full border-4 border-border border-t-primary" />
    </div>
  );
}

interface ProtectedRouteProps {
  children: React.ReactNode;
}

/**
 * Wraps protected route content. While the OIDC session is loading, shows a
 * spinner. Once resolved, unauthenticated users are redirected to /login with
 * the current path stored in location state so the login page can redirect back
 * after a successful sign-in.
 */
export function ProtectedRoute({ children }: ProtectedRouteProps) {
  const { isAuthenticated, isLoading } = useAuth();
  const location = useLocation();

  if (isLoading) return <Spinner />;

  if (!isAuthenticated) {
    return <Navigate to="/login" state={{ from: location.pathname }} replace />;
  }

  return <>{children}</>;
}
