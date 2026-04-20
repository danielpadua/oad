import { useState, useRef } from "react"
import { Link, useNavigate } from "react-router-dom"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { Plus, Upload, Eye, Pencil, Trash2, Search, X } from "lucide-react"
import type { ColumnDef } from "@tanstack/react-table"

import { http } from "@/lib/http-client"
import type { HttpError } from "@/lib/http-client"
import { toastSuccess, toastApiError, toastMutationError } from "@/lib/toast"
import type { Entity, EntityTypeDefinition, ListResponse, PaginatedResponse, BulkImportResult } from "@/lib/types"
import { FadeContent, Stepper, ClickSpark } from "@/components/reactbits"
import type { StepConfig } from "@/components/reactbits"
import { DataTable, DataTableColumnHeader } from "@/components/ui/data-table"
import type { DataTablePaginationState } from "@/components/ui/data-table"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { ConfirmDialog } from "@/components/ui/confirm-dialog"
import { Modal } from "@/components/ui/modal"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"

// ─── Queries ─────────────────────────────────────────────────────────────────

function useEntityTypes() {
  return useQuery<ListResponse<EntityTypeDefinition>, HttpError>({
    queryKey: ["entity-types"],
    queryFn: () => http.get<ListResponse<EntityTypeDefinition>>("/api/v1/entity-types"),
    staleTime: 5 * 60_000,
  })
}

function useEntities(typeFilter: string, pagination: DataTablePaginationState) {
  const params = new URLSearchParams({
    limit: String(pagination.pageSize),
    offset: String(pagination.pageIndex * pagination.pageSize),
  })
  if (typeFilter) params.set("type", typeFilter)

  return useQuery<PaginatedResponse<Entity>, HttpError>({
    queryKey: ["entities", typeFilter, pagination],
    queryFn: () => http.get<PaginatedResponse<Entity>>(`/api/v1/entities?${params.toString()}`),
  })
}

