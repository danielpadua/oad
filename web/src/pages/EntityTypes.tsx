import { useState } from "react"
import { Link, useNavigate } from "react-router-dom"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { Plus, Eye, Pencil, Trash2 } from "lucide-react"
import type { ColumnDef } from "@tanstack/react-table"

import { http } from "@/lib/http-client"
import type { HttpError } from "@/lib/http-client"
import { toastSuccess, toastApiError } from "@/lib/toast"
import type { EntityTypeDefinition, ListResponse } from "@/lib/types"
import { FadeContent } from "@/components/reactbits"
import { DataTable, DataTableColumnHeader } from "@/components/ui/data-table"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { ConfirmDialog } from "@/components/ui/confirm-dialog"

// ─── Queries ─────────────────────────────────────────────────────────────────

function useEntityTypes() {
  return useQuery<ListResponse<EntityTypeDefinition>, HttpError>({
    queryKey: ["entity-types"],
    queryFn: () =>
      http.get<ListResponse<EntityTypeDefinition>>("/api/v1/entity-types"),
  })
}

function useDeleteEntityType() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => http.del(`/api/v1/entity-types/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["entity-types"] })
      toastSuccess("Entity type deleted")
    },
    onError: toastApiError,
  })
}

// ─── Scope filter ─────────────────────────────────────────────────────────────

type ScopeFilter = "all" | "global" | "system_scoped"

const SCOPE_OPTIONS: { value: ScopeFilter; label: string }[] = [
  { value: "all", label: "All" },
  { value: "global", label: "Global" },
  { value: "system_scoped", label: "System-Scoped" },
]

function ScopeFilterBar({
  value,
  onChange,
}: {
  value: ScopeFilter
  onChange: (v: ScopeFilter) => void
}) {
  return (
    <div className="flex rounded-lg border border-border bg-muted/40 p-0.5">
      {SCOPE_OPTIONS.map((opt) => (
        <button
          key={opt.value}
          onClick={() => onChange(opt.value)}
          className={
            value === opt.value
              ? "cursor-pointer rounded-md bg-background px-3 py-1 text-sm font-medium shadow-sm"
              : "cursor-pointer rounded-md px-3 py-1 text-sm text-muted-foreground hover:text-foreground"
          }
        >
          {opt.label}
        </button>
      ))}
    </div>
  )
}

// ─── Columns ─────────────────────────────────────────────────────────────────

function buildColumns(
  onEdit: (etd: EntityTypeDefinition) => void,
  onDelete: (etd: EntityTypeDefinition) => void
): ColumnDef<EntityTypeDefinition>[] {
  return [
    {
      accessorKey: "type_name",
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title="Type Name" />
      ),
      cell: ({ row }) => (
        <Link
          to={`/entity-types/${row.original.id}`}
          className="font-mono text-sm font-medium hover:underline"
        >
          {row.original.type_name}
        </Link>
      ),
    },
    {
      accessorKey: "scope",
      header: "Scope",
      cell: ({ row }) => (
        <Badge
          variant={
            row.original.scope === "global" ? "default" : "secondary"
          }
        >
          {row.original.scope === "global" ? "Global" : "System-Scoped"}
        </Badge>
      ),
    },
    {
      accessorKey: "created_at",
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title="Created" />
      ),
      cell: ({ row }) =>
        new Date(row.original.created_at).toLocaleDateString(),
    },
    {
      id: "actions",
      cell: ({ row }) => (
        <div className="flex items-center gap-1">
          <Button variant="ghost" size="sm" asChild>
            <Link to={`/entity-types/${row.original.id}`}>
              <Eye className="size-3.5" />
              <span className="sr-only">View</span>
            </Link>
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => onEdit(row.original)}
          >
            <Pencil className="size-3.5" />
            <span className="sr-only">Edit</span>
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => onDelete(row.original)}
          >
            <Trash2 className="size-3.5 text-destructive" />
            <span className="sr-only">Delete</span>
          </Button>
        </div>
      ),
    },
  ]
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function EntityTypes() {
  const navigate = useNavigate()
  const { data, isLoading } = useEntityTypes()
  const deleteMutation = useDeleteEntityType()

  const [scopeFilter, setScopeFilter] = useState<ScopeFilter>("all")
  const [deleteTarget, setDeleteTarget] = useState<EntityTypeDefinition | undefined>()

  const allItems = data?.items ?? []
  const filtered =
    scopeFilter === "all"
      ? allItems
      : allItems.filter((e) => e.scope === scopeFilter)

  const columns = buildColumns(
    (etd) => navigate(`/entity-types/${etd.id}/edit`),
    setDeleteTarget
  )

  return (
    <FadeContent duration={0.4} className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">
            Entity Types
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Schema registry — define and manage entity type definitions
          </p>
        </div>
        <Button onClick={() => navigate("/entity-types/new")}>
          <Plus className="mr-1.5 size-4" />
          New Entity Type
        </Button>
      </div>

      {/* Filter */}
      <div className="flex items-center gap-4">
        <ScopeFilterBar value={scopeFilter} onChange={setScopeFilter} />
        {!isLoading && (
          <span className="text-sm text-muted-foreground">
            {filtered.length}{" "}
            {filtered.length === 1 ? "type" : "types"}
          </span>
        )}
      </div>

      {/* Table */}
      <DataTable
        columns={columns}
        data={filtered}
        isLoading={isLoading}
        emptyMessage="No entity types found."
      />

      {/* Delete confirmation */}
      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(o) => !o && setDeleteTarget(undefined)}
        title={`Delete "${deleteTarget?.type_name}"?`}
        description="This will permanently delete the entity type definition. Entities of this type must be removed first."
        confirmLabel="Delete"
        isLoading={deleteMutation.isPending}
        onConfirm={async () => {
          if (deleteTarget) {
            await deleteMutation.mutateAsync(deleteTarget.id)
            setDeleteTarget(undefined)
          }
        }}
      />
    </FadeContent>
  )
}
