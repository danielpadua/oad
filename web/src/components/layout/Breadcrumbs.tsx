import { Link, useLocation } from "react-router-dom";
import { ChevronRight, Home } from "lucide-react";
import { cn } from "@/lib/utils";

const LABELS: Record<string, string> = {
  "entity-types": "Entity Types",
  systems: "Systems",
  entities: "Entities",
  overlays: "Overlays",
  webhooks: "Webhooks",
  audit: "Audit Log",
  settings: "Settings",
};

export function Breadcrumbs() {
  const location = useLocation();
  const segments = location.pathname.split("/").filter(Boolean);

  if (segments.length === 0) return null;

  const crumbs = segments.map((seg, i) => ({
    label: LABELS[seg] ?? seg,
    to: "/" + segments.slice(0, i + 1).join("/"),
    isLast: i === segments.length - 1,
  }));

  return (
    <nav aria-label="Breadcrumb" className="flex items-center gap-1 text-sm text-muted-foreground">
      <Link to="/" className="flex items-center hover:text-foreground transition-colors">
        <Home className="h-3.5 w-3.5" />
        <span className="sr-only">Home</span>
      </Link>
      {crumbs.map((crumb) => (
        <span key={crumb.to} className="flex items-center gap-1">
          <ChevronRight className="h-3.5 w-3.5 flex-shrink-0" />
          {crumb.isLast ? (
            <span className={cn("font-medium text-foreground")}>{crumb.label}</span>
          ) : (
            <Link to={crumb.to} className="hover:text-foreground transition-colors">
              {crumb.label}
            </Link>
          )}
        </span>
      ))}
    </nav>
  );
}
