import { useState } from "react"
import { useQuery } from "@tanstack/react-query"
import {
  ScrollText,
  Filter,
  ChevronRight,
  Database,
  FileSearch,
} from "lucide-react"

import { http } from "@/lib/http-client"
import type { HttpError } from "@/lib/http-client"
import type { AuditLogEntry, PaginatedResponse } from "@/lib/types"
import { useScope } from "@/contexts/ScopeContext"
import { FadeContent, AnimatedList } from "@/components/reactbits"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Drawer } from "@/components/ui/drawer"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"

// ─── Helpers ─────────────────────────────────────────────────────────────────

function defaultSince(): string {
  const d = new Date()
  d.setDate(d.getDate() - 7)
  return d.toISOString().slice(0, 16)
}

function operationVariant(
  op: string
): "default" | "secondary" | "destructive" | "outline" {
  if (op === "create") return "default"
  if (op === "update") return "secondary"
  if (op === "delete") return "destructive"
  return "outline"
}

function formatTs(ts: string) {
  return new Date(ts).toLocaleString()
}

// ─── Diff renderer ────────────────────────────────────────────────────────────

type DiffStatus = "added" | "removed" | "modified" | "unchanged"

interface DiffEntry {
  key: string
  status: DiffStatus
  beforeVal: unknown
  afterVal: unknown
}

function computeDiff(
  before: Record<string, unknown> | null,
  after: Record<string, unknown> | null
): DiffEntry[] {
  const allKeys = new Set([
    ...Object.keys(before ?? {}),
    ...Object.keys(after ?? {}),
  ])
  const result: DiffEntry[] = []
  for (const key of allKeys) {
    const bVal = (before ?? {})[key]
    const aVal = (after ?? {})[key]
    const inBefore = before != null && key in before
    const inAfter = after != null && key in after
    if (!inBefore) {
      result.push({ key, status: "added", beforeVal: undefined, afterVal: aVal })
    } else if (!inAfter) {
      result.push({ key, status: "removed", beforeVal: bVal, afterVal: undefined })
    } else if (JSON.stringify(bVal) !== JSON.stringify(aVal)) {
      result.push({ key, status: "modified", beforeVal: bVal, afterVal: aVal })
    } else {
      result.push({ key, status: "unchanged", beforeVal: bVal, afterVal: aVal })
    }
  }
  return result
}

const diffStatusStyles: Record<DiffStatus, string> = {
  added: "bg-green-500/10 border-green-500/30 text-green-700 dark:text-green-400",
  removed: "bg-red-500/10 border-red-500/30 text-red-700 dark:text-red-400",
  modified: "bg-yellow-500/10 border-yellow-500/30 text-yellow-700 dark:text-yellow-400",
  unchanged: "bg-muted/30 border-border text-foreground",
}

const diffStatusLabel: Record<DiffStatus, string> = {
  added: "+ added",
  removed: "− removed",
  modified: "~ modified",
  unchanged: "= unchanged",
}

