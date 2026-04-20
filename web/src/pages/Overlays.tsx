import { useState, useRef, useEffect } from "react"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { Search, Layers, Plus, Pencil, Trash2, X } from "lucide-react"

import { http } from "@/lib/http-client"
import type { HttpError } from "@/lib/http-client"
import { toastSuccess, toastMutationError } from "@/lib/toast"
import type {
  Entity,
  SystemOverlaySchema,
  PropertyOverlay,
  PaginatedResponse,
  ListResponse,
  System,
} from "@/lib/types"
import { useScope } from "@/contexts/ScopeContext"
import { FadeContent, ClickSpark } from "@/components/reactbits"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Modal } from "@/components/ui/modal"
import { ConfirmDialog } from "@/components/ui/confirm-dialog"
import { Skeleton } from "@/components/ui/skeleton"
import { JsonSchemaEditor } from "@/components/ui/json-schema-editor"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { RequireAnyRole } from "@/components/auth/RoleGate"

// ─── Schema parsing (mirrors EntityFormPage utilities) ───────────────────────

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

interface ParsedSchema {
  properties: Record<string, SchemaProp>
}

function parseSchema(raw: unknown): ParsedSchema | null {
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
    return Object.keys(result).length > 0 ? { properties: result } : null
  } catch {
    return null
  }
}

// ─── Dynamic overlay field ────────────────────────────────────────────────────

interface DynamicFieldProps {
  name: string
  prop: SchemaProp
  value: unknown
  onChange: (v: unknown) => void
}

