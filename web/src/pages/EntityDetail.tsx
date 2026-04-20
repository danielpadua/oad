import { useState, useRef, useEffect } from "react"
import { useParams, Link, useNavigate } from "react-router-dom"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import {
  ArrowLeft, Pencil, Trash2, Plus, Link2Off, Search, Layers, ScrollText, X, GitMerge,
} from "lucide-react"

import { http } from "@/lib/http-client"
import type { HttpError } from "@/lib/http-client"
import { toastSuccess, toastApiError, toastMutationError } from "@/lib/toast"
import type {
  Entity, EntityTypeDefinition, Relation, PaginatedResponse, ListResponse,
  PropertyOverlay, SystemOverlaySchema, MergedEntityView, System,
} from "@/lib/types"
import { useScope } from "@/contexts/ScopeContext"
import { FadeContent, ClickSpark } from "@/components/reactbits"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { ConfirmDialog } from "@/components/ui/confirm-dialog"
import { Modal } from "@/components/ui/modal"
import { Skeleton } from "@/components/ui/skeleton"
import { JsonSchemaEditor } from "@/components/ui/json-schema-editor"
import { Checkbox } from "@/components/ui/checkbox"
import { RequireAnyRole } from "@/components/auth/RoleGate"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { cn } from "@/lib/utils"

// ─── Queries ─────────────────────────────────────────────────────────────────

function useEntity(id: string) {
  return useQuery<Entity, HttpError>({
    queryKey: ["entity", id],
    queryFn: () => http.get<Entity>(`/api/v1/entities/${id}`),
    enabled: !!id,
  })
}

function useEntityType(id: string | undefined) {
  return useQuery<EntityTypeDefinition, HttpError>({
    queryKey: ["entity-type", id],
    queryFn: () => http.get<EntityTypeDefinition>(`/api/v1/entity-types/${id!}`),
    enabled: !!id,
  })
}

function useRelations(entityId: string) {
  return useQuery<PaginatedResponse<Relation>, HttpError>({
    queryKey: ["entity-relations", entityId],
    queryFn: () =>
      http.get<PaginatedResponse<Relation>>(
        `/api/v1/entities/${entityId}/relations?limit=100`
      ),
    enabled: !!entityId,
  })
}

function useEntitySearch(query: string, typeFilter: string) {
  return useQuery<PaginatedResponse<Entity>, HttpError>({
    queryKey: ["entity-search", query, typeFilter],
    queryFn: () => {
      const params = new URLSearchParams({ limit: "10" })
      if (typeFilter) params.set("type", typeFilter)
      return http.get<PaginatedResponse<Entity>>(`/api/v1/entities?${params.toString()}`)
    },
    enabled: query.length >= 2,
    staleTime: 30_000,
  })
}

function useEntityOverlays(entityId: string) {
  return useQuery<PaginatedResponse<PropertyOverlay>, HttpError>({
    queryKey: ["entity-overlays", entityId],
    queryFn: () =>
      http.get<PaginatedResponse<PropertyOverlay>>(
        `/api/v1/entities/${entityId}/overlays?limit=50`
      ),
    enabled: !!entityId,
  })
}

function useOverlaySchemas(systemId: string | null) {
  return useQuery<ListResponse<SystemOverlaySchema>, HttpError>({
    queryKey: ["overlay-schemas", systemId],
    queryFn: () =>
      http.get<ListResponse<SystemOverlaySchema>>(
        `/api/v1/systems/${systemId!}/overlay-schemas`
      ),
    enabled: !!systemId,
  })
}

function useSystem(systemId: string | null) {
  return useQuery<System, HttpError>({
    queryKey: ["system", systemId],
    queryFn: () => http.get<System>(`/api/v1/systems/${systemId!}`),
    enabled: !!systemId,
  })
}

function useMergedView(entity: Entity | undefined, systemId: string | null) {
  return useQuery<MergedEntityView, HttpError>({
    queryKey: ["entity-merged", entity?.id, systemId],
    queryFn: () => {
      const params = new URLSearchParams({
        type: entity!.type,
        external_id: entity!.external_id,
        system_id: systemId!,
      })
      return http.get<MergedEntityView>(`/api/v1/entities/lookup?${params}`)
    },
    enabled: !!entity && !!systemId,
  })
}