function DiffRenderer({
  before,
  after,
}: {
  before: Record<string, unknown> | null
  after: Record<string, unknown> | null
}) {
  const diffs = computeDiff(before, after)

  if (diffs.length === 0) {
    if (before == null && after == null) {
      return (
        <p className="text-sm text-muted-foreground">No value recorded.</p>
      )
    }
    // Primitive or array at root — show raw JSON
    return (
      <div className="grid gap-3 sm:grid-cols-2">
        <div>
          <p className="mb-1 text-xs font-medium text-muted-foreground">Before</p>
          <pre className="rounded-lg border border-border bg-muted/30 p-3 font-mono text-xs whitespace-pre-wrap break-all">
            {before != null ? JSON.stringify(before, null, 2) : "—"}
          </pre>
        </div>
        <div>
          <p className="mb-1 text-xs font-medium text-muted-foreground">After</p>
          <pre className="rounded-lg border border-border bg-muted/30 p-3 font-mono text-xs whitespace-pre-wrap break-all">
            {after != null ? JSON.stringify(after, null, 2) : "—"}
          </pre>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-2">
      {diffs.map(({ key, status, beforeVal, afterVal }) => (
        <div
          key={key}
          className={`rounded-lg border px-3 py-2 ${diffStatusStyles[status]}`}
        >
          <div className="flex items-center justify-between gap-2">
            <span className="font-mono text-xs font-semibold">{key}</span>
            <span className="text-xs opacity-70">{diffStatusLabel[status]}</span>
          </div>
          {status === "modified" ? (
            <div className="mt-1.5 grid gap-1 sm:grid-cols-2">
              <div>
                <p className="mb-0.5 text-xs opacity-60">Before</p>
                <pre className="font-mono text-xs whitespace-pre-wrap break-all opacity-80">
                  {JSON.stringify(beforeVal, null, 2)}
                </pre>
              </div>
              <div>
                <p className="mb-0.5 text-xs opacity-60">After</p>
                <pre className="font-mono text-xs whitespace-pre-wrap break-all">
                  {JSON.stringify(afterVal, null, 2)}
                </pre>
              </div>
            </div>
          ) : status !== "unchanged" ? (
            <pre className="mt-1 font-mono text-xs whitespace-pre-wrap break-all opacity-80">
              {JSON.stringify(status === "removed" ? beforeVal : afterVal, null, 2)}
            </pre>
          ) : (
            <pre className="mt-1 font-mono text-xs whitespace-pre-wrap break-all opacity-60">
              {JSON.stringify(beforeVal, null, 2)}
            </pre>
          )}
        </div>
      ))}
    </div>
  )
}

// ─── Audit entry card ─────────────────────────────────────────────────────────

interface AuditEntryCardProps {
  entry: AuditLogEntry
  onClick: () => void
}

function AuditEntryCard({ entry, onClick }: AuditEntryCardProps) {
  return (
    <button
      className="w-full rounded-lg border border-border bg-card px-4 py-3 text-left transition-colors hover:bg-accent/30"
      onClick={onClick}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 flex-1 space-y-1">
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant={operationVariant(entry.operation)} className="capitalize text-xs">
              {entry.operation}
            </Badge>
            <span className="font-mono text-sm font-medium">
              {entry.resource_type}
            </span>
            {entry.system_id && (
              <Badge variant="outline" className="font-mono text-xs">
                scoped
              </Badge>
            )}
          </div>
          <div className="flex flex-wrap gap-x-4 gap-y-0.5 text-xs text-muted-foreground">
            <span>
              <span className="font-medium">Actor:</span> {entry.actor}
            </span>
            <span>
              <span className="font-medium">ID:</span>{" "}
              <span className="font-mono">{entry.resource_id.slice(0, 8)}…</span>
            </span>
            <span>{formatTs(entry.timestamp)}</span>
          </div>
        </div>
        <ChevronRight className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
      </div>
    </button>
  )
}

// ─── Detail drawer ────────────────────────────────────────────────────────────

interface DetailDrawerProps {
  entry: AuditLogEntry | null
  onClose: () => void
}

function DetailDrawer({ entry, onClose }: DetailDrawerProps) {
  return (
    <Drawer
      open={!!entry}
      onOpenChange={(o) => !o && onClose()}
      title="Audit Entry Detail"
      width="lg"
    >
      {entry && (
        <div className="space-y-5 pt-1">
          {/* Metadata */}
          <div className="rounded-lg border border-border bg-muted/30 px-4 py-3 space-y-2">
            <div className="flex flex-wrap gap-x-6 gap-y-1 text-sm">
              <div>
                <span className="text-xs text-muted-foreground">ID</span>
                <p className="font-mono text-xs">{entry.id}</p>
              </div>
              <div>
                <span className="text-xs text-muted-foreground">Resource ID</span>
                <p className="font-mono text-xs">{entry.resource_id}</p>
              </div>
              <div>
                <span className="text-xs text-muted-foreground">Resource Type</span>
                <p className="font-mono text-sm">{entry.resource_type}</p>
              </div>
              <div>
                <span className="text-xs text-muted-foreground">Operation</span>
                <div className="mt-0.5">
                  <Badge variant={operationVariant(entry.operation)} className="capitalize text-xs">
                    {entry.operation}
                  </Badge>
                </div>
              </div>
              <div>
                <span className="text-xs text-muted-foreground">Actor</span>
                <p className="text-sm">{entry.actor}</p>
              </div>
              <div>
                <span className="text-xs text-muted-foreground">Timestamp</span>
                <p className="text-sm">{formatTs(entry.timestamp)}</p>
              </div>
              {entry.system_id && (
                <div>
                  <span className="text-xs text-muted-foreground">System ID</span>
                  <p className="font-mono text-xs">{entry.system_id}</p>
                </div>
              )}
            </div>
          </div>

          {/* Diff */}
          <div>
            <h3 className="mb-3 text-sm font-semibold">Changes</h3>
            <DiffRenderer before={entry.before_value} after={entry.after_value} />
          </div>
        </div>
      )}
    </Drawer>
  )
}

// ─── Filters ──────────────────────────────────────────────────────────────────

interface Filters {
  since: string
  actor: string
  operation: string
}

// ─── Queries ──────────────────────────────────────────────────────────────────

function useChangelog(
  filters: Filters,
  systemId: string | null,
  page: number,
  limit: number
) {
  const sinceDate = new Date(filters.since).toISOString()
  const offset = page * limit
  return useQuery<PaginatedResponse<AuditLogEntry>, HttpError>({
    queryKey: ["changelog", sinceDate, systemId, filters.actor, filters.operation, offset, limit],
    queryFn: () => {
      const params = new URLSearchParams({
        since: sinceDate,
        limit: String(limit),
        offset: String(offset),
      })
      if (systemId) params.set("system_id", systemId)
      if (filters.actor) params.set("actor", filters.actor)
      if (filters.operation && filters.operation !== "all") {
        params.set("operation", filters.operation)
      }
      return http.get<PaginatedResponse<AuditLogEntry>>(`/api/v1/changelog?${params}`)
    },
    staleTime: 15_000,
  })
}

// ─── Retrieval Log placeholder ────────────────────────────────────────────────

function RetrievalLogTab() {
  return (
    <div className="space-y-4">
      <div className="rounded-lg border border-border bg-muted/30 p-6 text-center">
        <FileSearch className="mx-auto size-10 text-muted-foreground/50" />
        <p className="mt-3 text-sm font-medium">Retrieval Log — Compliance View</p>
        <p className="mt-1 text-xs text-muted-foreground max-w-md mx-auto">
          Every PDP attribute lookup is recorded to{" "}
          <code className="rounded bg-muted px-1 font-mono">retrieval_log</code>.
          A query interface for this table (FR-AUD-002) is planned for a future release.
        </p>
      </div>

      <div className="rounded-lg border border-border bg-card px-4 py-4 space-y-3">
        <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">
          Captured fields
        </p>
        <div className="grid gap-2 sm:grid-cols-2 text-sm">
          {[
            ["caller_identity", "JWT subject or mTLS CN of the PDP caller"],
            ["query_parameters", "Serialized lookup parameters (type, external_id, filters)"],
            ["returned_refs", "Array of entity UUIDs returned by the query"],
            ["system_id", "System scope under which the query ran"],
            ["timestamp", "RFC 3339 timestamp of the retrieval event"],
          ].map(([field, desc]) => (
            <div key={field} className="space-y-0.5">
              <code className="font-mono text-xs font-medium">{field}</code>
              <p className="text-xs text-muted-foreground">{desc}</p>
            </div>
          ))}
        </div>
      </div>

      <div className="rounded-lg border border-amber-500/30 bg-amber-500/10 px-4 py-3 text-xs text-amber-700 dark:text-amber-400">
        <Database className="inline-block mr-1.5 size-3.5" />
        Records are written best-effort and are immutable at the database level.
        Retrieval log writes never block the attribute lookup response.
      </div>
    </div>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

const LIMIT = 25

export default function AuditLog() {
  const { activeSystemId } = useScope()
  const [activeTab, setActiveTab] = useState<"audit" | "retrieval">("audit")
  const [filters, setFilters] = useState<Filters>({
    since: defaultSince(),
    actor: "",
    operation: "all",
  })
  const [page, setPage] = useState(0)
  const [selectedEntry, setSelectedEntry] = useState<AuditLogEntry | null>(null)

  const { data, isLoading } = useChangelog(filters, activeSystemId, page, LIMIT)
  const entries = data?.items ?? []
  const total = data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / LIMIT))

  function patchFilter<K extends keyof Filters>(key: K, value: Filters[K]) {
    setFilters((prev) => ({ ...prev, [key]: value }))
    setPage(0)
  }

  return (
    <FadeContent duration={0.35} className="space-y-6">
      {/* Header */}
      <div className="flex items-start gap-4">
        <div>
          <div className="flex items-center gap-3">
            <ScrollText className="size-6 text-muted-foreground" />
            <h1 className="text-2xl font-semibold tracking-tight">
              Observability
            </h1>
          </div>
          <p className="mt-1 text-sm text-muted-foreground">
            Audit trail and retrieval logs for compliance and forensic investigation.
          </p>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 border-b border-border">
        {(["audit", "retrieval"] as const).map((tab) => (
          <button
            key={tab}
            onClick={() => setActiveTab(tab)}
            className={`px-4 py-2 text-sm font-medium transition-colors border-b-2 -mb-px ${
              activeTab === tab
                ? "border-primary text-foreground"
                : "border-transparent text-muted-foreground hover:text-foreground"
            }`}
          >
            {tab === "audit" ? "Audit Log" : "Retrieval Log"}
          </button>
        ))}
      </div>

      {activeTab === "retrieval" ? (
        <RetrievalLogTab />
      ) : (
        <>
          {/* Filters */}
          <div className="rounded-lg border border-border bg-card px-4 py-4">
            <div className="mb-3 flex items-center gap-2 text-sm font-medium">
              <Filter className="size-4 text-muted-foreground" />
              Filters
            </div>
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
              <div className="space-y-1.5">
                <Label className="text-xs">Since</Label>
                <Input
                  type="datetime-local"
                  value={filters.since}
                  onChange={(e) => patchFilter("since", e.target.value)}
                  className="text-sm"
                />
              </div>

              <div className="space-y-1.5">
                <Label className="text-xs">Actor</Label>
                <Input
                  placeholder="user@example.com"
                  value={filters.actor}
                  onChange={(e) => patchFilter("actor", e.target.value)}
                  className="text-sm"
                />
              </div>

              <div className="space-y-1.5">
                <Label className="text-xs">Operation</Label>
                <Select
                  value={filters.operation}
                  onValueChange={(v) => patchFilter("operation", v)}
                >
                  <SelectTrigger className="text-sm">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="all">All</SelectItem>
                    <SelectItem value="create">Create</SelectItem>
                    <SelectItem value="update">Update</SelectItem>
                    <SelectItem value="delete">Delete</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-1.5">
                <Label className="text-xs">System scope</Label>
                <div className="flex h-9 items-center rounded-md border border-border bg-muted/30 px-3 text-sm text-muted-foreground">
                  {activeSystemId ? (
                    <span className="font-mono text-xs truncate">{activeSystemId}</span>
                  ) : (
                    "All systems (platform view)"
                  )}
                </div>
              </div>
            </div>
          </div>

          {/* Entries */}
          <div className="space-y-3">
            <div className="flex items-center justify-between text-sm text-muted-foreground">
              <span>
                {isLoading
                  ? "Loading…"
                  : `${total} ${total === 1 ? "entry" : "entries"}`}
              </span>
              <span>
                Page {page + 1} of {totalPages}
              </span>
            </div>

            {isLoading ? (
              <div className="space-y-2">
                {Array.from({ length: 5 }).map((_, i) => (
                  <Skeleton key={i} className="h-16 w-full rounded-lg" />
                ))}
              </div>
            ) : entries.length === 0 ? (
              <div className="rounded-lg border border-dashed border-border py-14 text-center">
                <ScrollText className="mx-auto size-8 text-muted-foreground/50" />
                <p className="mt-3 text-sm font-medium">No audit entries found</p>
                <p className="mt-1 text-xs text-muted-foreground">
                  Try expanding the time range or removing filters.
                </p>
              </div>
            ) : (
              <AnimatedList className="space-y-2">
                {entries.map((entry) => (
                  <AuditEntryCard
                    key={entry.id}
                    entry={entry}
                    onClick={() => setSelectedEntry(entry)}
                  />
                ))}
              </AnimatedList>
            )}

            {/* Pagination */}
            {total > LIMIT && (
              <div className="flex items-center justify-between border-t border-border pt-3">
                <Button
                  variant="outline"
                  size="sm"
                  disabled={page === 0}
                  onClick={() => setPage((p) => Math.max(0, p - 1))}
                >
                  Previous
                </Button>
                <span className="text-xs text-muted-foreground">
                  {page * LIMIT + 1}–{Math.min((page + 1) * LIMIT, total)} of {total}
                </span>
                <Button
                  variant="outline"
                  size="sm"
                  disabled={page >= totalPages - 1}
                  onClick={() => setPage((p) => p + 1)}
                >
                  Next
                </Button>
              </div>
            )}
          </div>
        </>
      )}

      <DetailDrawer
        entry={selectedEntry}
        onClose={() => setSelectedEntry(null)}
      />
    </FadeContent>
  )
}
