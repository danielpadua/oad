import { useState } from "react";
import { Outlet } from "react-router-dom";
import { X } from "lucide-react";
import { TopBar } from "./TopBar";
import { Sidebar } from "./Sidebar";
import { Breadcrumbs } from "./Breadcrumbs";
import { ScopeBanner } from "./ScopeBanner";
import { cn } from "@/lib/utils";

export function AppShell() {
  const [sidebarOpen, setSidebarOpen] = useState(false);

  return (
    <div className="flex h-screen flex-col overflow-hidden bg-background">
      <TopBar onMenuToggle={() => setSidebarOpen((p) => !p)} />
      <ScopeBanner />

      <div className="flex flex-1 overflow-hidden">
        {/* Mobile sidebar overlay */}
        {sidebarOpen && (
          <div
            className="fixed inset-0 z-20 bg-black/50 md:hidden"
            onClick={() => setSidebarOpen(false)}
            aria-hidden
          />
        )}

        {/* Sidebar — always visible on md+, slide-in on mobile */}
        <div
          className={cn(
            "fixed inset-y-0 left-0 z-30 transition-transform duration-200 md:static md:translate-x-0",
            sidebarOpen ? "translate-x-0" : "-translate-x-full",
          )}
        >
          <div className="flex h-full flex-col">
            {/* Mobile close button */}
            <button
              className="flex items-center justify-end p-3 md:hidden"
              onClick={() => setSidebarOpen(false)}
              aria-label="Close navigation"
            >
              <X className="size-5 text-muted-foreground" />
            </button>
            <Sidebar onNavigate={() => setSidebarOpen(false)} />
          </div>
        </div>

        <main className="flex flex-1 flex-col overflow-y-auto">
          <div className="border-b border-border px-4 py-2 sm:px-6">
            <Breadcrumbs />
          </div>
          <div className="flex-1 p-4 sm:p-6">
            <Outlet />
          </div>
        </main>
      </div>
    </div>
  );
}