// ─── Schema parsing for overlay form ─────────────────────────────────────────

interface SchemaProp {
  type: string
  description?: string
  required: boolean
  enum?: unknown[]
  minimum?: number
  maximum?: number
  minLength?: number
  maxLength?: number
}

function parseSchema(raw: unknown): Record<string, SchemaProp> | null {
  try {
    const schema =
      typeof raw === "string"
        ? (JSON.parse(raw) as Record<string, unknown>)
        : (raw as Record<string, unknown>)
    if (!schema?.properties || typeof schema.properties !== "object") return null
    const required = new Set<string>((schema.required as string[]) ?? [])
    const props = schema.properties as Record<string, Record<string, unknown>>
    const result: Record<string, SchemaProp> = {}
    for (const [key, def] of Object.entries(props)) {
      result[key] = {
        type: (def.type as string) ?? "string",
        description: def.description as string | undefined,
        required: required.has(key),
        enum: def.enum as unknown[] | undefined,
        minimum: def.minimum as number | undefined,
        maximum: def.maximum as number | undefined,
        minLength: def.minLength as number | undefined,
        maxLength: def.maxLength as number | undefined,
      }
    }
    return Object.keys(result).length > 0 ? result : null
  } catch {
    return null
  }
}

function OverlayDynamicField({
  name,
  prop,
  value,
  onChange,
}: {
  name: string
  prop: SchemaProp
  value: unknown
  onChange: (v: unknown) => void
}) {
  if (prop.type === "boolean") {
    return (
      <div className="flex items-center gap-2">
        <Checkbox
          id={`ovl-${name}`}
          checked={!!value}
          onCheckedChange={(c) => onChange(!!c)}
        />
        <Label htmlFor={`ovl-${name}`} className="font-mono text-sm">
          {name}
          {prop.required && <span className="ml-1 text-destructive">*</span>}
        </Label>
      </div>
    )
  }
  if (prop.enum?.length) {
    return (
      <div className="space-y-1.5">
        <Label htmlFor={`ovl-${name}`} className="font-mono text-sm">
          {name}
          {prop.required && <span className="ml-1 text-destructive">*</span>}
        </Label>
        <Select value={String(value ?? "")} onValueChange={onChange}>
          <SelectTrigger id={`ovl-${name}`}>
            <SelectValue placeholder="Select…" />
          </SelectTrigger>
          <SelectContent>
            {prop.enum.map((v) => (
              <SelectItem key={String(v)} value={String(v)}>
                {String(v)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
    )
  }
  if (prop.type === "number" || prop.type === "integer") {
    return (
      <div className="space-y-1.5">
        <Label htmlFor={`ovl-${name}`} className="font-mono text-sm">
          {name}
          {prop.required && <span className="ml-1 text-destructive">*</span>}
        </Label>
        <Input
          id={`ovl-${name}`}
          type="number"
          min={prop.minimum}
          max={prop.maximum}
          step={prop.type === "integer" ? 1 : undefined}
          value={value != null ? String(value) : ""}
          onChange={(e) =>
            onChange(e.target.value === "" ? undefined : Number(e.target.value))
          }
        />
      </div>
    )
  }
  return (
    <div className="space-y-1.5">
      <Label htmlFor={`ovl-${name}`} className="font-mono text-sm">
        {name}
        {prop.required && <span className="ml-1 text-destructive">*</span>}
        {prop.type !== "string" && (
          <span className="ml-1 text-xs font-normal text-muted-foreground">
            ({prop.type})
          </span>
        )}
      </Label>
      <Input
        id={`ovl-${name}`}
        value={typeof value === "string" ? value : ""}
        minLength={prop.minLength}
        maxLength={prop.maxLength}
        onChange={(e) => onChange(e.target.value)}
      />
    </div>
  )
}

// ─── Overlay form modal ───────────────────────────────────────────────────────

interface OverlayFormModalProps {
  open: boolean
  onOpenChange: (v: boolean) => void
  entity: Entity
  systemId: string
  existing?: PropertyOverlay
  onSuccess: () => void
}

function OverlayFormModal({
  open,
  onOpenChange,
  entity,
  systemId,
  existing,
  onSuccess,
}: OverlayFormModalProps) {
  const { data: schemasData } = useOverlaySchemas(systemId)
  const { data: system } = useSystem(systemId)

  const overlaySchema = schemasData?.items.find(
    (s) => s.entity_type_id === entity.type_id
  )
  const parsedProps = overlaySchema
    ? parseSchema(overlaySchema.allowed_overlay_properties)
    : null

  const [values, setValues] = useState<Record<string, unknown>>(
    existing?.properties ?? {}
  )
  const [rawJson, setRawJson] = useState(
    JSON.stringify(existing?.properties ?? {}, null, 2)
  )
  const [rawMode, setRawMode] = useState(!parsedProps)
  const [jsonError, setJsonError] = useState<string | null>(null)

  useEffect(() => {
    if (!rawMode && !parsedProps) setRawMode(true)
  }, [parsedProps, rawMode])

  useEffect(() => {
    if (open) {
      const init = existing?.properties ?? {}
      setValues(init)
      setRawJson(JSON.stringify(init, null, 2))
      setRawMode(!parsedProps)
      setJsonError(null)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, existing])

  const createMutation = useMutation({
    mutationFn: (properties: unknown) =>
      http.post<PropertyOverlay>(`/api/v1/entities/${entity.id}/overlays`, {
        properties,
      }),
    onSuccess: () => { toastSuccess("Overlay created"); onSuccess(); onOpenChange(false) },
    onError: toastMutationError,
  })

  const updateMutation = useMutation({
    mutationFn: (properties: unknown) =>
      http.put<PropertyOverlay>(
        `/api/v1/entities/${entity.id}/overlays/${existing!.id}`,
        { properties }
      ),
    onSuccess: () => { toastSuccess("Overlay updated"); onSuccess(); onOpenChange(false) },
    onError: toastMutationError,
  })

  const isPending = createMutation.isPending || updateMutation.isPending
  const isEdit = !!existing

  function handleSubmit() {
    if (rawMode) {
      try {
        const parsed = JSON.parse(rawJson) as unknown
        setJsonError(null)
        isEdit ? updateMutation.mutate(parsed) : createMutation.mutate(parsed)
      } catch {
        setJsonError("Invalid JSON — fix the syntax before saving.")
      }
    } else {
      isEdit ? updateMutation.mutate(values) : createMutation.mutate(values)
    }
  }

  return (
    <Modal
      open={open}
      onOpenChange={onOpenChange}
      title={isEdit ? "Edit Overlay" : "Create Overlay"}
      size="md"
    >
      <div className="space-y-4 pt-2">
        {/* System + namespace info */}
        {system && (
          <div className="rounded-lg border border-border bg-muted/30 px-3 py-2">
            <p className="text-xs text-muted-foreground">System scope</p>
            <div className="mt-0.5 flex items-center gap-2">
              <span className="text-sm font-medium">{system.name}</span>
              <span className="text-xs text-muted-foreground">
                · keys prefixed with{" "}
                <code className="rounded bg-muted px-1 font-mono text-xs">
                  {system.name}.
                </code>
              </span>
            </div>
          </div>
        )}

        {!overlaySchema && (
          <div className="rounded-lg border border-amber-500/30 bg-amber-500/10 px-3 py-2 text-xs text-amber-700 dark:text-amber-400">
            No overlay schema for this entity type. Properties won't be
            schema-validated.
          </div>
        )}

        {/* Properties */}
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <Label className="text-sm font-medium">Properties</Label>
            {parsedProps && (
              <Button
                variant="ghost"
                size="sm"
                className="text-xs"
                onClick={() => setRawMode((v) => !v)}
              >
                {rawMode ? "Visual editor" : "Raw JSON"}
              </Button>
            )}
          </div>

          {rawMode ? (
            <>
              <JsonSchemaEditor
                value={rawJson}
                onChange={(v) => { setRawJson(v); setJsonError(null) }}
                minHeight="180px"
                aria-label="Overlay properties"
              />
              {jsonError && (
                <p className="text-sm font-medium text-destructive">{jsonError}</p>
              )}
            </>
          ) : (
            parsedProps && (
              <div className="space-y-3">
                {Object.entries(parsedProps).map(([key, prop]) => (
                  <OverlayDynamicField
                    key={key}
                    name={key}
                    prop={prop}
                    value={values[key]}
                    onChange={(v) =>
                      setValues((prev) => ({ ...prev, [key]: v }))
                    }
                  />
                ))}
              </div>
            )
          )}
        </div>

        <div className="flex justify-end gap-2 border-t border-border pt-3">
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <ClickSpark color="hsl(var(--primary))" disabled={isPending}>
            <Button onClick={handleSubmit} disabled={isPending}>
              {isPending ? "Saving…" : isEdit ? "Save Changes" : "Create"}
            </Button>
          </ClickSpark>
        </div>
      </div>
    </Modal>
  )
}

// ─── Tabs ─────────────────────────────────────────────────────────────────────

type Tab = "properties" | "relations" | "overlays" | "audit"

const TABS: { id: Tab; label: string }[] = [
  { id: "properties", label: "Properties" },
  { id: "relations", label: "Relations" },
  { id: "overlays", label: "Overlays" },
  { id: "audit", label: "Audit" },
]

// ─── Add Relation Dialog ──────────────────────────────────────────────────────

interface AllowedRelations {
  [relationType: string]: { target_types?: string[] }
}

interface AddRelationDialogProps {
  open: boolean
  onOpenChange: (v: boolean) => void
  subjectEntityId: string
  allowedRelations: AllowedRelations
}

function AddRelationDialog({
  open,
  onOpenChange,
  subjectEntityId,
  allowedRelations,
}: AddRelationDialogProps) {
  const queryClient = useQueryClient()
  const [relationType, setRelationType] = useState("")
  const [targetSearch, setTargetSearch] = useState("")
  const [targetEntity, setTargetEntity] = useState<Entity | null>(null)
  const [showDropdown, setShowDropdown] = useState(false)
  const dropdownRef = useRef<HTMLDivElement>(null)

  const relationTypes = Object.keys(allowedRelations)
  const allowedTargetTypes =
    relationType && allowedRelations[relationType]?.target_types
      ? allowedRelations[relationType].target_types!
      : []

  const searchTypeFilter = allowedTargetTypes[0] ?? ""
  const { data: searchResults } = useEntitySearch(targetSearch, searchTypeFilter)

  const createMutation = useMutation({
    mutationFn: (payload: {
      subject_entity_id: string
      relation_type: string
      target_entity_id: string
    }) => http.post<Relation>("/api/v1/relations", payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["entity-relations", subjectEntityId] })
      toastSuccess("Relation created")
      onOpenChange(false)
    },
    onError: toastMutationError,
  })

  function reset() {
    setRelationType("")
    setTargetSearch("")
    setTargetEntity(null)
    setShowDropdown(false)
  }

  function handleClose(v: boolean) {
    if (!v) reset()
    onOpenChange(v)
  }

  function handleSubmit() {
    if (!relationType || !targetEntity) return
    createMutation.mutate({
      subject_entity_id: subjectEntityId,
      relation_type: relationType,
      target_entity_id: targetEntity.id,
    })
  }

  const filteredResults = (searchResults?.items ?? []).filter(
    (e) => e.id !== subjectEntityId
  )

  return (
    <Modal open={open} onOpenChange={handleClose} title="Add Relation" size="sm">
      <div className="space-y-4 pt-2">
        {/* Relation type */}
        <div className="space-y-1.5">
          <label className="text-sm font-medium">Relation Type</label>
          {relationTypes.length > 0 ? (
            <Select
              value={relationType}
              onValueChange={(v) => {
                setRelationType(v)
                setTargetEntity(null)
                setTargetSearch("")
              }}
            >
              <SelectTrigger>
                <SelectValue placeholder="Select relation type…" />
              </SelectTrigger>
              <SelectContent>
                {relationTypes.map((rt) => (
                  <SelectItem key={rt} value={rt}>
                    <span className="font-mono">{rt}</span>
                    {allowedRelations[rt]?.target_types?.length ? (
                      <span className="ml-2 text-xs text-muted-foreground">
                        → {allowedRelations[rt].target_types!.join(", ")}
                      </span>
                    ) : null}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          ) : (
            <Input
              placeholder="e.g. member_of"
              value={relationType}
              onChange={(e) => setRelationType(e.target.value)}
            />
          )}
        </div>

        {/* Target entity search */}
        <div className="space-y-1.5">
          <label className="text-sm font-medium">Target Entity</label>
          {targetEntity ? (
            <div className="flex items-center justify-between rounded-lg border border-border bg-muted/30 px-3 py-2">
              <div>
                <p className="font-mono text-sm">{targetEntity.external_id}</p>
                <p className="text-xs text-muted-foreground">{targetEntity.type}</p>
              </div>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setTargetEntity(null)}
              >
                <X className="size-3.5" />
              </Button>
            </div>
          ) : (
            <div className="relative" ref={dropdownRef}>
              <Search className="absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
              <Input
                placeholder={
                  allowedTargetTypes.length
                    ? `Search ${allowedTargetTypes.join(" or ")} entities…`
                    : "Search entities…"
                }
                value={targetSearch}
                className="pl-8"
                onChange={(e) => {
                  setTargetSearch(e.target.value)
                  setShowDropdown(true)
                }}
                onFocus={() => setShowDropdown(true)}
              />
              {showDropdown && filteredResults.length > 0 && (
                <>
                  <div
                    className="fixed inset-0 z-10"
                    onClick={() => setShowDropdown(false)}
                    aria-hidden
                  />
                  <div className="absolute left-0 top-full z-20 mt-1 w-full rounded-lg border border-border bg-card py-1 shadow-lg">
                    {filteredResults.map((e) => (
                      <button
                        key={e.id}
                        className="flex w-full cursor-pointer flex-col px-3 py-2 text-left hover:bg-muted"
                        onClick={() => {
                          setTargetEntity(e)
                          setTargetSearch(e.external_id)
                          setShowDropdown(false)
                        }}
                      >
                        <span className="font-mono text-sm">{e.external_id}</span>
                        <span className="text-xs text-muted-foreground">{e.type}</span>
                      </button>
                    ))}
                  </div>
                </>
              )}
            </div>
          )}
          {allowedTargetTypes.length > 0 && (
            <p className="text-xs text-muted-foreground">
              Restricted to: {allowedTargetTypes.join(", ")}
            </p>
          )}
        </div>

        {/* Actions */}
        <div className="flex justify-end gap-2 border-t border-border pt-3">
          <Button variant="outline" onClick={() => handleClose(false)}>
            Cancel
          </Button>
          <ClickSpark
            color="hsl(var(--primary))"
            disabled={!relationType || !targetEntity || createMutation.isPending}
          >
            <Button
              onClick={handleSubmit}
              disabled={!relationType || !targetEntity || createMutation.isPending}
            >
              {createMutation.isPending ? "Adding…" : "Add Relation"}
            </Button>
          </ClickSpark>
        </div>
      </div>
    </Modal>
  )
}

// ─── Relations Tab ────────────────────────────────────────────────────────────

function RelationsTab({
  entity,
  allowedRelations,
}: {
  entity: Entity
  allowedRelations: AllowedRelations
}) {
  const queryClient = useQueryClient()
  const { data, isLoading } = useRelations(entity.id)
  const [addOpen, setAddOpen] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<Relation | undefined>()

  const deleteMutation = useMutation({
    mutationFn: (id: string) => http.del(`/api/v1/relations/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["entity-relations", entity.id] })
      toastSuccess("Relation removed")
    },
    onError: toastApiError,
  })

  const relations = data?.items ?? []

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">
          {isLoading ? "Loading…" : `${relations.length} ${relations.length === 1 ? "relation" : "relations"}`}
        </p>
        <Button size="sm" onClick={() => setAddOpen(true)}>
          <Plus className="mr-1.5 size-3.5" />
          Add Relation
        </Button>
      </div>

      {isLoading ? (
        <div className="space-y-2">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-14 w-full" />
          ))}
        </div>
      ) : relations.length === 0 ? (
        <div className="rounded-lg border border-dashed border-border py-10 text-center text-sm text-muted-foreground">
          No relations yet. Click "Add Relation" to create the first one.
        </div>
      ) : (
        <div className="space-y-2">
          {relations.map((rel) => (
            <div
              key={rel.id}
              className="flex items-center gap-3 rounded-lg border border-border bg-card px-4 py-3"
            >
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2">
                  <Badge variant="outline" className="font-mono text-xs">
                    {rel.relation_type}
                  </Badge>
                  <span className="text-xs text-muted-foreground">→</span>
                  <Link
                    to={`/entities/${rel.target_entity_id}`}
                    className="font-mono text-sm hover:underline truncate"
                  >
                    {rel.target_entity_id}
                  </Link>
                </div>
                <p className="mt-0.5 text-xs text-muted-foreground">
                  Created {new Date(rel.created_at).toLocaleString()}
                  {rel.system_id && ` · system ${rel.system_id}`}
                </p>
              </div>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setDeleteTarget(rel)}
              >
                <Link2Off className="size-3.5 text-destructive" />
                <span className="sr-only">Remove</span>
              </Button>
            </div>
          ))}
        </div>
      )}

      <AddRelationDialog
        open={addOpen}
        onOpenChange={setAddOpen}
        subjectEntityId={entity.id}
        allowedRelations={allowedRelations}
      />

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(o) => !o && setDeleteTarget(undefined)}
        title="Remove relation?"
        description={`Remove the "${deleteTarget?.relation_type}" relation? This action cannot be undone.`}
        confirmLabel="Remove"
        isLoading={deleteMutation.isPending}
        onConfirm={async () => {
          if (deleteTarget) {
            await deleteMutation.mutateAsync(deleteTarget.id)
            setDeleteTarget(undefined)
          }
        }}
      />
    </div>
  )
}

// ─── Overlays Tab ─────────────────────────────────────────────────────────────

function OverlaysTab({ entity }: { entity: Entity }) {
  const queryClient = useQueryClient()
  const { activeSystemId } = useScope()

  const { data: overlaysData, isLoading: overlaysLoading } = useEntityOverlays(entity.id)
  const { data: mergedView, isLoading: mergedLoading } = useMergedView(entity, activeSystemId)

  const [createOpen, setCreateOpen] = useState(false)
  const [editTarget, setEditTarget] = useState<PropertyOverlay | undefined>()
  const [deleteTarget, setDeleteTarget] = useState<PropertyOverlay | undefined>()

  const deleteMutation = useMutation({
    mutationFn: (id: string) =>
      http.del(`/api/v1/entities/${entity.id}/overlays/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["entity-overlays", entity.id] })
      queryClient.invalidateQueries({ queryKey: ["entity-merged", entity.id] })
      toastSuccess("Overlay deleted")
    },
    onError: toastApiError,
  })

  const overlays = overlaysData?.items ?? []

  function onFormSuccess() {
    queryClient.invalidateQueries({ queryKey: ["entity-overlays", entity.id] })
    queryClient.invalidateQueries({ queryKey: ["entity-merged", entity.id] })
  }

  return (
    <div className="space-y-6">
      {/* Overlay list */}
      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <div>
            <h3 className="text-sm font-semibold">System Overlays</h3>
            <p className="text-xs text-muted-foreground">
              System-specific properties attached to this entity.
              {!activeSystemId && " Select a system scope to manage overlays."}
            </p>
          </div>
          {activeSystemId && (
            <RequireAnyRole roles={["admin", "editor"]}>
              <Button size="sm" onClick={() => setCreateOpen(true)}>
                <Plus className="mr-1.5 size-3.5" />
                Add Overlay
              </Button>
            </RequireAnyRole>
          )}
        </div>

        {overlaysLoading ? (
          <div className="space-y-2">
            {Array.from({ length: 2 }).map((_, i) => (
              <Skeleton key={i} className="h-14 w-full" />
            ))}
          </div>
        ) : overlays.length === 0 ? (
          <div className="rounded-lg border border-dashed border-border py-8 text-center text-sm text-muted-foreground">
            No overlays attached to this entity.
          </div>
        ) : (
          <div className="space-y-2">
            {overlays.map((ov) => {
              const keys = Object.keys(ov.properties ?? {})
              return (
                <div
                  key={ov.id}
                  className="flex items-start gap-3 rounded-lg border border-border bg-card px-4 py-3"
                >
                  <div className="min-w-0 flex-1 space-y-1.5">
                    <div className="flex flex-wrap items-center gap-1.5">
                      {keys.map((k) => (
                        <Badge key={k} variant="outline" className="font-mono text-xs">
                          {k}
                        </Badge>
                      ))}
                      {keys.length === 0 && (
                        <span className="text-xs text-muted-foreground">No properties</span>
                      )}
                    </div>
                    <p className="text-xs text-muted-foreground">
                      System: <span className="font-mono">{ov.system_id}</span> · Updated{" "}
                      {new Date(ov.updated_at).toLocaleString()}
                    </p>
                  </div>
                  <RequireAnyRole roles={["admin", "editor"]}>
                    <div className="flex shrink-0 gap-1">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => setEditTarget(ov)}
                      >
                        <Pencil className="size-3.5" />
                        <span className="sr-only">Edit</span>
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => setDeleteTarget(ov)}
                      >
                        <Trash2 className="size-3.5 text-destructive" />
                        <span className="sr-only">Delete</span>
                      </Button>
                    </div>
                  </RequireAnyRole>
                </div>
              )
            })}
          </div>
        )}
      </div>

      {/* Merged view */}
      <div className="space-y-3 border-t border-border pt-6">
        <div>
          <h3 className="flex items-center gap-2 text-sm font-semibold">
            <GitMerge className="size-4 text-muted-foreground" />
            Merged View
          </h3>
          <p className="mt-0.5 text-xs text-muted-foreground">
            {activeSystemId
              ? "Global entity properties merged with the active system's overlay."
              : "Select a system scope to preview the merged property view."}
          </p>
        </div>

        {!activeSystemId ? (
          <div className="rounded-lg border border-dashed border-border py-6 text-center text-xs text-muted-foreground">
            <Layers className="mx-auto mb-2 size-5 text-muted-foreground/40" />
            No system scope selected.
          </div>
        ) : mergedLoading ? (
          <Skeleton className="h-32 w-full" />
        ) : mergedView ? (
          <JsonSchemaEditor
            value={JSON.stringify(mergedView.properties, null, 2)}
            readOnly
            minHeight="160px"
            aria-label="Merged entity properties"
          />
        ) : (
          <div className="rounded-lg border border-dashed border-border py-6 text-center text-xs text-muted-foreground">
            Merged view not available.
          </div>
        )}
      </div>

      {/* Modals */}
      {activeSystemId && createOpen && (
        <OverlayFormModal
          open={createOpen}
          onOpenChange={setCreateOpen}
          entity={entity}
          systemId={activeSystemId}
          onSuccess={onFormSuccess}
        />
      )}
      {activeSystemId && editTarget && (
        <OverlayFormModal
          open={!!editTarget}
          onOpenChange={(v) => !v && setEditTarget(undefined)}
          entity={entity}
          systemId={activeSystemId}
          existing={editTarget}
          onSuccess={onFormSuccess}
        />
      )}

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(o) => !o && setDeleteTarget(undefined)}
        title="Delete overlay?"
        description="Remove this overlay? System-specific properties will be permanently lost."
        confirmLabel="Delete"
        isLoading={deleteMutation.isPending}
        onConfirm={async () => {
          if (deleteTarget) {
            await deleteMutation.mutateAsync(deleteTarget.id)
            setDeleteTarget(undefined)
          }
        }}
      />
    </div>
  )
}

