import { type ReactNode } from "react";
import { render, type RenderOptions } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import type { AuthIdentity } from "@/contexts/AuthContext";

// ─── Auth mock factory ────────────────────────────────────────────────────────

export function makeIdentity(overrides?: Partial<AuthIdentity>): AuthIdentity {
  return {
    sub: "test-user",
    email: "test@example.com",
    name: "Test User",
    roles: ["admin"],
    systemId: null,
    ...overrides,
  };
}

// ─── QueryClient factory ──────────────────────────────────────────────────────

export function makeQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
}

// ─── Render helpers ───────────────────────────────────────────────────────────

interface WrapperOptions {
  queryClient?: QueryClient;
  initialEntries?: string[];
}

function createWrapper({ queryClient, initialEntries = ["/"] }: WrapperOptions = {}) {
  const qc = queryClient ?? makeQueryClient();
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={qc}>
        <MemoryRouter initialEntries={initialEntries}>{children}</MemoryRouter>
      </QueryClientProvider>
    );
  };
}

export function renderWithProviders(
  ui: ReactNode,
  options?: WrapperOptions & Omit<RenderOptions, "wrapper">,
) {
  const { queryClient, initialEntries, ...renderOptions } = options ?? {};
  const wrapper = createWrapper({ queryClient, initialEntries });
  return render(ui, { wrapper, ...renderOptions });
}
