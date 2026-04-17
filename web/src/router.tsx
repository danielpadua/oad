import { lazy, Suspense } from "react";
import { createBrowserRouter, type RouteObject } from "react-router-dom";
import { AppShell } from "@/components/layout/AppShell";
import { AnimatedContent } from "@/components/reactbits";
import { ProtectedRoute } from "@/components/auth/ProtectedRoute";

// ─── Lazy pages ───────────────────────────────────────────────────────────────

const Dashboard = lazy(() => import("@/pages/Dashboard"));
const Login = lazy(() => import("@/pages/Login"));
const Callback = lazy(() => import("@/pages/Callback"));
const SilentRenew = lazy(() => import("@/pages/SilentRenew"));
const Forbidden = lazy(() => import("@/pages/Forbidden"));

// ─── Helpers ──────────────────────────────────────────────────────────────────

function PageTransition({ children }: { children: React.ReactNode }) {
  return (
    <AnimatedContent distance={24} direction="up" duration={0.35}>
      {children}
    </AnimatedContent>
  );
}

function PageLoader() {
  return (
    <div className="flex h-full items-center justify-center">
      <div className="h-8 w-8 animate-spin rounded-full border-4 border-border border-t-primary" />
    </div>
  );
}

// ─── Routes ───────────────────────────────────────────────────────────────────

const protectedRoutes: RouteObject[] = [
  {
    index: true,
    element: (
      <Suspense fallback={<PageLoader />}>
        <ProtectedRoute>
          <PageTransition>
            <Dashboard />
          </PageTransition>
        </ProtectedRoute>
      </Suspense>
    ),
  },
];

export const router = createBrowserRouter([
  // Public auth routes — no AppShell, no ProtectedRoute wrapper.
  {
    path: "/login",
    element: (
      <Suspense fallback={null}>
        <Login />
      </Suspense>
    ),
  },
  {
    path: "/callback",
    element: (
      <Suspense fallback={null}>
        <Callback />
      </Suspense>
    ),
  },
  // Loaded inside an iframe by oidc-client-ts during silent token renewal.
  {
    path: "/silent-renew",
    element: (
      <Suspense fallback={null}>
        <SilentRenew />
      </Suspense>
    ),
  },
  {
    path: "/403",
    element: (
      <Suspense fallback={null}>
        <Forbidden />
      </Suspense>
    ),
  },
  // Protected shell — all authenticated views live here.
  {
    path: "/",
    element: <AppShell />,
    children: protectedRoutes,
  },
]);
