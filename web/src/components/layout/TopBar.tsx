import { useState } from "react";
import { LogOut, ChevronDown, Layers } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { GradientText, BorderGlow } from "@/components/reactbits";
import { useAuth } from "@/contexts/AuthContext";
import { useScope } from "@/contexts/ScopeContext";
import { useRole } from "@/components/auth/RoleGate";
import { http, type HttpError } from "@/lib/http-client";
import { cn } from "@/lib/utils";

// ─── Types ────────────────────────────────────────────────────────────────────

interface System {
  id: string;
  name: string;
  active: boolean;
}

interface SystemsResponse {
  items: System[];
  total: number;
}

// ─── Systems query (shared by selector and banner via TanStack cache) ─────────

function useSystemsList(enabled: boolean) {
  return useQuery<SystemsResponse, HttpError>({
    queryKey: ["systems"],
    queryFn: () => http.get<SystemsResponse>("/api/v1/systems"),
    enabled,
    staleTime: 5 * 60_000,
  });
}

// ─── System scope selector (platform admins only) ────────────────────────────

function SystemScopeSelector() {
  const { activeSystemId, setActiveSystemId } = useScope();
  const { data } = useSystemsList(true);
  const [open, setOpen] = useState(false);

  const activeSystems = data?.items.filter((s) => s.active) ?? [];
  const activeName = activeSystems.find((s) => s.id === activeSystemId)?.name;
  const hasScope = activeSystemId !== null;

  const triggerButton = (
    <button
      className={cn(
        "flex items-center gap-1.5 rounded-lg border px-2.5 py-1 text-xs transition-colors",
        hasScope
          ? "border-transparent bg-primary/10 text-primary"
          : "border-border bg-muted/50 text-muted-foreground hover:bg-muted hover:text-foreground"
      )}
      onClick={() => setOpen((p) => !p)}
      aria-haspopup="listbox"
      aria-expanded={open}
    >
      <Layers className="h-3.5 w-3.5 flex-shrink-0" />
      <span className="max-w-36 truncate">{activeName ?? "All Systems"}</span>
      <ChevronDown className={cn("h-3 w-3 transition-transform", open && "rotate-180")} />
    </button>
  );

  return (
    <div className="relative">
      {hasScope ? (
        <BorderGlow color="#818cf8" size={4}>
          {triggerButton}
        </BorderGlow>
      ) : (
        triggerButton
      )}

      {open && (
        <>
          <div className="fixed inset-0 z-10" onClick={() => setOpen(false)} aria-hidden />
          <div
            role="listbox"
            className="absolute right-0 top-full z-20 mt-1 min-w-52 rounded-lg border border-border bg-card py-1 shadow-lg"
          >
            <button
              role="option"
              aria-selected={!activeSystemId}
              className={cn(
                "flex w-full items-center px-3 py-1.5 text-xs transition-colors hover:bg-muted",
                !activeSystemId && "font-medium text-primary"
              )}
              onClick={() => {
                setActiveSystemId(null);
                setOpen(false);
              }}
            >
              All Systems
            </button>

            {activeSystems.length > 0 && <div className="my-1 border-t border-border" />}

            {activeSystems.map((s) => (
              <button
                key={s.id}
                role="option"
                aria-selected={activeSystemId === s.id}
                className={cn(
                  "flex w-full items-center px-3 py-1.5 text-xs transition-colors hover:bg-muted",
                  activeSystemId === s.id && "font-medium text-primary"
                )}
                onClick={() => {
                  setActiveSystemId(s.id);
                  setOpen(false);
                }}
              >
                {s.name}
              </button>
            ))}
          </div>
        </>
      )}
    </div>
  );
}

// ─── User menu ────────────────────────────────────────────────────────────────

function UserMenu() {
  const { identity, logout } = useAuth();
  const [open, setOpen] = useState(false);

  const initials = identity?.name
    ? identity.name
        .split(" ")
        .map((p) => p[0])
        .slice(0, 2)
        .join("")
        .toUpperCase()
    : (identity?.email?.[0]?.toUpperCase() ?? "?");

  const displayName = identity?.name ?? identity?.email ?? identity?.sub ?? "Unknown";
  const primaryRole = identity?.roles[0];

  return (
    <div className="relative">
      <button
        className="flex items-center gap-2 rounded-lg px-2 py-1 text-xs text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
        onClick={() => setOpen((p) => !p)}
        aria-label="User menu"
        aria-haspopup="menu"
        aria-expanded={open}
      >
        <span className="flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-full bg-primary/20 text-xs font-semibold text-primary">
          {initials}
        </span>
        <span className="hidden max-w-36 truncate sm:block">{displayName}</span>
        <ChevronDown className={cn("h-3 w-3 transition-transform", open && "rotate-180")} />
      </button>

      {open && (
        <>
          <div className="fixed inset-0 z-10" onClick={() => setOpen(false)} aria-hidden />
          <div
            role="menu"
            className="absolute right-0 top-full z-20 mt-1 min-w-52 rounded-lg border border-border bg-card py-1 shadow-lg"
          >
            <div className="px-3 py-2">
              <p className="truncate text-xs font-medium text-foreground">{displayName}</p>
              {primaryRole && (
                <p className="text-xs capitalize text-muted-foreground">{primaryRole}</p>
              )}
            </div>
            <div className="border-t border-border" />
            <button
              role="menuitem"
              className="flex w-full items-center gap-2 px-3 py-2 text-xs text-destructive transition-colors hover:bg-destructive/10"
              onClick={() => {
                setOpen(false);
                void logout();
              }}
            >
              <LogOut className="h-3.5 w-3.5" />
              Sign out
            </button>
          </div>
        </>
      )}
    </div>
  );
}

// ─── TopBar ───────────────────────────────────────────────────────────────────

export function TopBar() {
  const { isPlatformAdmin } = useRole();

  return (
    <header className="flex h-14 items-center gap-4 border-b border-border bg-card px-6">
      <GradientText
        className="text-xl tracking-tight"
        colors={["#818cf8", "#a78bfa", "#38bdf8", "#818cf8"]}
        animationSpeed={6}
      >
        OAD
      </GradientText>
      <span className="text-sm text-muted-foreground">Open Authoritative Directory</span>

      <div className="flex-1" />

      {isPlatformAdmin && <SystemScopeSelector />}
      <UserMenu />
    </header>
  );
}
