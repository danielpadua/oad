import { useEffect } from "react";
import { useNavigate, useLocation } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";
import { SoftAurora, GradientText, DecryptedText } from "@/components/reactbits";

export default function Login() {
  const { isAuthenticated, isLoading, login } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();

  const returnTo = (location.state as { from?: string } | null)?.from ?? "/";

  // Already authenticated — skip the login page.
  useEffect(() => {
    if (!isLoading && isAuthenticated) {
      navigate(returnTo, { replace: true });
    }
  }, [isAuthenticated, isLoading, navigate, returnTo]);

  const handleLogin = () => void login(returnTo);

  return (
    <div className="relative flex h-screen w-full flex-col items-center justify-center overflow-hidden bg-background">
      <SoftAurora />

      <div className="relative z-10 flex flex-col items-center gap-10">
        {/* Branding */}
        <div className="flex flex-col items-center gap-3">
          <GradientText
            colors={["#3b82f6", "#8b5cf6", "#06b6d4", "#3b82f6"]}
            animationSpeed={6}
            className="text-6xl font-bold tracking-tight"
          >
            OAD
          </GradientText>
          <DecryptedText
            text="Open Authoritative Directory"
            className="text-base text-muted-foreground"
            duration={1800}
            delay={400}
          />
        </div>

        {/* Description */}
        <p className="max-w-xs text-center text-sm text-muted-foreground/80">
          Policy Information Point — authenticate to manage attribute assignments and audit access decisions.
        </p>

        {/* Sign-in button */}
        <button
          onClick={handleLogin}
          disabled={isLoading}
          className="cursor-pointer rounded-md bg-primary px-10 py-2.5 text-sm font-medium text-primary-foreground shadow-sm transition-colors hover:bg-primary/90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50"
        >
          {isLoading ? "Checking session…" : "Sign in with SSO"}
        </button>
      </div>
    </div>
  );
}
