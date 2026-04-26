import { render, screen } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router-dom";
import { vi } from "vitest";
import { ProtectedRoute } from "@/components/auth/ProtectedRoute";
import { makeIdentity } from "@/test/utils";

vi.mock("@/contexts/AuthContext", () => ({
  useAuth: vi.fn(),
}));

import { useAuth } from "@/contexts/AuthContext";

describe("ProtectedRoute", () => {
  it("shows a loading spinner while authentication is resolving", () => {
    vi.mocked(useAuth).mockReturnValue({
      isAuthenticated: false,
      isLoading: true,
      identity: null,
      login: vi.fn(),
      logout: vi.fn(),
    });

    render(
      <MemoryRouter initialEntries={["/dashboard"]}>
        <ProtectedRoute>
          <span>dashboard content</span>
        </ProtectedRoute>
      </MemoryRouter>,
    );

    expect(screen.queryByText("dashboard content")).not.toBeInTheDocument();
    // spinner is rendered as a div with animate-spin class
    expect(document.querySelector(".animate-spin")).toBeInTheDocument();
  });

  it("redirects to /login when user is not authenticated", () => {
    vi.mocked(useAuth).mockReturnValue({
      isAuthenticated: false,
      isLoading: false,
      identity: null,
      login: vi.fn(),
      logout: vi.fn(),
    });

    render(
      <MemoryRouter initialEntries={["/dashboard"]}>
        <Routes>
          <Route
            path="/dashboard"
            element={
              <ProtectedRoute>
                <span>dashboard content</span>
              </ProtectedRoute>
            }
          />
          <Route path="/login" element={<span>login page</span>} />
        </Routes>
      </MemoryRouter>,
    );

    expect(screen.queryByText("dashboard content")).not.toBeInTheDocument();
    expect(screen.getByText("login page")).toBeInTheDocument();
  });

  it("renders children when user is authenticated", () => {
    vi.mocked(useAuth).mockReturnValue({
      isAuthenticated: true,
      isLoading: false,
      identity: makeIdentity(),
      login: vi.fn(),
      logout: vi.fn(),
    });

    render(
      <MemoryRouter>
        <ProtectedRoute>
          <span>protected content</span>
        </ProtectedRoute>
      </MemoryRouter>,
    );

    expect(screen.getByText("protected content")).toBeInTheDocument();
  });
});
