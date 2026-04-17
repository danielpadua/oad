import { useQuery } from "@tanstack/react-query";
import { Activity, Database, Globe, Webhook, AlertCircle, CheckCircle2 } from "lucide-react";
import { CountUp, FadeContent, SpotlightCard } from "@/components/reactbits";
import { http, HttpError } from "@/lib/http-client";
import { cn } from "@/lib/utils";

// ─── Types ────────────────────────────────────────────────────────────────────

interface HealthResponse {
  status: "ok" | "degraded";
  database: "ok" | "degraded";
  uptime_seconds: number;
}

interface StatsResponse {
  total_entities: number;
  active_systems: number;
  pending_webhooks: number;
}

// ─── Queries ─────────────────────────────────────────────────────────────────

function useHealth() {
  return useQuery<HealthResponse, HttpError>({
    queryKey: ["health"],
    queryFn: () => http.get<HealthResponse>("/health"),
    refetchInterval: 30_000,
  });
}

// Placeholder until a dedicated /api/v1/stats endpoint exists.
function useStats() {
  return useQuery<StatsResponse, HttpError>({
    queryKey: ["dashboard-stats"],
    queryFn: () =>
      Promise.resolve<StatsResponse>({
        total_entities: 0,
        active_systems: 0,
        pending_webhooks: 0,
      }),
    staleTime: Infinity,
  });
}

// ─── Sub-components ───────────────────────────────────────────────────────────

interface MetricCardProps {
  label: string;
  value: number;
  icon: React.ComponentType<{ className?: string }>;
  suffix?: string;
}

function MetricCard({ label, value, icon: Icon, suffix }: MetricCardProps) {
  return (
    <SpotlightCard className="flex items-start gap-4" spotlightColor="rgba(129,140,248,0.08)">
      <div className="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
        <Icon className="h-5 w-5" />
      </div>
      <div>
        <p className="text-sm text-muted-foreground">{label}</p>
        <p className="mt-0.5 text-2xl font-semibold tracking-tight">
          <CountUp to={value} duration={1.8} />
          {suffix && <span className="ml-1 text-base font-normal text-muted-foreground">{suffix}</span>}
        </p>
      </div>
    </SpotlightCard>
  );
}

interface HealthStatusBadgeProps {
  status: "ok" | "degraded" | undefined;
  loading: boolean;
}

function HealthStatusBadge({ status, loading }: HealthStatusBadgeProps) {
  if (loading) {
    return <span className="text-sm text-muted-foreground">Checking…</span>;
  }

  const isOk = status === "ok";
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium",
        isOk
          ? "bg-green-500/10 text-green-600 dark:text-green-400"
          : "bg-destructive/10 text-destructive"
      )}
    >
      {isOk ? (
        <CheckCircle2 className="h-3.5 w-3.5" />
      ) : (
        <AlertCircle className="h-3.5 w-3.5" />
      )}
      {isOk ? "Healthy" : "Degraded"}
    </span>
  );
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function Dashboard() {
  const { data: health, isLoading: healthLoading } = useHealth();
  const { data: stats, isLoading: statsLoading } = useStats();

  return (
    <FadeContent duration={0.4} className="space-y-8">
      {/* Page header */}
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Dashboard</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Overview of the Open Authoritative Directory
        </p>
      </div>

      {/* Metric cards */}
      <section>
        <h2 className="mb-4 text-sm font-medium uppercase tracking-widest text-muted-foreground">
          Key Metrics
        </h2>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
          <MetricCard
            label="Total Entities"
            value={statsLoading ? 0 : (stats?.total_entities ?? 0)}
            icon={Database}
          />
          <MetricCard
            label="Active Systems"
            value={statsLoading ? 0 : (stats?.active_systems ?? 0)}
            icon={Globe}
          />
          <MetricCard
            label="Pending Webhooks"
            value={statsLoading ? 0 : (stats?.pending_webhooks ?? 0)}
            icon={Webhook}
          />
        </div>
      </section>

      {/* System health */}
      <section>
        <h2 className="mb-4 text-sm font-medium uppercase tracking-widest text-muted-foreground">
          System Health
        </h2>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <SpotlightCard spotlightColor="rgba(56,189,248,0.07)">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-3">
                <Activity className="h-5 w-5 text-muted-foreground" />
                <span className="font-medium">API Server</span>
              </div>
              <HealthStatusBadge status={health?.status} loading={healthLoading} />
            </div>
          </SpotlightCard>

          <SpotlightCard spotlightColor="rgba(56,189,248,0.07)">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-3">
                <Database className="h-5 w-5 text-muted-foreground" />
                <span className="font-medium">Database</span>
              </div>
              <HealthStatusBadge status={health?.database} loading={healthLoading} />
            </div>
          </SpotlightCard>
        </div>
      </section>
    </FadeContent>
  );
}