function useDeleteEntity() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => http.del(`/api/v1/entities/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["entities"] })
      toastSuccess("Entity deleted")
    },
    onError: toastApiError,
  })
}

function useBulkImport() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (payload: { entities: unknown[]; mode: string }) =>
      http.post<BulkImportResult>("/api/v1/entities/bulk", payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["entities"] })
    },
    onError: toastMutationError,
  })
}

// ─── Bulk Import Modal ────────────────────────────────────────────────────────

const BULK_STEPS: StepConfig[] = [
  { title: "Upload", description: "Paste or upload a JSON array of entities" },
  { title: "Configure", description: "Select import mode and preview" },
  { title: "Results", description: "Import summary and errors" },
]

interface BulkImportModalProps {
  open: boolean
  onOpenChange: (v: boolean) => void
}

function BulkImportModal({ open, onOpenChange }: BulkImportModalProps) {
  const [step, setStep] = useState(0)
  const [rawJson, setRawJson] = useState("")
  const [parseError, setParseError] = useState<string | null>(null)
  const [parsed, setParsed] = useState<unknown[]>([])
  const [mode, setMode] = useState<"create" | "upsert">("create")
  const [result, setResult] = useState<BulkImportResult | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const bulkMutation = useBulkImport()

  function reset() {
    setStep(0)
    setRawJson("")
    setParseError(null)
    setParsed([])
    setMode("create")
    setResult(null)
  }

  function handleClose(v: boolean) {
    if (!v) reset()
    onOpenChange(v)
  }

  function handleFileLoad(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return
    const reader = new FileReader()
    reader.onload = (ev) => {
      setRawJson(ev.target?.result as string)
      setParseError(null)
    }
    reader.readAsText(file)
  }

  function handleNext() {
    try {
      const data = JSON.parse(rawJson) as unknown
      if (!Array.isArray(data)) {
        setParseError("Input must be a JSON array of entity objects.")
        return
      }
      setParsed(data)
      setParseError(null)
      setStep(1)
    } catch {
      setParseError("Invalid JSON — check the syntax and try again.")
    }
  }

  async function handleImport() {
    const res = await bulkMutation.mutateAsync({ entities: parsed, mode })
    setResult(res)
    setStep(2)
    if (res.errors.length === 0) {
      toastSuccess(`Import complete — ${res.created} created, ${res.updated} updated`)
    } else {
      toastApiError(
        new Error(`Import finished with ${res.errors.length} error(s)`)
      )
    }
  }

  return (
    <Modal
      open={open}
      onOpenChange={handleClose}
      title="Bulk Import Entities"
      size="lg"
    >
      <div className="flex gap-8 pt-2">
        {/* Stepper sidebar */}
        <div className="w-40 shrink-0">
          <Stepper steps={BULK_STEPS} currentStep={step} />
        </div>

        {/* Step content */}
        <div className="min-w-0 flex-1 space-y-4">
          {step === 0 && (
            <>
              <p className="text-sm text-muted-foreground">
                Provide a JSON array of entity objects. Each item must have{" "}
                <code className="rounded bg-muted px-1 text-xs">type</code> and{" "}
                <code className="rounded bg-muted px-1 text-xs">external_id</code>.
              </p>
              <textarea
                className="h-48 w-full resize-none rounded-lg border border-border bg-muted/20 p-3 font-mono text-xs focus:outline-none focus:ring-2 focus:ring-ring"
                placeholder={'[\n  { "type": "user", "external_id": "alice@example.com", "properties": {} }\n]'}
                value={rawJson}
                onChange={(e) => {
                  setRawJson(e.target.value)
                  setParseError(null)
                }}
              />
              <div className="flex items-center gap-3">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => fileInputRef.current?.click()}
                >
                  <Upload className="mr-1.5 size-3.5" />
                  Load from file
                </Button>
                <input
                  ref={fileInputRef}
                  type="file"
                  accept=".json,application/json"
                  className="hidden"
                  onChange={handleFileLoad}
                />
                {rawJson && (
                  <Button variant="ghost" size="sm" onClick={() => setRawJson("")}>
                    <X className="mr-1.5 size-3.5" />
                    Clear
                  </Button>
                )}
              </div>
              {parseError && (
                <p className="text-sm font-medium text-destructive">{parseError}</p>
              )}
            </>
          )}

          {step === 1 && (
            <>
              <div className="rounded-lg border border-border bg-card p-4">
                <p className="text-sm">
                  <span className="font-semibold tabular-nums">{parsed.length}</span>{" "}
                  <span className="text-muted-foreground">
                    {parsed.length === 1 ? "entity" : "entities"} ready to import.
                  </span>
                </p>
              </div>
              <div className="space-y-2">
                <p className="text-sm font-medium">Import mode</p>
                <div className="flex gap-3">
                  {(["create", "upsert"] as const).map((m) => (
                    <button
                      key={m}
                      onClick={() => setMode(m)}
                      className={`cursor-pointer rounded-lg border px-4 py-2 text-sm transition-colors ${
                        mode === m
                          ? "border-primary bg-primary/10 text-primary font-medium"
                          : "border-border bg-background text-muted-foreground hover:bg-muted"
                      }`}
                    >
                      {m === "create" ? "Create only" : "Upsert (create or update)"}
                    </button>
                  ))}
                </div>
                <p className="text-xs text-muted-foreground">
                  {mode === "create"
                    ? "Fails if any entity with the same type + external_id already exists."
                    : "Creates new entities and updates existing ones by type + external_id."}
                </p>
              </div>
            </>
          )}

          {step === 2 && result && (
            <div className="space-y-4">
              <div className="grid grid-cols-3 gap-3">
                {[
                  { label: "Total", value: result.total },
                  { label: "Created", value: result.created },
                  { label: "Updated", value: result.updated },
                ].map(({ label, value }) => (
                  <div
                    key={label}
                    className="rounded-lg border border-border bg-card p-3 text-center"
                  >
                    <p className="text-2xl font-semibold tabular-nums">{value}</p>
                    <p className="text-xs text-muted-foreground">{label}</p>
                  </div>
                ))}
              </div>
              {result.errors.length > 0 && (
                <div className="space-y-1.5">
                  <p className="text-sm font-medium text-destructive">
                    {result.errors.length} error{result.errors.length > 1 ? "s" : ""}
                  </p>
                  <div className="max-h-48 overflow-y-auto rounded-lg border border-destructive/30 bg-destructive/5 p-3 space-y-1">
                    {result.errors.map((e) => (
                      <p key={e.index} className="font-mono text-xs text-destructive">
                        [{e.index}] {e.error}
                      </p>
                    ))}
                  </div>
                </div>
              )}
            </div>
          )}

          {/* Footer actions */}
          <div className="flex items-center justify-between border-t border-border pt-4">
            <Button variant="outline" onClick={() => handleClose(false)}>
              {step === 2 ? "Close" : "Cancel"}
            </Button>
            <div className="flex gap-2">
              {step > 0 && step < 2 && (
                <Button variant="outline" onClick={() => setStep((s) => s - 1)}>
                  Previous
                </Button>
              )}
              {step === 0 && (
                <Button onClick={handleNext} disabled={!rawJson.trim()}>
                  Next
                </Button>
              )}
              {step === 1 && (
                <ClickSpark color="hsl(var(--primary))" disabled={bulkMutation.isPending}>
                  <Button onClick={() => void handleImport()} disabled={bulkMutation.isPending}>
                    {bulkMutation.isPending ? "Importing…" : "Import"}
                  </Button>
                </ClickSpark>
              )}
            </div>
          </div>
        </div>
      </div>
    </Modal>
  )
}

// ─── Columns ─────────────────────────────────────────────────────────────────

function buildColumns(
  onEdit: (e: Entity) => void,
  onDelete: (e: Entity) => void
): ColumnDef<Entity>[] {
  return [
    {
      accessorKey: "external_id",
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title="External ID" />
      ),
      cell: ({ row }) => (
        <Link
          to={`/entities/${row.original.id}`}
          className="font-mono text-sm hover:underline"
        >
          {row.original.external_id}
        </Link>
      ),
    },
    {
      accessorKey: "type",
      header: "Type",
      cell: ({ row }) => (
        <Badge variant="secondary" className="font-mono">
          {row.original.type}
        </Badge>
      ),
    },
    {
      id: "properties",
      header: "Properties",
      cell: ({ row }) => {
        const keys = Object.keys(row.original.properties ?? {})
        if (keys.length === 0) return <span className="text-muted-foreground text-xs">—</span>
        return (
          <span className="text-xs text-muted-foreground">
            {keys.slice(0, 3).join(", ")}
            {keys.length > 3 && ` +${keys.length - 3} more`}
          </span>
        )
      },
    },
    {
      accessorKey: "created_at",
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title="Created" />
      ),
      cell: ({ row }) => new Date(row.original.created_at).toLocaleDateString(),
    },
    {
      id: "actions",
      cell: ({ row }) => (
        <div className="flex items-center gap-1">
          <Button variant="ghost" size="sm" asChild>
            <Link to={`/entities/${row.original.id}`}>
              <Eye className="size-3.5" />
              <span className="sr-only">View</span>
            </Link>
          </Button>
          <Button variant="ghost" size="sm" onClick={() => onEdit(row.original)}>
            <Pencil className="size-3.5" />
            <span className="sr-only">Edit</span>
          </Button>
          <Button variant="ghost" size="sm" onClick={() => onDelete(row.original)}>
            <Trash2 className="size-3.5 text-destructive" />
            <span className="sr-only">Delete</span>
          </Button>
        </div>
      ),
    },
  ]
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function Entities() {
  const navigate = useNavigate()
  const { data: typesData } = useEntityTypes()
  const deleteMutation = useDeleteEntity()

  const [typeFilter, setTypeFilter] = useState("")
  const [search, setSearch] = useState("")
  const [pagination, setPagination] = useState<DataTablePaginationState>({
    pageIndex: 0,
    pageSize: 25,
  })
  const [deleteTarget, setDeleteTarget] = useState<Entity | undefined>()
  const [bulkOpen, setBulkOpen] = useState(false)

  const { data, isLoading } = useEntities(typeFilter, pagination)

  const allItems = data?.items ?? []
  const filtered = search.trim()
    ? allItems.filter((e) =>
        e.external_id.toLowerCase().includes(search.toLowerCase())
      )
    : allItems

  const columns = buildColumns(
    (e) => navigate(`/entities/${e.id}/edit`),
    setDeleteTarget
  )

  function handleTypeFilter(v: string) {
    setTypeFilter(v === "all" ? "" : v)
    setPagination((p) => ({ ...p, pageIndex: 0 }))
  }

  return (
    <FadeContent duration={0.4} className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Entities</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Typed nodes in the authorization graph — subjects, resources, and roles
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => setBulkOpen(true)}>
            <Upload className="mr-1.5 size-4" />
            Bulk Import
          </Button>
          <Button onClick={() => navigate("/entities/new")}>
            <Plus className="mr-1.5 size-4" />
            New Entity
          </Button>
        </div>
      </div>

      {/* Filters */}
      <div className="flex items-center gap-3">
        <Select value={typeFilter || "all"} onValueChange={handleTypeFilter}>
          <SelectTrigger className="w-48">
            <SelectValue placeholder="All types" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All types</SelectItem>
            {typesData?.items.map((t) => (
              <SelectItem key={t.id} value={t.type_name}>
                {t.type_name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <div className="relative flex-1 max-w-xs">
          <Search className="absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder="Search by external ID…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-8"
          />
        </div>

        {!isLoading && (
          <span className="text-sm text-muted-foreground">
            {data?.total ?? 0} {(data?.total ?? 0) === 1 ? "entity" : "entities"}
          </span>
        )}
      </div>

      {/* Table */}
      <DataTable
        columns={columns}
        data={filtered}
        isLoading={isLoading}
        pagination={pagination}
        onPaginationChange={setPagination}
        rowCount={data?.total ?? 0}
        emptyMessage="No entities found."
      />

      {/* Delete confirmation */}
      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(o) => !o && setDeleteTarget(undefined)}
        title={`Delete "${deleteTarget?.external_id}"?`}
        description="This will permanently delete the entity. Relations referencing this entity must be removed first."
        confirmLabel="Delete"
        isLoading={deleteMutation.isPending}
        onConfirm={async () => {
          if (deleteTarget) {
            await deleteMutation.mutateAsync(deleteTarget.id)
            setDeleteTarget(undefined)
          }
        }}
      />

      {/* Bulk import modal */}
      <BulkImportModal open={bulkOpen} onOpenChange={setBulkOpen} />
    </FadeContent>
  )
}
