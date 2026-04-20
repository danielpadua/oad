import { useState } from "react"
import { Link, useParams, useNavigate } from "react-router-dom"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { ArrowLeft } from "lucide-react"

import { http } from "@/lib/http-client"
import type { HttpError } from "@/lib/http-client"
import { toastSuccess, toastMutationError } from "@/lib/toast"
import type {
  System,
  SystemOverlaySchema,
  EntityTypeDefinition,
  ListResponse,
} from "@/lib/types"
import { FadeContent, Stepper, ClickSpark } from "@/components/reactbits"
import type { StepConfig } from "@/components/reactbits"
import { PropertyBuilder } from "@/components/ui/property-builder"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"

// ─── Queries ──────────────────────────────────────────────────────────────────

function useSystem(id: string) {
  return useQuery<System, HttpError>({
    queryKey: ["system", id],
    queryFn: () => http.get<System>(`/api/v1/systems/${id}`),
    enabled: !!id,
  })
}

function useEntityTypes() {
  return useQuery<ListResponse<EntityTypeDefinition>, HttpError>({
    queryKey: ["entity-types"],
    queryFn: () =>
      http.get<ListResponse<EntityTypeDefinition>>("/api/v1/entity-types"),
  })
}

function useOverlaySchema(systemId: string, schemaId: string | undefined) {
  return useQuery<SystemOverlaySchema, HttpError>({
    queryKey: ["overlay-schema", systemId, schemaId],
    queryFn: () =>
      http.get<SystemOverlaySchema>(
        `/api/v1/systems/${systemId}/overlay-schemas/${schemaId!}`
      ),
    enabled: !!schemaId,
  })
}

// ─── Constants ────────────────────────────────────────────────────────────────

const DEFAULT_OVERLAY = `{
  "type": "object",
  "properties": {}
}`

// ─── Form content ─────────────────────────────────────────────────────────────

interface FormContentProps {
  system: System
  schema?: SystemOverlaySchema
  entityTypes: EntityTypeDefinition[]
}

