import { renderHook } from "@testing-library/react";
import { vi } from "vitest";
import { useRole, RequireRole, RequireAnyRole } from "@/components/auth/RoleGate";
import { render, screen } from "@testing-library/react";
import { makeIdentity } from "@/test/utils";

// ─── Mock AuthContext ─────────────────────────────────────────────────────────

vi.mock("@/contexts/AuthContext", () => ({
  useAuth: vi.fn(),
}));

import { useAuth } from "@/contexts/AuthContext";

function mockAuth(roles: string[], systemId: string | null = null) {
  vi.mocked(useAuth).mockReturnValue({
    isAuthenticated: true,
    isLoading: false,
    identity: makeIdentity({ roles, systemId }),
    login: vi.fn(),
    logout: vi.fn(),
  });
}

// ─── useRole ──────────────────────────────────────────────────────────────────

describe("useRole", () => {
  it("hasRole returns true for a matching role", () => {
    mockAuth(["editor"]);
    const { result } = renderHook(() => useRole());
    expect(result.current.hasRole("editor")).toBe(true);
  });

  it("hasRole returns false for a missing role", () => {
    mockAuth(["viewer"]);
    const { result } = renderHook(() => useRole());
    expect(result.current.hasRole("admin")).toBe(false);
  });

  it("hasAnyRole returns true when at least one role matches", () => {
    mockAuth(["editor"]);
    const { result } = renderHook(() => useRole());
    expect(result.current.hasAnyRole(["admin", "editor"])).toBe(true);
  });

  it("hasAnyRole returns false when none match", () => {
    mockAuth(["viewer"]);
    const { result } = renderHook(() => useRole());
    expect(result.current.hasAnyRole(["admin", "editor"])).toBe(false);
  });

  it("canWrite is true for admin", () => {
    mockAuth(["admin"]);
    const { result } = renderHook(() => useRole());
    expect(result.current.canWrite).toBe(true);
  });

  it("canWrite is true for editor", () => {
    mockAuth(["editor"]);
    const { result } = renderHook(() => useRole());
    expect(result.current.canWrite).toBe(true);
  });

  it("canWrite is false for viewer", () => {
    mockAuth(["viewer"]);
    const { result } = renderHook(() => useRole());
    expect(result.current.canWrite).toBe(false);
  });

  it("canDelete is true only for admin", () => {
    mockAuth(["admin"]);
    const { result: admin } = renderHook(() => useRole());
    expect(admin.current.canDelete).toBe(true);

    mockAuth(["editor"]);
    const { result: editor } = renderHook(() => useRole());
    expect(editor.current.canDelete).toBe(false);
  });

  it("isPlatformAdmin is true when systemId is null", () => {
    mockAuth(["admin"], null);
    const { result } = renderHook(() => useRole());
    expect(result.current.isPlatformAdmin).toBe(true);
  });

  it("isPlatformAdmin is false when systemId is set", () => {
    mockAuth(["admin"], "sys-1");
    const { result } = renderHook(() => useRole());
    expect(result.current.isPlatformAdmin).toBe(false);
  });

  it("returns empty roles when identity is null", () => {
    vi.mocked(useAuth).mockReturnValue({
      isAuthenticated: false,
      isLoading: false,
      identity: null,
      login: vi.fn(),
      logout: vi.fn(),
    });
    const { result } = renderHook(() => useRole());
    expect(result.current.hasRole("admin")).toBe(false);
    expect(result.current.canWrite).toBe(false);
    expect(result.current.isPlatformAdmin).toBe(false);
  });
});

// ─── RequireRole ──────────────────────────────────────────────────────────────

describe("RequireRole", () => {
  it("renders children when role matches", () => {
    mockAuth(["admin"]);
    render(<RequireRole role="admin"><span>secret</span></RequireRole>);
    expect(screen.getByText("secret")).toBeInTheDocument();
  });

  it("renders nothing by default when role does not match", () => {
    mockAuth(["viewer"]);
    const { container } = render(
      <RequireRole role="admin"><span>secret</span></RequireRole>,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("renders fallback when role does not match", () => {
    mockAuth(["viewer"]);
    render(
      <RequireRole role="admin" fallback={<span>no access</span>}>
        <span>secret</span>
      </RequireRole>,
    );
    expect(screen.queryByText("secret")).not.toBeInTheDocument();
    expect(screen.getByText("no access")).toBeInTheDocument();
  });
});

// ─── RequireAnyRole ───────────────────────────────────────────────────────────

describe("RequireAnyRole", () => {
  it("renders children when any role matches", () => {
    mockAuth(["editor"]);
    render(
      <RequireAnyRole roles={["admin", "editor"]}><span>allowed</span></RequireAnyRole>,
    );
    expect(screen.getByText("allowed")).toBeInTheDocument();
  });

  it("renders fallback when no role matches", () => {
    mockAuth(["viewer"]);
    render(
      <RequireAnyRole roles={["admin", "editor"]} fallback={<span>denied</span>}>
        <span>allowed</span>
      </RequireAnyRole>,
    );
    expect(screen.queryByText("allowed")).not.toBeInTheDocument();
    expect(screen.getByText("denied")).toBeInTheDocument();
  });
});
