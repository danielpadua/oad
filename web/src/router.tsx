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
const EntityTypes = lazy(() => import("@/pages/EntityTypes"));
const EntityTypeDetail = lazy(() => import("@/pages/EntityTypeDetail"));
const EntityTypeFormPage = lazy(() => import("@/pages/EntityTypeFormPage"));
const Systems = lazy(() => import("@/pages/Systems"));
const SystemDetail = lazy(() => import("@/pages/SystemDetail"));
const OverlaySchemaFormPage = lazy(() => import("@/pages/OverlaySchemaFormPage"));
const Entities = lazy(() => import("@/pages/Entities"));
const EntityDetail = lazy(() => import("@/pages/EntityDetail"));
const EntityFormPage = lazy(() => import("@/pages/EntityFormPage"));
const Overlays = lazy(() => import("@/pages/Overlays"));
const AuditLog = lazy(() => import("@/pages/AuditLog"));
const Webhooks = lazy(() => import("@/pages/Webhooks"));

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

function Protected({ children }: { children: React.ReactNode }) {
  return (
    <Suspense fallback={<PageLoader />}>
      <ProtectedRoute>
        <PageTransition>{children}</PageTransition>
      </ProtectedRoute>
    </Suspense>
  );
}

const protectedRoutes: RouteObject[] = [
  {
    index: true,
    element: (
      <Protected>
        <Dashboard />
      </Protected>
    ),
  },
  {
    path: "entity-types",
    element: (
      <Protected>
        <EntityTypes />
      </Protected>
    ),
  },
  {
    path: "entity-types/new",
    element: (
      <Protected>
        <EntityTypeFormPage />
      </Protected>
    ),
  },
  {
    path: "entity-types/:id",
    element: (
      <Protected>
        <EntityTypeDetail />
      </Protected>
    ),
  },
  {
    path: "entity-types/:id/edit",
    element: (
      <Protected>
        <EntityTypeFormPage />
      </Protected>
    ),
  },
  {
    path: "systems",
    element: (
      <Protected>
        <Systems />
      </Protected>
    ),
  },
  {
    path: "systems/:id",
    element: (
      <Protected>
        <SystemDetail />
      </Protected>
    ),
  },
  {
    path: "systems/:id/overlay-schemas/new",
    element: (
      <Protected>
        <OverlaySchemaFormPage />
      </Protected>
    ),
  },
  {
    path: "systems/:id/overlay-schemas/:schemaId/edit",
    element: (
      <Protected>
        <OverlaySchemaFormPage />
      </Protected>
    ),
  },
  {
    path: "entities",
    element: (
      <Protected>
        <Entities />
      </Protected>
    ),
  },
  {
    path: "entities/new",
    element: (
      <Protected>
        <EntityFormPage />
      </Protected>
    ),
  },
  {
    path: "entities/:id",
    element: (
      <Protected>
        <EntityDetail />
      </Protected>
    ),
  },
  {
    path: "entities/:id/edit",
    element: (
      <Protected>
        <EntityFormPage />
      </Protected>
    ),
  },
  {
    path: "overlays",
    element: (
      <Protected>
        <Overlays />
      </Protected>
    ),
  },
  {
    path: "audit",
    element: (
      <Protected>
        <AuditLog />
      </Protected>
    ),
  },
  {
    path: "webhooks",
    element: (
      <Protected>
        <Webhooks />
      </Protected>
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
