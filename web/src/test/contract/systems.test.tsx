import { vi } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { server } from "@/test/mocks/server";
import { makeIdentity, makeQueryClient, renderWithProviders } from "@/test/utils";

vi.mock("@/contexts/AuthContext", () => ({ useAuth: vi.fn() }));
vi.mock("@/lib/oidc", () => ({
  getUserManager: () => ({ settings: { authority: "http://localhost", client_id: "test" } }),
  initUserManager: vi.fn(),
}));

import { useAuth } from "@/contexts/AuthContext";
import Systems from "@/pages/Systems";
import { mockSystem } from "@/test/mocks/handlers";

function setupAuth(overrides?: Partial<ReturnType<typeof makeIdentity>>) {
  vi.mocked(useAuth).mockReturnValue({
    isAuthenticated: true,
    isLoading: false,
    identity: makeIdentity(overrides),
    login: vi.fn(),
    logout: vi.fn(),
  });
}

describe("Systems page — contract", () => {
  it("fetches and displays systems from the API", async () => {
    setupAuth({ systemId: null });
    renderWithProviders(<Systems />, { queryClient: makeQueryClient() });

    await waitFor(() => {
      expect(screen.getByText(mockSystem.name)).toBeInTheDocument();
    });
  });

  it("displays Active badge for active systems", async () => {
    setupAuth({ systemId: null });
    renderWithProviders(<Systems />, { queryClient: makeQueryClient() });

    await waitFor(() => {
      expect(screen.getByText("Active")).toBeInTheDocument();
    });
  });

  it("shows empty state when API returns no systems", async () => {
    setupAuth({ systemId: null });
    server.use(
      http.get("/api/v1/systems", () => HttpResponse.json({ items: [], total: 0 })),
    );

    renderWithProviders(<Systems />, { queryClient: makeQueryClient() });

    await waitFor(() => {
      expect(screen.getByText("No systems registered yet")).toBeInTheDocument();
    });
  });

  it("shows Register System button for platform admin (systemId null)", async () => {
    setupAuth({ systemId: null });
    renderWithProviders(<Systems />, { queryClient: makeQueryClient() });

    await waitFor(() => screen.getByText(mockSystem.name));
    expect(screen.getByRole("button", { name: /register system/i })).toBeInTheDocument();
  });

  it("hides Register System button when user has a system scope (not platform admin)", async () => {
    setupAuth({ systemId: "sys-1" });
    renderWithProviders(<Systems />, { queryClient: makeQueryClient() });

    await waitFor(() => screen.getByText(mockSystem.name));
    expect(screen.queryByRole("button", { name: /register system/i })).not.toBeInTheDocument();
  });

  it("opens register modal when button is clicked", async () => {
    setupAuth({ systemId: null });
    renderWithProviders(<Systems />, { queryClient: makeQueryClient() });

    await waitFor(() => screen.getByRole("button", { name: /register system/i }));
    await userEvent.click(screen.getByRole("button", { name: /register system/i }));

    const dialog = screen.getByRole("dialog");
    expect(dialog).toBeInTheDocument();
    // verify modal contains the registration form fields
    expect(dialog).toHaveTextContent("System Name");
  });

  it("falls back to empty state when API fails", async () => {
    setupAuth({ systemId: null });
    server.use(
      http.get("/api/v1/systems", () =>
        HttpResponse.json(
          { code: "INTERNAL_ERROR", message: "Server error" },
          { status: 500 },
        ),
      ),
    );

    renderWithProviders(<Systems />, { queryClient: makeQueryClient() });

    // No dedicated error UI — falls back to the empty state message
    await waitFor(() => {
      expect(screen.getByText("No systems registered yet")).toBeInTheDocument();
    });
  });
});
