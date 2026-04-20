import { useState, useEffect } from "react"
import { Link, useParams, useNavigate } from "react-router-dom"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { ArrowLeft } from "lucide-react"

import { http } from "@/lib/http-client"
import type { HttpError } from "@/lib/http-client"
import { toastSuccess, toastMutationError } from "@/lib/toast"
import type { Entity, EntityTypeDefinition, ListResponse } from "@/lib/types"
import { FadeContent, ClickSpark } from "@/components/reactbits"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Checkbox } from "@/components/ui/checkbox"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import { JsonSchemaEditor } from "@/components/ui/json-schema-editor"
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

function useEntityType(id: string | undefined) {
  return useQuery<EntityTypeDefinition, HttpError>({
    queryKey: ["entity-type", id],
    queryFn: () => http.get<EntityTypeDefinition>(`/api/v1/entity-types/${id!}`),
    enabled: !!id,
  })
}

function useEntity(id: string | undefined) {
  return useQuery<Entity, HttpError>({
    queryKey: ["entity", id],
    queryFn: () => http.get<Entity>(`/api/v1/entities/${id!}`),
    enabled: !!id,
  })
}

// ─── Dynamic property form ────────────────────────────────────────────────────

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

function parseAllowedProperties(raw: unknown): ParsedSchema | null {
  try {
    const schema =
      typeof raw === "string" ? (JSON.parse(raw) as Record<string, unknown>) : (raw as Record<string, unknown>)
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

interface DynamicFieldProps {
  name: string
  prop: SchemaProp
  value: unknown
  onChange: (v: unknown) => void
}

function DynamicField({ name, prop, value, onChange }: DynamicFieldProps) {
  const labelText = prop.description ? `${name} — ${prop.description}` : name
  const isRequired = prop.required

  if (prop.type === "boolean") {
    return (
      <div className="flex items-center gap-2">
        <Checkbox
          id={`prop-${name}`}
          checked={!!value}
          onCheckedChange={(c) => onChange(!!c)}
        />
        <Label htmlFor={`prop-${name}`} className="text-sm">
          {labelText}
          {isRequired && <span className="ml-1 text-destructive">*</span>}
        </Label>
      </div>
    )
  }

  if (prop.enum?.length) {
    return (
      <div className="space-y-1.5">
        <Label htmlFor={`prop-${name}`} className="text-sm">
          {labelText}
          {isRequired && <span className="ml-1 text-destructive">*</span>}
        </Label>
        <Select
          value={String(value ?? "")}
          onValueChange={onChange}
        >
          <SelectTrigger id={`prop-${name}`}>
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
        <Label htmlFor={`prop-${name}`} className="text-sm">
          {labelText}
          {isRequired && <span className="ml-1 text-destructive">*</span>}
        </Label>
        <Input
          id={`prop-${name}`}
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
        <Label htmlFor={`prop-${name}`} className="text-sm">
          {labelText}
          {isRequired && <span className="ml-1 text-destructive">*</span>}
        </Label>
        <Input
          id={`prop-${name}`}
          value={typeof value === "string" ? value : ""}
          minLength={prop.minLength}
          maxLength={prop.maxLength}
          onChange={(e) => onChange(e.target.value)}
        />
      </div>
    )
  }

  // Fallback: raw JSON for arrays, objects, etc.
  return (
    <div className="space-y-1.5">
      <Label htmlFor={`prop-${name}`} className="text-sm">
        {labelText}
        {isRequired && <span className="ml-1 text-destructive">*</span>}
        <span className="ml-1 text-xs text-muted-foreground">({prop.type} — raw JSON)</span>
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

interface DynamicPropertyFormProps {
  schema: unknown
  value: Record<string, unknown>
  onChange: (v: Record<string, unknown>) => void
}

function DynamicPropertyForm({ schema, value, onChange }: DynamicPropertyFormProps) {
  const parsed = parseAllowedProperties(schema)
  const [rawJson, setRawJson] = useState(JSON.stringify(value, null, 2))
  const [rawMode, setRawMode] = useState(!parsed)

  useEffect(() => {
    if (!rawMode) {
      setRawJson(JSON.stringify(value, null, 2))
    }
  }, [value, rawMode])

  if (rawMode) {
    return (
      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <p className="text-xs text-muted-foreground">Raw JSON properties.</p>
          {parsed && (
            <Button variant="ghost" size="sm" onClick={() => setRawMode(false)}>
              Visual editor
            </Button>
          )}
        </div>
        <JsonSchemaEditor
          value={rawJson}
          onChange={(v) => {
            setRawJson(v)
            try {
              onChange(JSON.parse(v) as Record<string, unknown>)
            } catch {
              // keep stale until valid JSON
            }
          }}
          minHeight="180px"
          aria-label="Entity properties"
        />
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-end">
        <Button variant="ghost" size="sm" onClick={() => setRawMode(true)}>
          Raw JSON
        </Button>
      </div>
      {Object.entries(parsed!.properties).map(([key, prop]) => (
        <DynamicField
          key={key}
          name={key}
          prop={prop}
          value={value[key]}
          onChange={(v) => onChange({ ...value, [key]: v })}
        />
      ))}
    </div>
  )
}

// ─── Form content ─────────────────────────────────────────────────────────────

interface FormContentProps {
  entity?: Entity
}

function EntityFormContent({ entity }: FormContentProps) {
  const isEdit = !!entity
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const { data: typesData } = useEntityTypes()
  const allTypes = typesData?.items ?? []

  const [selectedTypeId, setSelectedTypeId] = useState(entity?.type_id ?? "")
  const [externalId, setExternalId] = useState(entity?.external_id ?? "")
  const [properties, setProperties] = useState<Record<string, unknown>>(
    entity?.properties ?? {}
  )
  const [errors, setErrors] = useState<Record<string, string>>({})

  const { data: selectedTypeDef } = useEntityType(selectedTypeId || undefined)

  const createMutation = useMutation({
    mutationFn: (payload: {
      type: string
      external_id: string
      properties: unknown
    }) => http.post<Entity>("/api/v1/entities", payload),
    onSuccess: (created) => {
      queryClient.invalidateQueries({ queryKey: ["entities"] })
      toastSuccess("Entity created")
      navigate(`/entities/${created.id}`)
    },
    onError: toastMutationError,
  })

  const updateMutation = useMutation({
    mutationFn: (payload: { properties: unknown }) =>
      http.patch<Entity>(`/api/v1/entities/${entity!.id}`, payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["entity", entity!.id] })
      queryClient.invalidateQueries({ queryKey: ["entities"] })
      toastSuccess("Entity updated")
      navigate(`/entities/${entity!.id}`)
    },
    onError: toastMutationError,
  })

  const isPending = createMutation.isPending || updateMutation.isPending

  function validate(): boolean {
    const next: Record<string, string> = {}
    if (!isEdit && !selectedTypeId) next.type = "Entity type is required."
    if (!isEdit && !externalId.trim()) next.external_id = "External ID is required."
    setErrors(next)
    return Object.keys(next).length === 0
  }

  function handleSubmit() {
    if (!validate()) return
    if (isEdit) {
      updateMutation.mutate({ properties })
    } else {
      const typeName = allTypes.find((t) => t.id === selectedTypeId)?.type_name ?? ""
      createMutation.mutate({ type: typeName, external_id: externalId.trim(), properties })
    }
  }

  const cancelTo = isEdit ? `/entities/${entity!.id}` : "/entities"

  return (
    <div className="max-w-xl space-y-6">
      {/* Entity type */}
      {!isEdit && (
        <div className="space-y-1.5">
          <Label htmlFor="entity-type" className="text-sm font-medium">
            Entity Type <span className="text-destructive">*</span>
          </Label>
          <Select
            value={selectedTypeId}
            onValueChange={(v) => {
              setSelectedTypeId(v)
              setProperties({})
            }}
          >
            <SelectTrigger id="entity-type">
              <SelectValue placeholder="Select entity type…" />
            </SelectTrigger>
            <SelectContent>
              {allTypes.map((t) => (
                <SelectItem key={t.id} value={t.id}>
                  <span className="font-mono">{t.type_name}</span>
                  <span className="ml-2 text-xs text-muted-foreground capitalize">
                    ({t.scope})
                  </span>
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          {errors.type && (
            <p className="text-sm text-destructive">{errors.type}</p>
          )}
        </div>
      )}

      {/* External ID */}
      {!isEdit ? (
        <div className="space-y-1.5">
          <Label htmlFor="external-id" className="text-sm font-medium">
            External ID <span className="text-destructive">*</span>
          </Label>
          <Input
            id="external-id"
            placeholder="e.g. alice@example.com, resource-123"
            value={externalId}
            onChange={(e) => setExternalId(e.target.value)}
          />
          <p className="text-xs text-muted-foreground">
            Unique identifier within the entity type, provided by the source system.
          </p>
          {errors.external_id && (
            <p className="text-sm text-destructive">{errors.external_id}</p>
          )}
        </div>
      ) : (
        <div className="space-y-1.5">
          <Label className="text-sm font-medium text-muted-foreground">External ID</Label>
          <p className="font-mono text-sm">{entity!.external_id}</p>
        </div>
      )}

      {/* Properties */}
      <div className="space-y-2">
        <Label className="text-sm font-medium">Properties</Label>
        {selectedTypeDef || isEdit ? (
          <DynamicPropertyForm
            schema={selectedTypeDef?.allowed_properties ?? null}
            value={properties}
            onChange={setProperties}
          />
        ) : (
          <div className="rounded-lg border border-dashed border-border py-8 text-center text-sm text-muted-foreground">
            {isEdit ? "" : "Select an entity type to configure properties."}
          </div>
        )}
      </div>

      {/* Actions */}
      <div className="flex items-center justify-between border-t border-border pt-6">
        <Button variant="outline" asChild disabled={isPending}>
          <Link to={cancelTo}>Cancel</Link>
        </Button>
        <ClickSpark color="hsl(var(--primary))" disabled={isPending}>
          <Button onClick={handleSubmit} disabled={isPending}>
            {isPending ? "Saving…" : isEdit ? "Save Changes" : "Create Entity"}
          </Button>
        </ClickSpark>
      </div>
    </div>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function EntityFormPage() {
  const { id } = useParams<{ id?: string }>()
  const isEdit = !!id

  const { data: entity, isLoading, isError } = useEntity(id)

  if (isEdit && isLoading) {
    return (
      <div className="space-y-8">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-7 w-64" />
        <div className="space-y-4">
          <Skeleton className="h-10 w-full max-w-xl" />
          <Skeleton className="h-10 w-full max-w-xl" />
          <Skeleton className="h-32 w-full max-w-xl" />
        </div>
      </div>
    )
  }

  if (isEdit && (isError || !entity)) {
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
    <FadeContent duration={0.35} className="space-y-8">
      {/* Breadcrumb */}
      <Button variant="ghost" size="sm" asChild className="-ml-2">
        <Link to={isEdit ? `/entities/${id}` : "/entities"}>
          <ArrowLeft className="mr-1.5 size-4" />
          {isEdit ? entity!.external_id : "Entities"}
        </Link>
      </Button>

      {/* Title */}
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">
          {isEdit ? `Edit: ${entity!.external_id}` : "Create Entity"}
        </h1>
        <p className="mt-1 text-sm text-muted-foreground">
          {isEdit
            ? "Update the properties of this entity."
            : "Define the type, external ID, and initial properties."}
        </p>
      </div>

      <EntityFormContent key={id ?? "new"} entity={entity} />
    </FadeContent>
  )
}
