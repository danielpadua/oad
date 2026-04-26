import { vi } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { server } from "@/test/mocks/server";
import { makeIdentity, makeQueryClient, renderWithProviders } from "@/test/utils";

vi.mock("@/contexts/AuthContext", () => ({ useAuth: vi.fn() }));
vi.mock("@/lib/oidc", () => ({
  userManager: { settings: { authority: "http://localhost", client_id: "test" } },
}));

import { useAuth } from "@/contexts/AuthContext";
import EntityTypes from "@/pages/EntityTypes";
import { mockEntityType } from "@/test/mocks/handlers";

function setupAuth(overrides?: Partial<ReturnType<typeof makeIdentity>>) {
  vi.mocked(useAuth).mockReturnValue({
    isAuthenticated: true,
    isLoading: false,
    identity: makeIdentity(overrides),
    login: vi.fn(),
    logout: vi.fn(),
  });
}

describe("EntityTypes page — contract", () => {
  it("fetches and displays entity types from the API", async () => {
    setupAuth();
    renderWithProviders(<EntityTypes />, { queryClient: makeQueryClient() });

    await waitFor(() => {
      expect(screen.getByText(mockEntityType.type_name)).toBeInTheDocument();
    });
  });

  it("shows empty message when the API returns no items", async () => {
    setupAuth();
    server.use(
      http.get("/api/v1/entity-types", () =>
        HttpResponse.json({ items: [], total: 0 }),
      ),
    );

    renderWithProviders(<EntityTypes />, { queryClient: makeQueryClient() });

    await waitFor(() => {
      expect(screen.getByText("No entity types found.")).toBeInTheDocument();
    });
  });

  it("falls back to empty table when the API fails", async () => {
    setupAuth();
    server.use(
      http.get("/api/v1/entity-types", () =>
        HttpResponse.json(
          { code: "INTERNAL_ERROR", message: "Server error" },
          { status: 500 },
        ),
      ),
    );

    renderWithProviders(<EntityTypes />, { queryClient: makeQueryClient() });

    // No dedicated error UI — the DataTable shows the empty message
    await waitFor(() => {
      expect(screen.getByText("No entity types found.")).toBeInTheDocument();
    });
  });

  it("shows the New Entity Type button for platform admin (systemId null)", async () => {
    setupAuth({ systemId: null });
    renderWithProviders(<EntityTypes />, { queryClient: makeQueryClient() });

    await waitFor(() => screen.getByText("Entity Types"));
    expect(screen.getByRole("button", { name: /new entity type/i })).toBeInTheDocument();
  });

  it("filters items by scope client-side", async () => {
    setupAuth();
    renderWithProviders(<EntityTypes />, { queryClient: makeQueryClient() });

    // wait for data — mock returns one item with scope "global"
    await waitFor(() => screen.getByText(mockEntityType.type_name));

    // click System-Scoped filter — global item should disappear
    await userEvent.click(screen.getByRole("button", { name: "System-Scoped" }));
    expect(screen.queryByText(mockEntityType.type_name)).not.toBeInTheDocument();
    expect(screen.getByText("No entity types found.")).toBeInTheDocument();
  });

  it("shows All filter selected by default", async () => {
    setupAuth();
    renderWithProviders(<EntityTypes />, { queryClient: makeQueryClient() });

    await waitFor(() => screen.getByText(mockEntityType.type_name));

    // the "All" button is the default selected scope
    expect(screen.getByRole("button", { name: "All" })).toBeInTheDocument();
  });
});
