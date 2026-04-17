import { Link } from "react-router-dom";
import { ShieldOff, ArrowLeft } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useAuth } from "@/contexts/AuthContext";

export default function Forbidden() {
  const { identity } = useAuth();
  const roleList = identity?.roles.join(", ") || "none";

  return (
    <div className="flex h-screen flex-col items-center justify-center gap-6 bg-background px-4 text-center">
      <div className="flex h-16 w-16 items-center justify-center rounded-full bg-destructive/10">
        <ShieldOff className="h-8 w-8 text-destructive" />
      </div>

      <div className="space-y-2">
        <h1 className="text-3xl font-semibold tracking-tight">403 — Forbidden</h1>
        <p className="max-w-sm text-sm text-muted-foreground">
          You don't have the required permissions to access this resource.
          {identity && (
            <>
              {" "}
              Your current roles: <strong className="text-foreground">{roleList}</strong>.
            </>
          )}
        </p>
      </div>

      <Button variant="outline" asChild>
        <Link to="/">
          <ArrowLeft className="h-4 w-4" />
          Back to Dashboard
        </Link>
      </Button>

      <p className="text-xs text-muted-foreground/60">
        If you believe this is an error, contact your system administrator.
      </p>
    </div>
  );
}
