import { lazy, Suspense } from "react";
import { createBrowserRouter, type RouteObject } from "react-router-dom";
import { AppShell } from "@/components/layout/AppShell";
import { AnimatedContent } from "@/components/reactbits";

// Lazy-loaded route pages
const Dashboard = lazy(() => import("@/pages/Dashboard"));

/** Wraps a page component with the AnimatedContent transition. */
function PageTransition({ children }: { children: React.ReactNode }) {
  return (
    <AnimatedContent distance={24} direction="up" duration={0.35}>
      {children}
    </AnimatedContent>
  );
}

/** Minimal fallback shown while a lazy chunk loads. */
function PageLoader() {
  return (
    <div className="flex h-full items-center justify-center">
      <div className="h-8 w-8 animate-spin rounded-full border-4 border-border border-t-primary" />
    </div>
  );
}

const protectedRoutes: RouteObject[] = [
  {
    index: true,
    element: (
      <Suspense fallback={<PageLoader />}>
        <PageTransition>
          <Dashboard />
        </PageTransition>
      </Suspense>
    ),
  },
];

export const router = createBrowserRouter([
  {
    path: "/",
    element: <AppShell />,
    children: protectedRoutes,
  },
]);