function OverlaySchemaFormContent({
  system,
  schema,
  entityTypes,
}: FormContentProps) {
  const isEdit = !!schema
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const entityTypeMap = new Map(entityTypes.map((et) => [et.id, et.type_name]))

  const steps: StepConfig[] = isEdit
    ? [
        {
          title: "Overlay Properties",
          description: "Define namespaced overlay properties",
        },
      ]
    : [
        {
          title: "Entity Type",
          description: "Select the entity type to extend",
        },
        {
          title: "Overlay Properties",
          description: "Define namespaced overlay properties",
        },
      ]

  const propsStepIndex = isEdit ? 0 : 1

  const [currentStep, setCurrentStep] = useState(0)
  const [entityTypeId, setEntityTypeId] = useState(
    schema?.entity_type_id ?? ""
  )
  const [entityTypeError, setEntityTypeError] = useState<string | null>(null)
  const [propsJson, setPropsJson] = useState(() =>
    schema?.allowed_overlay_properties != null
      ? JSON.stringify(schema.allowed_overlay_properties, null, 2)
      : DEFAULT_OVERLAY
  )
  const [propsError, setPropsError] = useState<string | null>(null)

  const cancelTo = `/systems/${system.id}`

  // ─── Mutations ──────────────────────────────────────────────────────────────

  const createMutation = useMutation({
    mutationFn: (payload: {
      entity_type_id: string
      allowed_overlay_properties: unknown
    }) =>
      http.post<SystemOverlaySchema>(
        `/api/v1/systems/${system.id}/overlay-schemas`,
        payload
      ),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["overlay-schemas", system.id] })
      toastSuccess("Overlay schema created")
      navigate(cancelTo)
    },
    onError: toastMutationError,
  })

  const updateMutation = useMutation({
    mutationFn: (payload: { allowed_overlay_properties: unknown }) =>
      http.put<SystemOverlaySchema>(
        `/api/v1/systems/${system.id}/overlay-schemas/${schema!.id}`,
        payload
      ),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["overlay-schemas", system.id] })
      queryClient.invalidateQueries({
        queryKey: ["overlay-schema", system.id, schema!.id],
      })
      toastSuccess("Overlay schema updated")
      navigate(cancelTo)
    },
    onError: toastMutationError,
  })

  const isPending = createMutation.isPending || updateMutation.isPending

  // ─── Navigation ─────────────────────────────────────────────────────────────

  function handleNext() {
    if (!isEdit && currentStep === 0) {
      if (!entityTypeId) {
        setEntityTypeError("Please select an entity type.")
        return
      }
      setEntityTypeError(null)
      setCurrentStep(1)
    }
  }

  function handleSubmit() {
    let parsedSchema: unknown
    try {
      parsedSchema = JSON.parse(propsJson)
      setPropsError(null)
    } catch {
      setPropsError("Invalid JSON — please fix the schema.")
      return
    }

    if (isEdit) {
      updateMutation.mutate({ allowed_overlay_properties: parsedSchema })
    } else {
      createMutation.mutate({
        entity_type_id: entityTypeId,
        allowed_overlay_properties: parsedSchema,
      })
    }
  }

  const isLastStep = currentStep === steps.length - 1

  // Active entity type name (for create step 0 preview + namespace hint)
  const selectedTypeName = entityTypeId
    ? (entityTypeMap.get(entityTypeId) ?? entityTypeId)
    : null
  const displayTypeName = isEdit
    ? (entityTypeMap.get(schema.entity_type_id) ?? schema.entity_type_id)
    : selectedTypeName

  // ─── Step content ────────────────────────────────────────────────────────────

  function renderStepContent() {
    // Step 0 (create only): entity type selector
    if (!isEdit && currentStep === 0) {
      return (
        <div className="max-w-lg space-y-5">
          <div className="space-y-1.5">
            <Label>Entity Type</Label>
            <Select
              value={entityTypeId}
              onValueChange={(v) => {
                setEntityTypeId(v)
                setEntityTypeError(null)
              }}
            >
              <SelectTrigger className="w-full">
                <SelectValue placeholder="Select entity type…" />
              </SelectTrigger>
              <SelectContent>
                {entityTypes.map((et) => (
                  <SelectItem key={et.id} value={et.id}>
                    <span className="font-mono">{et.type_name}</span>
                    <span className="ml-2 text-xs text-muted-foreground capitalize">
                      ({et.scope})
                    </span>
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {entityTypeError && (
              <p className="text-sm font-medium text-destructive">
                {entityTypeError}
              </p>
            )}
          </div>

          {selectedTypeName && (
            <div className="rounded-lg border border-border bg-muted/40 px-3 py-2 text-sm text-muted-foreground">
              All property keys must be prefixed with{" "}
              <code className="rounded bg-background px-1 py-0.5 font-mono text-xs text-foreground">
                {system.name}.
              </code>{" "}
              Example:{" "}
              <code className="rounded bg-background px-1 py-0.5 font-mono text-xs text-foreground">
                {system.name}.{selectedTypeName}_field
              </code>
            </div>
          )}
        </div>
      )
    }

    // Step for properties (propsStepIndex)
    if (currentStep === propsStepIndex) {
      return (
        <div className="space-y-4">
          <div className="rounded-lg border border-border bg-muted/40 px-3 py-2 text-sm text-muted-foreground">
            All property keys must be prefixed with{" "}
            <code className="rounded bg-background px-1 py-0.5 font-mono text-xs text-foreground">
              {system.name}.
            </code>
            {displayTypeName && (
              <>
                {" "}
                Example:{" "}
                <code className="rounded bg-background px-1 py-0.5 font-mono text-xs text-foreground">
                  {system.name}.{displayTypeName}_field
                </code>
              </>
            )}
          </div>

          <p className="text-sm text-muted-foreground">
            Define which overlay properties{" "}
            <strong className="text-foreground">{system.name}</strong> may
            attach to entities of type{" "}
            <code className="rounded bg-muted px-1 py-0.5 font-mono text-xs">
              {displayTypeName ?? "—"}
            </code>
            . Use the visual editor for common cases or switch to Raw JSON for
            full JSON Schema control.
          </p>

          <PropertyBuilder
            value={propsJson}
            onChange={(v) => {
              setPropsJson(v)
              setPropsError(null)
            }}
          />

          {propsError && (
            <p className="text-sm font-medium text-destructive">{propsError}</p>
          )}
        </div>
      )
    }

    return null
  }

  // ─── Render ──────────────────────────────────────────────────────────────────

  return (
    <div className="flex gap-12">
      {/* Stepper sidebar */}
      <div className="w-52 shrink-0 pt-1">
        <Stepper steps={steps} currentStep={currentStep} />
      </div>

      {/* Content + actions */}
      <div className="min-w-0 flex-1">
        {renderStepContent()}

        <div className="mt-10 flex items-center justify-between border-t border-border pt-6">
          <Button variant="outline" asChild disabled={isPending}>
            <Link to={cancelTo}>Cancel</Link>
          </Button>
          <div className="flex gap-2">
            {currentStep > 0 && (
              <Button
                variant="outline"
                onClick={() => setCurrentStep((s) => s - 1)}
                disabled={isPending}
              >
                Previous
              </Button>
            )}
            {isLastStep ? (
              <ClickSpark color="hsl(var(--primary))" disabled={isPending}>
                <Button onClick={handleSubmit} disabled={isPending}>
                  {isPending ? "Saving…" : isEdit ? "Save Changes" : "Create"}
                </Button>
              </ClickSpark>
            ) : (
              <Button onClick={handleNext} disabled={isPending}>
                Next
              </Button>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function OverlaySchemaFormPage() {
  const { id, schemaId } = useParams<{ id: string; schemaId?: string }>()
  const isEdit = !!schemaId

  const { data: system, isLoading: systemLoading, isError: systemError } =
    useSystem(id!)
  const { data: etdData, isLoading: etdLoading } = useEntityTypes()
  const {
    data: schema,
    isLoading: schemaLoading,
    isError: schemaError,
  } = useOverlaySchema(id!, schemaId)

  const isLoading =
    systemLoading || etdLoading || (isEdit && schemaLoading)

  if (isLoading) {
    return (
      <div className="space-y-8">
        <Skeleton className="h-8 w-48" />
        <div className="space-y-1">
          <Skeleton className="h-7 w-64" />
          <Skeleton className="h-4 w-96" />
        </div>
        <div className="flex gap-12">
          <Skeleton className="h-40 w-52 shrink-0" />
          <div className="flex-1 space-y-4">
            <Skeleton className="h-10 w-full max-w-lg" />
            <Skeleton className="h-48 w-full" />
          </div>
        </div>
      </div>
    )
  }

  if (systemError || !system) {
    return (
      <div className="flex flex-col items-center gap-4 py-16 text-center">
        <p className="text-muted-foreground">System not found.</p>
        <Button variant="outline" asChild>
          <Link to="/systems">Back to Systems</Link>
        </Button>
      </div>
    )
  }

  if (isEdit && (schemaError || !schema)) {
    return (
      <div className="flex flex-col items-center gap-4 py-16 text-center">
        <p className="text-muted-foreground">Overlay schema not found.</p>
        <Button variant="outline" asChild>
          <Link to={`/systems/${id}`}>{system.name}</Link>
        </Button>
      </div>
    )
  }

  const entityTypes = etdData?.items ?? []

  return (
    <FadeContent duration={0.35} className="space-y-8">
      {/* Breadcrumb */}
      <Button variant="ghost" size="sm" asChild className="-ml-2">
        <Link to={`/systems/${id}`}>
          <ArrowLeft className="mr-1.5 size-4" />
          {system.name}
        </Link>
      </Button>

      {/* Title */}
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">
          {isEdit ? "Edit Overlay Schema" : "Add Overlay Schema"}
        </h1>
        <p className="mt-1 text-sm text-muted-foreground">
          {isEdit
            ? `Update the allowed overlay properties for ${system.name}.`
            : `Define which overlay properties ${system.name} may attach to entities of a specific type.`}
        </p>
      </div>

      <OverlaySchemaFormContent
        key={schemaId ?? "new"}
        system={system}
        schema={schema}
        entityTypes={entityTypes}
      />
    </FadeContent>
  )
}
