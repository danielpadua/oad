import { useState } from "react"
import { Link, useNavigate } from "react-router-dom"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { Plus, Eye, Pencil, Trash2 } from "lucide-react"
import type { ColumnDef } from "@tanstack/react-table"
import { useTranslation } from "react-i18next"

import { http } from "@/lib/http-client"
import type { HttpError } from "@/lib/http-client"
import { toastSuccess, toastApiError } from "@/lib/toast"
import type { EntityTypeDefinition, ListResponse } from "@/lib/types"
import { useAuth } from "@/contexts/AuthContext"
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

function ScopeFilterBar({
  value,
  onChange,
}: {
  value: ScopeFilter
  onChange: (v: ScopeFilter) => void
}) {
  const { t } = useTranslation()
  const options: { value: ScopeFilter; label: string }[] = [
    { value: "all", label: t("scope.all") },
    { value: "global", label: t("scope.global") },
    { value: "system_scoped", label: t("scope.system_scoped") },
  ]

  return (
    <div
      className="flex rounded-lg border border-border bg-muted/40 p-0.5"
      role="group"
      aria-label="Filter by scope"
    >
      {options.map((opt) => (
        <button
          key={opt.value}
          onClick={() => onChange(opt.value)}
          aria-pressed={value === opt.value}
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

type TFunc = (key: string, opts?: Record<string, unknown>) => string

function buildColumns(
  onEdit: (etd: EntityTypeDefinition) => void,
  onDelete: (etd: EntityTypeDefinition) => void,
  canMutate: (etd: EntityTypeDefinition) => boolean,
  t: TFunc,
  tc: TFunc,
): ColumnDef<EntityTypeDefinition>[] {
  return [
    {
      accessorKey: "type_name",
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t("columns.typeName")} />
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
      header: t("columns.scope"),
      cell: ({ row }) => (
        <Badge
          variant={
            row.original.scope === "global" ? "default" : "secondary"
          }
        >
          {row.original.scope === "global" ? tc("scope.global") : tc("scope.system_scoped")}
        </Badge>
      ),
    },
    {
      accessorKey: "created_at",
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t("columns.created")} />
      ),
      cell: ({ row }) =>
        new Date(row.original.created_at).toLocaleDateString(),
    },
    {
      id: "actions",
      cell: ({ row }) => {
        const mutable = canMutate(row.original)
        return (
          <div className="flex items-center gap-1">
            <Button variant="ghost" size="sm" asChild>
              <Link to={`/entity-types/${row.original.id}`}>
                <Eye className="size-3.5" />
                <span className="sr-only">{tc("actions.view")}</span>
              </Link>
            </Button>
            {mutable && (
              <>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => onEdit(row.original)}
                >
                  <Pencil className="size-3.5" />
                  <span className="sr-only">{tc("actions.edit")}</span>
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => onDelete(row.original)}
                >
                  <Trash2 className="size-3.5 text-destructive" />
                  <span className="sr-only">{tc("actions.delete")}</span>
                </Button>
              </>
            )}
          </div>
        )
      },
    },
  ]
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function EntityTypes() {
  const navigate = useNavigate()
  const { data, isLoading } = useEntityTypes()
  const deleteMutation = useDeleteEntityType()
  const { identity } = useAuth()
  const isPlatformAdmin = identity?.systemId == null
  const { t } = useTranslation("entityTypes")
  const { t: tc } = useTranslation()

  const [scopeFilter, setScopeFilter] = useState<ScopeFilter>("all")
  const [deleteTarget, setDeleteTarget] = useState<EntityTypeDefinition | undefined>()

  const allItems = data?.items ?? []
  const filtered =
    scopeFilter === "all"
      ? allItems
      : allItems.filter((e) => e.scope === scopeFilter)

  const columns = buildColumns(
    (etd) => navigate(`/entity-types/${etd.id}/edit`),
    setDeleteTarget,
    // System-scoped admins may mutate only system-scoped entity types.
    // Global definitions cross tenant boundaries and are platform-admin only.
    (etd) => isPlatformAdmin || etd.scope !== "global",
    t,
    tc,
  )

  return (
    <FadeContent duration={0.4} className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">
            {t("title")}
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            {t("subtitle")}
          </p>
        </div>
        <Button onClick={() => navigate("/entity-types/new")}>
          <Plus className="mr-1.5 size-4" />
          {t("new")}
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
        emptyMessage={t("empty")}
      />

      {/* Delete confirmation */}
      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(o) => !o && setDeleteTarget(undefined)}
        title={t("delete.title", { name: deleteTarget?.type_name })}
        description={t("delete.description")}
        confirmLabel={t("delete.confirm")}
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