function DynamicField({ name, prop, value, onChange }: DynamicFieldProps) {
  const labelText = prop.description ? `${name} — ${prop.description}` : name

  if (prop.type === "boolean") {
    return (
      <div className="flex items-center gap-2">
        <Checkbox
          id={`ovl-${name}`}
          checked={!!value}
          onCheckedChange={(c) => onChange(!!c)}
        />
        <Label htmlFor={`ovl-${name}`} className="font-mono text-sm">
          {labelText}
          {prop.required && <span className="ml-1 text-destructive">*</span>}
        </Label>
      </div>
    )
  }

  if (prop.enum?.length) {
    return (
      <div className="space-y-1.5">
        <Label htmlFor={`ovl-${name}`} className="font-mono text-sm">
          {labelText}
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
          {labelText}
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

  if (prop.type === "string") {
    return (
      <div className="space-y-1.5">
        <Label htmlFor={`ovl-${name}`} className="font-mono text-sm">
          {labelText}
          {prop.required && <span className="ml-1 text-destructive">*</span>}
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

  return (
    <div className="space-y-1.5">
      <Label className="font-mono text-sm">
        {labelText}
        {prop.required && <span className="ml-1 text-destructive">*</span>}
        <span className="ml-1 text-xs font-normal text-muted-foreground">
          ({prop.type} — raw JSON)
        </span>
      </Label>
      <JsonSchemaEditor
        value={value != null ? JSON.stringify(value, null, 2) : ""}
        onChange={(raw) => {
          try {
            onChange(JSON.parse(raw))
          } catch {
            // keep current until valid
          }
        }}
        minHeight="80px"
        aria-label={`Property ${name}`}
      />
    </div>
  )
}

// ─── Queries ──────────────────────────────────────────────────────────────────

function useEntitySearch(query: string, typeFilter: string) {
  return useQuery<PaginatedResponse<Entity>, HttpError>({
    queryKey: ["entity-search-overlays", query, typeFilter],
    queryFn: () => {
      const params = new URLSearchParams({ limit: "20" })
      if (typeFilter) params.set("type", typeFilter)
      return http.get<PaginatedResponse<Entity>>(`/api/v1/entities?${params}`)
    },
    enabled: query.length >= 2,
    staleTime: 30_000,
  })
}

function useEntityOverlays(entityId: string | null) {
  return useQuery<PaginatedResponse<PropertyOverlay>, HttpError>({
    queryKey: ["entity-overlays", entityId],
    queryFn: () =>
      http.get<PaginatedResponse<PropertyOverlay>>(
        `/api/v1/entities/${entityId!}/overlays?limit=50`
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

// ─── Overlay Form Modal ───────────────────────────────────────────────────────

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
  const parsedSchema = overlaySchema
    ? parseSchema(overlaySchema.allowed_overlay_properties)
    : null

  const [values, setValues] = useState<Record<string, unknown>>(
    existing?.properties ?? {}
  )
  const [rawJson, setRawJson] = useState(
    JSON.stringify(existing?.properties ?? {}, null, 2)
  )
  const [rawMode, setRawMode] = useState(!parsedSchema)
  const [jsonError, setJsonError] = useState<string | null>(null)

  // Sync rawMode when schema loads
  useEffect(() => {
    if (!rawMode && !parsedSchema) setRawMode(true)
  }, [parsedSchema, rawMode])

  // Reset state when modal opens/closes
  useEffect(() => {
    if (open) {
      const init = existing?.properties ?? {}
      setValues(init)
      setRawJson(JSON.stringify(init, null, 2))
      setRawMode(!parsedSchema)
      setJsonError(null)
    }
  }, [open, existing, parsedSchema])

  const createMutation = useMutation({
    mutationFn: (properties: unknown) =>
      http.post<PropertyOverlay>(`/api/v1/entities/${entity.id}/overlays`, {
        properties,
      }),
    onSuccess: () => {
      toastSuccess("Overlay created")
      onSuccess()
      onOpenChange(false)
    },
    onError: toastMutationError,
  })

  const updateMutation = useMutation({
    mutationFn: (properties: unknown) =>
      http.put<PropertyOverlay>(
        `/api/v1/entities/${entity.id}/overlays/${existing!.id}`,
        { properties }
      ),
    onSuccess: () => {
      toastSuccess("Overlay updated")
      onSuccess()
      onOpenChange(false)
    },
    onError: toastMutationError,
  })

  const isPending = createMutation.isPending || updateMutation.isPending
  const isEdit = !!existing

  function handleSubmit() {
    if (rawMode) {
      try {
        const parsed = JSON.parse(rawJson) as unknown
        setJsonError(null)
        if (isEdit) {
          updateMutation.mutate(parsed)
        } else {
          createMutation.mutate(parsed)
        }
      } catch {
        setJsonError("Invalid JSON — fix the syntax before saving.")
      }
    } else {
      if (isEdit) {
        updateMutation.mutate(values)
      } else {
        createMutation.mutate(values)
      }
    }
  }

  function patchValue(key: string, v: unknown) {
    setValues((prev) => ({ ...prev, [key]: v }))
  }

  const title = isEdit ? "Edit Overlay" : "Create Overlay"
  const systemPrefix = system ? `${system.name}.` : ""

  return (
    <Modal open={open} onOpenChange={onOpenChange} title={title} size="md">
      <div className="space-y-4 pt-2">
        {/* Entity reference */}
        <div className="rounded-lg border border-border bg-muted/30 px-3 py-2">
          <p className="text-xs text-muted-foreground">Entity</p>
          <div className="mt-0.5 flex items-center gap-2">
            <span className="font-mono text-sm font-medium">
              {entity.external_id}
            </span>
            <Badge variant="outline" className="font-mono text-xs">
              {entity.type}
            </Badge>
          </div>
        </div>

        {/* System + namespace info */}
        {system && (
          <div className="rounded-lg border border-border bg-muted/30 px-3 py-2">
            <p className="text-xs text-muted-foreground">System scope</p>
            <div className="mt-0.5 flex items-center gap-2">
              <span className="text-sm font-medium">{system.name}</span>
              <span className="text-xs text-muted-foreground">
                · keys prefixed with{" "}
                <code className="rounded bg-muted px-1 font-mono text-xs">
                  {systemPrefix}
                </code>
              </span>
            </div>
          </div>
        )}

        {/* No schema warning */}
        {!overlaySchema && (
          <div className="rounded-lg border border-amber-500/30 bg-amber-500/10 px-3 py-2 text-xs text-amber-700 dark:text-amber-400">
            No overlay schema defined for this entity type. You can still
            provide raw JSON properties, but they won't be validated against a
            schema.
          </div>
        )}

        {/* Properties form */}
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <Label className="text-sm font-medium">Properties</Label>
            {parsedSchema && (
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
                onChange={(v) => {
                  setRawJson(v)
                  setJsonError(null)
                }}
                minHeight="180px"
                aria-label="Overlay properties JSON"
              />
              {jsonError && (
                <p className="text-sm font-medium text-destructive">
                  {jsonError}
                </p>
              )}
            </>
          ) : (
            parsedSchema && (
              <div className="space-y-3">
                {Object.entries(parsedSchema.properties).map(([key, prop]) => (
                  <DynamicField
                    key={key}
                    name={key}
                    prop={prop}
                    value={values[key]}
                    onChange={(v) => patchValue(key, v)}
                  />
                ))}
              </div>
            )
          )}
        </div>

        {/* Actions */}
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

// ─── Entity search combobox ───────────────────────────────────────────────────

interface EntitySearchProps {
  selected: Entity | null
  onSelect: (e: Entity | null) => void
}

function EntitySearch({ selected, onSelect }: EntitySearchProps) {
  const [query, setQuery] = useState("")
  const [showDropdown, setShowDropdown] = useState(false)
  const dropdownRef = useRef<HTMLDivElement>(null)
  const { data: results } = useEntitySearch(query, "")

  const filtered = (results?.items ?? []).filter((e) =>
    e.external_id.toLowerCase().includes(query.toLowerCase())
  )

  if (selected) {
    return (
      <div className="flex items-center justify-between rounded-lg border border-border bg-muted/30 px-3 py-2.5">
        <div className="flex items-center gap-3">
          <div>
            <p className="font-mono text-sm font-medium">{selected.external_id}</p>
            <p className="text-xs text-muted-foreground">{selected.type}</p>
          </div>
          <Badge variant="secondary" className="font-mono text-xs">
            {selected.type}
          </Badge>
        </div>
        <Button
          variant="ghost"
          size="sm"
          onClick={() => onSelect(null)}
          aria-label="Clear selection"
        >
          <X className="size-3.5" />
        </Button>
      </div>
    )
  }

  return (
    <div className="relative" ref={dropdownRef}>
      <Search className="absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
      <Input
        placeholder="Search entities by external ID (type at least 2 characters)…"
        value={query}
        className="pl-8"
        onChange={(e) => {
          setQuery(e.target.value)
          setShowDropdown(true)
        }}
        onFocus={() => setShowDropdown(true)}
      />
      {showDropdown && filtered.length > 0 && (
        <>
          <div
            className="fixed inset-0 z-10"
            onClick={() => setShowDropdown(false)}
            aria-hidden
          />
          <div className="absolute left-0 top-full z-20 mt-1 w-full rounded-lg border border-border bg-card py-1 shadow-lg">
            {filtered.map((e) => (
              <button
                key={e.id}
                className="flex w-full cursor-pointer flex-col px-3 py-2 text-left hover:bg-muted"
                onClick={() => {
                  onSelect(e)
                  setQuery("")
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
  )
}

// ─── Overlay card ─────────────────────────────────────────────────────────────

interface OverlayCardProps {
  overlay: PropertyOverlay
  onEdit: () => void
  onDelete: () => void
}

function OverlayCard({ overlay, onEdit, onDelete }: OverlayCardProps) {
  const propCount = Object.keys(overlay.properties ?? {}).length
  const propKeys = Object.keys(overlay.properties ?? {})

  return (
    <div className="rounded-lg border border-border bg-card px-4 py-3">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 flex-1 space-y-1.5">
          <div className="flex flex-wrap items-center gap-1.5">
            {propKeys.map((k) => (
              <Badge
                key={k}
                variant="outline"
                className="font-mono text-xs"
              >
                {k}
              </Badge>
            ))}
            {propCount === 0 && (
              <span className="text-xs text-muted-foreground">
                No properties
              </span>
            )}
          </div>
          <p className="text-xs text-muted-foreground">
            {propCount} {propCount === 1 ? "property" : "properties"} · Updated{" "}
            {new Date(overlay.updated_at).toLocaleString()}
          </p>
        </div>
        <div className="flex shrink-0 gap-1">
          <RequireAnyRole roles={["admin", "editor"]}>
            <Button variant="ghost" size="sm" onClick={onEdit}>
              <Pencil className="size-3.5" />
              <span className="sr-only">Edit</span>
            </Button>
            <Button variant="ghost" size="sm" onClick={onDelete}>
              <Trash2 className="size-3.5 text-destructive" />
              <span className="sr-only">Delete</span>
            </Button>
          </RequireAnyRole>
        </div>
      </div>
    </div>
  )
}

// ─── Entity overlay panel ─────────────────────────────────────────────────────

interface EntityOverlayPanelProps {
  entity: Entity
  systemId: string | null
}

function EntityOverlayPanel({ entity, systemId }: EntityOverlayPanelProps) {
  const queryClient = useQueryClient()
  const { data, isLoading } = useEntityOverlays(entity.id)
  const [createOpen, setCreateOpen] = useState(false)
  const [editTarget, setEditTarget] = useState<PropertyOverlay | undefined>()
  const [deleteTarget, setDeleteTarget] = useState<PropertyOverlay | undefined>()

  const deleteMutation = useMutation({
    mutationFn: (id: string) =>
      http.del(`/api/v1/entities/${entity.id}/overlays/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["entity-overlays", entity.id] })
      toastSuccess("Overlay deleted")
    },
    onError: toastMutationError,
  })

  const overlays = data?.items ?? []

  function onFormSuccess() {
    queryClient.invalidateQueries({ queryKey: ["entity-overlays", entity.id] })
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">
          {isLoading
            ? "Loading overlays…"
            : `${overlays.length} ${overlays.length === 1 ? "overlay" : "overlays"}`}
        </p>
        {systemId && (
          <RequireAnyRole roles={["admin", "editor"]}>
            <Button size="sm" onClick={() => setCreateOpen(true)}>
              <Plus className="mr-1.5 size-3.5" />
              Create Overlay
            </Button>
          </RequireAnyRole>
        )}
      </div>

      {isLoading ? (
        <div className="space-y-2">
          {Array.from({ length: 2 }).map((_, i) => (
            <Skeleton key={i} className="h-16 w-full" />
          ))}
        </div>
      ) : overlays.length === 0 ? (
        <div className="rounded-lg border border-dashed border-border py-10 text-center text-sm text-muted-foreground">
          No overlays found.{" "}
          {systemId
            ? 'Click "Create Overlay" to attach system-specific properties.'
            : "Select a system scope to manage overlays."}
        </div>
      ) : (
        <div className="space-y-2">
          {overlays.map((ov) => (
            <OverlayCard
              key={ov.id}
              overlay={ov}
              onEdit={() => setEditTarget(ov)}
              onDelete={() => setDeleteTarget(ov)}
            />
          ))}
        </div>
      )}

      {systemId && (
        <>
          {createOpen && (
            <OverlayFormModal
              open={createOpen}
              onOpenChange={setCreateOpen}
              entity={entity}
              systemId={systemId}
              onSuccess={onFormSuccess}
            />
          )}
          {editTarget && (
            <OverlayFormModal
              open={!!editTarget}
              onOpenChange={(v) => !v && setEditTarget(undefined)}
              entity={entity}
              systemId={systemId}
              existing={editTarget}
              onSuccess={onFormSuccess}
            />
          )}
        </>
      )}

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(o) => !o && setDeleteTarget(undefined)}
        title="Delete overlay?"
        description="Remove this overlay? System-specific properties on this entity will be lost."
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

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function Overlays() {
  const { activeSystemId } = useScope()
  const { data: system } = useSystem(activeSystemId)
  const [selectedEntity, setSelectedEntity] = useState<Entity | null>(null)

  return (
    <FadeContent duration={0.35} className="space-y-6">
      {/* Header */}
      <div className="flex items-start justify-between gap-4">
        <div>
          <div className="flex items-center gap-3">
            <Layers className="size-6 text-muted-foreground" />
            <h1 className="text-2xl font-semibold tracking-tight">Overlays</h1>
            {system && (
              <Badge variant="secondary">{system.name}</Badge>
            )}
          </div>
          <p className="mt-1 text-sm text-muted-foreground">
            {activeSystemId
              ? `Manage system-specific property overlays for ${system?.name ?? "the selected system"}.`
              : "Select a system scope in the top bar to manage overlays."}
          </p>
        </div>
      </div>

      {/* No scope warning */}
      {!activeSystemId && (
        <div className="rounded-lg border border-border bg-muted/30 py-12 text-center">
          <Layers className="mx-auto size-8 text-muted-foreground/50" />
          <p className="mt-3 text-sm font-medium">No system scope selected</p>
          <p className="mt-1 text-xs text-muted-foreground">
            Use the system selector in the top bar to choose a system. Overlays
            are scoped to a specific system.
          </p>
        </div>
      )}

      {activeSystemId && (
        <>
          {/* Entity search */}
          <div className="space-y-2">
            <Label className="text-sm font-medium">Find entity</Label>
            <p className="text-xs text-muted-foreground">
              Search for the entity you want to attach overlay properties to.
            </p>
            <EntitySearch
              selected={selectedEntity}
              onSelect={setSelectedEntity}
            />
          </div>

          {/* Overlay management for selected entity */}
          {selectedEntity ? (
            <div className="space-y-4">
              <div className="border-t border-border pt-4">
                <h2 className="flex items-center gap-2 text-base font-semibold">
                  <span className="font-mono">{selectedEntity.external_id}</span>
                  <Badge variant="outline" className="font-mono text-xs">
                    {selectedEntity.type}
                  </Badge>
                </h2>
                <p className="mt-1 text-xs text-muted-foreground">
                  ID:{" "}
                  <span className="font-mono">{selectedEntity.id}</span>
                </p>
              </div>
              <EntityOverlayPanel
                entity={selectedEntity}
                systemId={activeSystemId}
              />
            </div>
          ) : (
            <div className="rounded-lg border border-dashed border-border py-12 text-center">
              <Search className="mx-auto size-8 text-muted-foreground/50" />
              <p className="mt-3 text-sm font-medium">No entity selected</p>
              <p className="mt-1 text-xs text-muted-foreground">
                Search for an entity above to view and manage its overlays.
              </p>
            </div>
          )}
        </>
      )}
    </FadeContent>
  )
}
