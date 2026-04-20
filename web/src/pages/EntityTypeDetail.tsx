import { useState } from "react"
import { useParams, Link, useNavigate } from "react-router-dom"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { ArrowLeft, Pencil, Trash2, Database } from "lucide-react"

import { http } from "@/lib/http-client"
import type { HttpError } from "@/lib/http-client"
import { toastSuccess, toastApiError } from "@/lib/toast"
import type { EntityTypeDefinition, ListResponse } from "@/lib/types"
import { FadeContent } from "@/components/reactbits"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { ConfirmDialog } from "@/components/ui/confirm-dialog"
import { PropertyBuilder } from "@/components/ui/property-builder"
import { RelationBuilder } from "@/components/ui/relation-builder"
import { Skeleton } from "@/components/ui/skeleton"

// ─── Queries ─────────────────────────────────────────────────────────────────

function useEntityType(id: string) {
  return useQuery<EntityTypeDefinition, HttpError>({
    queryKey: ["entity-type", id],
    queryFn: () =>
      http.get<EntityTypeDefinition>(`/api/v1/entity-types/${id}`),
    enabled: !!id,
  })
}

function useEntityUsageCount(typeName: string | undefined) {
  return useQuery<ListResponse<unknown>, HttpError>({
    queryKey: ["entity-usage-count", typeName],
    queryFn: () =>
      http.get<ListResponse<unknown>>(
        `/api/v1/entities?type=${encodeURIComponent(typeName!)}&limit=1`
      ),
    enabled: !!typeName,
  })
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function EntityTypeDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const { data: etd, isLoading, isError } = useEntityType(id!)
  const { data: usageData, isLoading: usageLoading } = useEntityUsageCount(
    etd?.type_name
  )

  const [deleteOpen, setDeleteOpen] = useState(false)

  const deleteMutation = useMutation({
    mutationFn: () => http.del(`/api/v1/entity-types/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["entity-types"] })
      toastSuccess("Entity type deleted")
      navigate("/entity-types")
    },
    onError: toastApiError,
  })

  if (isLoading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-32 w-full" />
        <Skeleton className="h-48 w-full" />
      </div>
    )
  }

  if (isError || !etd) {
    return (
      <div className="flex flex-col items-center gap-4 py-16 text-center">
        <p className="text-muted-foreground">Entity type not found.</p>
        <Button variant="outline" asChild>
          <Link to="/entity-types">Back to Entity Types</Link>
        </Button>
      </div>
    )
  }

  const usageCount = usageData?.total ?? 0

  return (
    <FadeContent duration={0.35} className="space-y-6">
      {/* Breadcrumb */}
      <Button variant="ghost" size="sm" asChild className="-ml-2">
        <Link to="/entity-types">
          <ArrowLeft className="mr-1.5 size-4" />
          Entity Types
        </Link>
      </Button>

      {/* Header */}
      <div className="flex items-start justify-between gap-4">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="font-mono text-2xl font-semibold tracking-tight">
              {etd.type_name}
            </h1>
            <Badge
              variant={etd.scope === "global" ? "default" : "secondary"}
            >
              {etd.scope === "global" ? "Global" : "System-Scoped"}
            </Badge>
          </div>
          <p className="mt-1 text-sm text-muted-foreground">
            Created {new Date(etd.created_at).toLocaleString()} · Last updated{" "}
            {new Date(etd.updated_at).toLocaleString()}
          </p>
        </div>
        <div className="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => navigate(`/entity-types/${id}/edit`)}
          >
            <Pencil className="mr-1.5 size-3.5" />
            Edit
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => setDeleteOpen(true)}
          >
            <Trash2 className="mr-1.5 size-3.5 text-destructive" />
            Delete
          </Button>
        </div>
      </div>

      {/* Usage card */}
      <div className="flex items-center gap-3 rounded-lg border border-border bg-card px-4 py-3">
        <div className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
          <Database className="size-4" />
        </div>
        <div>
          <p className="text-xs uppercase tracking-widest text-muted-foreground">
            Entity Usage
          </p>
          {usageLoading ? (
            <Skeleton className="mt-0.5 h-5 w-12" />
          ) : (
            <p className="text-lg font-semibold tabular-nums">
              {usageCount.toLocaleString()}{" "}
              <span className="text-sm font-normal text-muted-foreground">
                {usageCount === 1 ? "entity" : "entities"}
              </span>
            </p>
          )}
        </div>
      </div>

      {/* Allowed Properties */}
      <section className="space-y-2">
        <h2 className="text-sm font-medium uppercase tracking-widest text-muted-foreground">
          Allowed Properties
        </h2>
        <PropertyBuilder
          value={
            etd.allowed_properties != null
              ? JSON.stringify(etd.allowed_properties, null, 2)
              : '{"type":"object","properties":{}}'
          }
          readOnly
        />
      </section>

      {/* Allowed Relations */}
      <section className="space-y-2">
        <h2 className="text-sm font-medium uppercase tracking-widest text-muted-foreground">
          Allowed Relations
        </h2>
        <RelationBuilder
          value={
            etd.allowed_relations != null
              ? JSON.stringify(etd.allowed_relations, null, 2)
              : "{}"
          }
          readOnly
        />
      </section>

      {/* Delete confirmation */}
      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title={`Delete "${etd.type_name}"?`}
        description={
          usageCount > 0 ? (
            <span>
              This type has{" "}
              <strong className="text-foreground">
                {usageCount} {usageCount === 1 ? "entity" : "entities"}
              </strong>{" "}
              using it. Remove all entities of this type before deleting.
            </span>
          ) : (
            "This will permanently delete the entity type definition. This action cannot be undone."
          )
        }
        confirmLabel="Delete"
        isLoading={deleteMutation.isPending}
        onConfirm={() => { deleteMutation.mutate(undefined) }}
      />
    </FadeContent>
  )
}
