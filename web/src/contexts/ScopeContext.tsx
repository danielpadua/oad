import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { useAuth } from "@/contexts/AuthContext";

// ─── Types ────────────────────────────────────────────────────────────────────

interface ScopeContextValue {
  /** Currently active system UUID, or null for unrestricted (all systems) view. */
  activeSystemId: string | null;
  /** Only callable by platform admins (identity.systemId === null). No-op otherwise. */
  setActiveSystemId: (id: string | null) => void;
}

// ─── Context ──────────────────────────────────────────────────────────────────

const ScopeContext = createContext<ScopeContextValue | null>(null);

// ─── Provider ─────────────────────────────────────────────────────────────────

export function ScopeProvider({ children }: { children: ReactNode }) {
  const { identity } = useAuth();

  // Platform admins (systemId === null) start with no filter; scoped users are fixed.
  const [activeSystemId, setActiveSystemIdRaw] = useState<string | null>(
    identity?.systemId ?? null
  );

  // Sync when identity changes (login / role switch).
  useEffect(() => {
    if (identity !== null) {
      setActiveSystemIdRaw(identity.systemId);
    }
  }, [identity?.systemId]);

  const setActiveSystemId = (id: string | null) => {
    if (identity?.systemId === null) {
      setActiveSystemIdRaw(id);
    }
  };

  return (
    <ScopeContext.Provider value={{ activeSystemId, setActiveSystemId }}>
      {children}
    </ScopeContext.Provider>
  );
}

// ─── Hook ─────────────────────────────────────────────────────────────────────

export function useScope(): ScopeContextValue {
  const ctx = useContext(ScopeContext);
  if (!ctx) throw new Error("useScope must be used inside <ScopeProvider>");
  return ctx;
}
