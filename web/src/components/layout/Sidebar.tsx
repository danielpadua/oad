import { NavLink } from "react-router-dom";
import {
  LayoutDashboard,
  FolderTree,
  Network,
  Layers,
  Users,
  Webhook,
  ScrollText,
  Settings,
} from "lucide-react";
import { useTranslation } from "react-i18next";
import { Dock } from "@/components/reactbits";
import { cn } from "@/lib/utils";

interface SidebarProps {
  onNavigate?: () => void;
}

export function Sidebar({ onNavigate }: SidebarProps) {
  const { t } = useTranslation();

  const navItems = [
    { to: "/", icon: LayoutDashboard, label: t("nav.dashboard") },
    { to: "/entity-types", icon: FolderTree, label: t("nav.entityTypes") },
    { to: "/systems", icon: Network, label: t("nav.systems") },
    { to: "/entities", icon: Users, label: t("nav.entities") },
    { to: "/overlays", icon: Layers, label: t("nav.overlays") },
    { to: "/webhooks", icon: Webhook, label: t("nav.webhooks") },
    { to: "/audit", icon: ScrollText, label: t("nav.auditLog") },
    { to: "/settings", icon: Settings, label: t("nav.settings") },
  ];

  return (
    <aside className="flex w-16 flex-col items-center border-r border-border bg-card py-4 sm:w-16">
      <Dock orientation="vertical" magnification={52} distance={100} className="gap-1">
        {navItems.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.to === "/"}
            title={item.label}
            onClick={onNavigate}
            className={({ isActive }) =>
              cn(
                "flex h-full w-full items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground",
                isActive &&
                  "bg-primary text-primary-foreground hover:bg-primary/90 hover:text-primary-foreground",
              )
            }
          >
            <item.icon className="h-5 w-5 shrink-0" />
            <span className="sr-only">{item.label}</span>
          </NavLink>
        ))}
      </Dock>
    </aside>
  );
}