// ─── Properties Tab ───────────────────────────────────────────────────────────

function PropertiesTab({ entity }: { entity: Entity }) {
  const queryClient = useQueryClient()
  const [editing, setEditing] = useState(false)
  const [propsJson, setPropsJson] = useState(
    JSON.stringify(entity.properties ?? {}, null, 2)
  )
  const [jsonError, setJsonError] = useState<string | null>(null)

  const updateMutation = useMutation({
    mutationFn: (properties: unknown) =>
      http.patch<Entity>(`/api/v1/entities/${entity.id}`, { properties }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["entity", entity.id] })
      toastSuccess("Properties updated")
      setEditing(false)
    },
    onError: toastMutationError,
  })

  function handleSave() {
    try {
      const parsed = JSON.parse(propsJson) as unknown
      setJsonError(null)
      updateMutation.mutate(parsed)
    } catch {
      setJsonError("Invalid JSON — fix the syntax before saving.")
    }
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">
          Entity properties stored as JSON.
        </p>
        {!editing ? (
          <Button size="sm" variant="outline" onClick={() => setEditing(true)}>
            <Pencil className="mr-1.5 size-3.5" />
            Edit
          </Button>
        ) : (
          <div className="flex gap-2">
            <Button
              size="sm"
              variant="outline"
              onClick={() => {
                setEditing(false)
                setPropsJson(JSON.stringify(entity.properties ?? {}, null, 2))
                setJsonError(null)
              }}
            >
              Cancel
            </Button>
            <ClickSpark
              color="hsl(var(--primary))"
              disabled={updateMutation.isPending}
            >
              <Button size="sm" onClick={handleSave} disabled={updateMutation.isPending}>
                {updateMutation.isPending ? "Saving…" : "Save"}
              </Button>
            </ClickSpark>
          </div>
        )}
      </div>

      <JsonSchemaEditor
        value={propsJson}
        onChange={editing ? setPropsJson : undefined}
        readOnly={!editing}
        minHeight="200px"
        aria-label="Entity properties"
      />

      {jsonError && (
        <p className="text-sm font-medium text-destructive">{jsonError}</p>
      )}
    </div>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function EntityDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [activeTab, setActiveTab] = useState<Tab>("properties")
  const [deleteOpen, setDeleteOpen] = useState(false)

  const { data: entity, isLoading, isError } = useEntity(id!)
  const { data: entityType } = useEntityType(entity?.type_id)

  const allowedRelations: AllowedRelations =
    entityType?.allowed_relations != null &&
    typeof entityType.allowed_relations === "object" &&
    !Array.isArray(entityType.allowed_relations)
      ? (entityType.allowed_relations as AllowedRelations)
      : {}

  const deleteMutation = useMutation({
    mutationFn: () => http.del(`/api/v1/entities/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["entities"] })
      toastSuccess("Entity deleted")
      navigate("/entities")
    },
    onError: toastApiError,
  })

  if (isLoading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-24 w-full" />
        <Skeleton className="h-48 w-full" />
      </div>
    )
  }

  if (isError || !entity) {
    return (
      <div className="flex flex-col items-center gap-4 py-16 text-center">
        <p className="text-muted-foreground">Entity not found.</p>
        <Button variant="outline" asChild>
          <Link to="/entities">Back to Entities</Link>
        </Button>
      </div>
    )
  }

  return (
    <FadeContent duration={0.35} className="space-y-6">
      {/* Breadcrumb */}
      <Button variant="ghost" size="sm" asChild className="-ml-2">
        <Link to="/entities">
          <ArrowLeft className="mr-1.5 size-4" />
          Entities
        </Link>
      </Button>

      {/* Header */}
      <div className="flex items-start justify-between gap-4">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="font-mono text-2xl font-semibold tracking-tight">
              {entity.external_id}
            </h1>
            <Badge variant="secondary" className="font-mono">
              {entity.type}
            </Badge>
          </div>
          <p className="mt-1 text-sm text-muted-foreground">
            ID: <span className="font-mono text-xs">{entity.id}</span> · Created{" "}
            {new Date(entity.created_at).toLocaleString()} · Updated{" "}
            {new Date(entity.updated_at).toLocaleString()}
          </p>
        </div>
        <div className="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => navigate(`/entities/${id}/edit`)}
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

      {/* Tabs */}
      <div>
        <div className="flex border-b border-border">
          {TABS.map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={cn(
                "cursor-pointer border-b-2 px-4 py-2 text-sm font-medium transition-colors",
                activeTab === tab.id
                  ? "border-primary text-foreground"
                  : "border-transparent text-muted-foreground hover:text-foreground"
              )}
            >
              {tab.label}
            </button>
          ))}
        </div>

        <div className="pt-6">
          {activeTab === "properties" && <PropertiesTab entity={entity} />}

          {activeTab === "relations" && (
            <RelationsTab entity={entity} allowedRelations={allowedRelations} />
          )}

          {activeTab === "overlays" && (
            <OverlaysTab entity={entity} />
          )}

          {activeTab === "audit" && (
            <div className="flex flex-col items-center gap-3 py-16 text-center">
              <ScrollText className="size-8 text-muted-foreground/50" />
              <p className="text-sm text-muted-foreground">
                Audit log viewer is coming in Phase 7.8.
              </p>
            </div>
          )}
        </div>
      </div>

      {/* Delete confirmation */}
      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title={`Delete "${entity.external_id}"?`}
        description="This will permanently delete the entity. All relations referencing this entity must be removed first."
        confirmLabel="Delete"
        isLoading={deleteMutation.isPending}
        onConfirm={() => { deleteMutation.mutate(undefined) }}
      />
    </FadeContent>
  )
}
