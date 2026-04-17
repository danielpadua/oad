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
import { Dock } from "@/components/reactbits";
import { cn } from "@/lib/utils";

interface NavItem {
  to: string;
  icon: React.ComponentType<{ className?: string }>;
  label: string;
}

const navItems: NavItem[] = [
  { to: "/", icon: LayoutDashboard, label: "Dashboard" },
  { to: "/entity-types", icon: FolderTree, label: "Entity Types" },
  { to: "/systems", icon: Network, label: "Systems" },
  { to: "/entities", icon: Users, label: "Entities" },
  { to: "/overlays", icon: Layers, label: "Overlays" },
  { to: "/webhooks", icon: Webhook, label: "Webhooks" },
  { to: "/audit", icon: ScrollText, label: "Audit Log" },
  { to: "/settings", icon: Settings, label: "Settings" },
];

export function Sidebar() {
  return (
    <aside className="flex w-16 flex-col items-center border-r border-border bg-card py-4">
      <Dock orientation="vertical" magnification={52} distance={100} className="gap-1">
        {navItems.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.to === "/"}
            title={item.label}
            className={({ isActive }) =>
              cn(
                "flex h-full w-full items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground",
                isActive &&
                  "bg-primary text-primary-foreground hover:bg-primary/90 hover:text-primary-foreground"
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
