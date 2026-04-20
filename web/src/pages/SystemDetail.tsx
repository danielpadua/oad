import { useState } from "react"
import { useParams, Link } from "react-router-dom"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { ArrowLeft, Plus, Pencil, Trash2, Layers } from "lucide-react"
import type { ColumnDef } from "@tanstack/react-table"

import { http } from "@/lib/http-client"
import type { HttpError } from "@/lib/http-client"
import { toastSuccess, toastMutationError } from "@/lib/toast"
import type {
  System,
  SystemOverlaySchema,
  EntityTypeDefinition,
  ListResponse,
} from "@/lib/types"
import { FadeContent } from "@/components/reactbits"
import { DataTable, DataTableColumnHeader } from "@/components/ui/data-table"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { ConfirmDialog } from "@/components/ui/confirm-dialog"
import { Skeleton } from "@/components/ui/skeleton"

// ─── Queries ─────────────────────────────────────────────────────────────────

function useSystem(id: string) {
  return useQuery<System, HttpError>({
    queryKey: ["system", id],
    queryFn: () => http.get<System>(`/api/v1/systems/${id}`),
    enabled: !!id,
  })
}

function useOverlaySchemas(systemId: string) {
  return useQuery<ListResponse<SystemOverlaySchema>, HttpError>({
    queryKey: ["overlay-schemas", systemId],
    queryFn: () =>
      http.get<ListResponse<SystemOverlaySchema>>(
        `/api/v1/systems/${systemId}/overlay-schemas`
      ),
    enabled: !!systemId,
  })
}

function useEntityTypes() {
  return useQuery<ListResponse<EntityTypeDefinition>, HttpError>({
    queryKey: ["entity-types"],
    queryFn: () =>
      http.get<ListResponse<EntityTypeDefinition>>("/api/v1/entity-types"),
  })
}

// ─── Table columns ────────────────────────────────────────────────────────────

function buildColumns(
  systemId: string,
  entityTypeMap: Map<string, string>,
  onDelete: (s: SystemOverlaySchema) => void
): ColumnDef<SystemOverlaySchema>[] {
  return [
    {
      accessorKey: "entity_type_id",
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title="Entity Type" />
      ),
      cell: ({ row }) => (
        <span className="font-mono text-sm">
          {entityTypeMap.get(row.original.entity_type_id) ??
            row.original.entity_type_id}
        </span>
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
            <Link
              to={`/systems/${systemId}/overlay-schemas/${row.original.id}/edit`}
            >
              <Pencil className="size-3.5" />
              <span className="sr-only">Edit</span>
            </Link>
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

export default function SystemDetail() {
  const { id } = useParams<{ id: string }>()
  const queryClient = useQueryClient()

  const { data: system, isLoading: systemLoading, isError } = useSystem(id!)
  const { data: schemasData, isLoading: schemasLoading } = useOverlaySchemas(id!)
  const { data: etdData } = useEntityTypes()

  const [deleteTarget, setDeleteTarget] = useState<
    SystemOverlaySchema | undefined
  >()

  const deleteMutation = useMutation({
    mutationFn: (schemaId: string) =>
      http.del(`/api/v1/systems/${id}/overlay-schemas/${schemaId}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["overlay-schemas", id] })
      toastSuccess("Overlay schema deleted")
    },
    onError: toastMutationError,
  })

  const schemas = schemasData?.items ?? []
  const entityTypes = etdData?.items ?? []

  // Build a map of entity_type_id → type_name for display.
  const entityTypeMap = new Map(entityTypes.map((et) => [et.id, et.type_name]))

  const columns = buildColumns(id!, entityTypeMap, setDeleteTarget)

  if (systemLoading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-24 w-full" />
        <Skeleton className="h-48 w-full" />
      </div>
    )
  }

  if (isError || !system) {
    return (
      <div className="flex flex-col items-center gap-4 py-16 text-center">
        <p className="text-muted-foreground">System not found.</p>
        <Button variant="outline" asChild>
          <Link to="/systems">Back to Systems</Link>
        </Button>
      </div>
    )
  }

  return (
    <FadeContent duration={0.35} className="space-y-6">
      {/* Back */}
      <Button variant="ghost" size="sm" asChild className="-ml-2">
        <Link to="/systems">
          <ArrowLeft className="mr-1.5 size-4" />
          Systems
        </Link>
      </Button>

      {/* Header */}
      <div className="flex items-start justify-between gap-4">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-2xl font-semibold tracking-tight">
              {system.name}
            </h1>
            <Badge variant={system.active ? "default" : "secondary"}>
              {system.active ? "Active" : "Inactive"}
            </Badge>
          </div>
          {system.description && (
            <p className="mt-1 text-sm text-muted-foreground">
              {system.description}
            </p>
          )}
          <p className="mt-1 text-xs text-muted-foreground">
            ID: <code className="font-mono">{system.id}</code> · Registered{" "}
            {new Date(system.created_at).toLocaleDateString()}
          </p>
        </div>
      </div>

      {/* Overlay Schemas section */}
      <section className="space-y-4">
        <div className="flex items-center justify-between gap-3">
          <div>
            <h2 className="flex items-center gap-2 text-lg font-semibold">
              <Layers className="size-5 text-muted-foreground" />
              Overlay Schemas
            </h2>
            <p className="text-sm text-muted-foreground">
              Per-type schemas that define which overlay properties{" "}
              <strong className="text-foreground">{system.name}</strong> may
              attach to entities. All keys must be prefixed with{" "}
              <code className="rounded bg-muted px-1 py-0.5 font-mono text-xs">
                {system.name}.
              </code>
            </p>
          </div>
          <Button size="sm" asChild>
            <Link to={`/systems/${id}/overlay-schemas/new`}>
              <Plus className="mr-1.5 size-3.5" />
              Add Schema
            </Link>
          </Button>
        </div>

        <DataTable
          columns={columns}
          data={schemas}
          isLoading={schemasLoading}
          emptyMessage="No overlay schemas defined for this system."
        />
      </section>

      {/* Delete confirmation */}
      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(o) => !o && setDeleteTarget(undefined)}
        title="Delete overlay schema?"
        description={
          deleteTarget
            ? `Remove the overlay schema for "${entityTypeMap.get(deleteTarget.entity_type_id) ?? "this type"}" from ${system.name}. Existing overlays using this schema may become invalid.`
            : undefined
        }
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
