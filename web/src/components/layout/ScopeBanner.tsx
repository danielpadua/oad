import { ShieldCheck } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { ShinyText } from "@/components/reactbits";
import { useAuth } from "@/contexts/AuthContext";
import { useScope } from "@/contexts/ScopeContext";
import { http, type HttpError } from "@/lib/http-client";

interface System {
  id: string;
  name: string;
}

interface SystemsResponse {
  items: System[];
  total: number;
}

export function ScopeBanner() {
  const { identity } = useAuth();
  const { activeSystemId } = useScope();

  const isPlatformAdmin = identity !== null && identity.systemId === null;

  const { data } = useQuery<SystemsResponse, HttpError>({
    queryKey: ["systems"],
    queryFn: () => http.get<SystemsResponse>("/api/v1/systems"),
    staleTime: 5 * 60_000,
    enabled: identity !== null,
  });

  if (!identity) return null;

  const systemName = activeSystemId
    ? (data?.items.find((s) => s.id === activeSystemId)?.name ?? activeSystemId)
    : null;

  return (
    <div className="flex h-8 items-center gap-2 border-b border-border bg-card/60 px-6 text-xs text-muted-foreground">
      <ShieldCheck className="h-3.5 w-3.5 flex-shrink-0 text-primary/60" />
      <span>Scope:</span>

      {systemName ? (
        <ShinyText text={systemName} className="text-xs" speed={6} />
      ) : (
        <span className="font-medium text-foreground">
          {isPlatformAdmin ? "All Systems" : "—"}
        </span>
      )}

      {isPlatformAdmin && (
        <span className="ml-auto font-medium text-primary/60">Platform Admin</span>
      )}
    </div>
  );
}
