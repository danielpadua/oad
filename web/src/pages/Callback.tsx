import { useEffect, useRef } from "react";
import { useNavigate } from "react-router-dom";
import { getUserManager } from "@/lib/oidc";

export default function Callback() {
  const navigate = useNavigate();
  const handled = useRef(false);

  useEffect(() => {
    // StrictMode double-fires effects — guard against processing the callback twice.
    if (handled.current) return;
    handled.current = true;

    getUserManager()
      .signinRedirectCallback()
      .then((user) => {
        const state = user.state as { returnTo?: string } | undefined;
        navigate(state?.returnTo ?? "/", { replace: true });
      })
      .catch((err) => {
        console.error("[OAD] OIDC callback error:", err);
        navigate("/login", { replace: true });
      });
  }, [navigate]);

  return (
    <div className="flex h-screen items-center justify-center bg-background">
      <div className="h-8 w-8 animate-spin rounded-full border-4 border-border border-t-primary" />
    </div>
  );
}
