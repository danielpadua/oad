import { useState } from "react"
import { Link, useParams, useNavigate } from "react-router-dom"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { ArrowLeft } from "lucide-react"

import { http } from "@/lib/http-client"
import type { HttpError } from "@/lib/http-client"
import { toastSuccess, toastMutationError } from "@/lib/toast"
import type { EntityTypeDefinition } from "@/lib/types"
import { useAuth } from "@/contexts/AuthContext"
import { FadeContent, Stepper, ClickSpark } from "@/components/reactbits"
import type { StepConfig } from "@/components/reactbits"
import { PropertyBuilder } from "@/components/ui/property-builder"
import { RelationBuilder } from "@/components/ui/relation-builder"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"

// ─── Validation schema ────────────────────────────────────────────────────────

const basicInfoSchema = z.object({
  type_name: z
    .string()
    .min(1, "Type name is required")
    .regex(
      /^[a-z][a-z0-9_]*$/,
      "Lowercase letters, digits, underscores only — must start with a letter"
    ),
  scope: z.enum(["global", "system_scoped"], { message: "Scope is required" }),
})

type BasicInfoValues = z.infer<typeof basicInfoSchema>

const DEFAULT_PROPS = `{
  "type": "object",
  "properties": {}
}`
const DEFAULT_RELS = "{}"

// ─── Inner form (mounts only after data is ready) ────────────────────────────

interface FormContentProps {
  etd?: EntityTypeDefinition
}

function EntityTypeFormContent({ etd }: FormContentProps) {
  const isEdit = !!etd
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { identity } = useAuth()
  const isPlatformAdmin = identity?.systemId == null

  const steps: StepConfig[] = isEdit
    ? [
        { title: "Allowed Properties", description: "JSON Schema for entity properties" },
        { title: "Allowed Relations", description: "Permitted relation types and target types" },
      ]
    : [
        { title: "Basic Info", description: "Type name and scope" },
        { title: "Allowed Properties", description: "JSON Schema for entity properties" },
        { title: "Allowed Relations", description: "Permitted relation types and target types" },
      ]

  const propsStepIndex = isEdit ? 0 : 1
  const relsStepIndex = isEdit ? 1 : 2

  const [currentStep, setCurrentStep] = useState(0)
  const [propsJson, setPropsJson] = useState(() =>
    etd?.allowed_properties != null
      ? JSON.stringify(etd.allowed_properties, null, 2)
      : DEFAULT_PROPS
  )
  const [propsError, setPropsError] = useState<string | null>(null)
  const [relsJson, setRelsJson] = useState(() =>
    etd?.allowed_relations != null
      ? JSON.stringify(etd.allowed_relations, null, 2)
      : DEFAULT_RELS
  )
  const [relsError, setRelsError] = useState<string | null>(null)

  const form = useForm<BasicInfoValues>({
    resolver: zodResolver(basicInfoSchema),
    // System-scoped admins may only create system-scoped entity types;
    // start them on that scope and lock the field below.
    defaultValues: {
      type_name: "",
      scope: isPlatformAdmin ? "global" : "system_scoped",
    },
  })

  // ─── Mutations ──────────────────────────────────────────────────────────────

  const createMutation = useMutation({
    mutationFn: (payload: {
      type_name: string
      scope: string
      allowed_properties: unknown
      allowed_relations: unknown
    }) => http.post<EntityTypeDefinition>("/api/v1/entity-types", payload),
    onSuccess: (created) => {
      queryClient.invalidateQueries({ queryKey: ["entity-types"] })
      toastSuccess("Entity type created")
      navigate(`/entity-types/${created.id}`)
    },
    onError: toastMutationError,
  })

  const updateMutation = useMutation({
    mutationFn: (payload: {
      allowed_properties: unknown
      allowed_relations: unknown
    }) =>
      http.put<EntityTypeDefinition>(
        `/api/v1/entity-types/${etd!.id}`,
        payload
      ),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["entity-types"] })
      queryClient.invalidateQueries({ queryKey: ["entity-type", etd!.id] })
      toastSuccess("Entity type updated")
      navigate(`/entity-types/${etd!.id}`)
    },
    onError: toastMutationError,
  })

  const isPending = createMutation.isPending || updateMutation.isPending

  // ─── Navigation ─────────────────────────────────────────────────────────────

  function parseJson(
    value: string,
    setError: (msg: string | null) => void
  ): [unknown, boolean] {
    try {
      const parsed = JSON.parse(value)
      setError(null)
      return [parsed, true]
    } catch {
      setError("Invalid JSON — please fix before continuing.")
      return [null, false]
    }
  }

  async function handleNext() {
    if (!isEdit && currentStep === 0) {
      const valid = await form.trigger(["type_name", "scope"])
      if (valid) setCurrentStep(1)
      return
    }
    if (currentStep === propsStepIndex) {
      const [, ok] = parseJson(propsJson, setPropsError)
      if (ok) setCurrentStep(currentStep + 1)
    }
  }

  async function handleSubmit() {
    const [parsedProps, propsOk] = parseJson(propsJson, setPropsError)
    if (!propsOk) {
      setCurrentStep(propsStepIndex)
      return
    }
    const [parsedRels, relsOk] = parseJson(relsJson, setRelsError)
    if (!relsOk) {
      setCurrentStep(relsStepIndex)
      return
    }

    if (isEdit) {
      updateMutation.mutate({
        allowed_properties: parsedProps,
        allowed_relations: parsedRels,
      })
    } else {
      const { type_name, scope } = form.getValues()
      createMutation.mutate({
        type_name,
        scope,
        allowed_properties: parsedProps,
        allowed_relations: parsedRels,
      })
    }
  }

  const isLastStep = currentStep === steps.length - 1
  const cancelTo = isEdit ? `/entity-types/${etd!.id}` : "/entity-types"

  // ─── Step content ────────────────────────────────────────────────────────────

  function renderStepContent() {
    if (!isEdit && currentStep === 0) {
      return (
        <Form {...form}>
          <div className="max-w-lg space-y-5">
            <FormField
              control={form.control}
              name="type_name"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Type Name</FormLabel>
                  <FormControl>
                    <Input
                      placeholder="e.g. user, resource, department"
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    Lowercase letters, digits, and underscores. Must start with
                    a letter.
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="scope"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Scope</FormLabel>
                  <FormControl>
                    <Select
                      value={field.value}
                      onValueChange={field.onChange}
                      disabled={!isPlatformAdmin}
                    >
                      <SelectTrigger className="w-full">
                        <SelectValue placeholder="Select scope…" />
                      </SelectTrigger>
                      <SelectContent>
                        {isPlatformAdmin && (
                          <SelectItem value="global">Global</SelectItem>
                        )}
                        <SelectItem value="system_scoped">
                          System-Scoped
                        </SelectItem>
                      </SelectContent>
                    </Select>
                  </FormControl>
                  <FormDescription>
                    {isPlatformAdmin
                      ? "Global types are shared across all systems. System-scoped types are owned by a specific system."
                      : "System-scoped types are owned by a specific system. Global types are reserved for platform admins."}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>
        </Form>
      )
    }

    if (currentStep === propsStepIndex) {
      return (
        <div className="space-y-3">
          <p className="text-sm text-muted-foreground">
            Define which properties entities of this type may have. Use the
            visual editor for common cases or switch to Raw JSON for full JSON
            Schema control.
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

    if (currentStep === relsStepIndex) {
      return (
        <div className="space-y-3">
          <p className="text-sm text-muted-foreground">
            Declare which relation types entities of this type may participate
            in and which entity types they can point to.
          </p>
          <RelationBuilder
            value={relsJson}
            onChange={(v) => {
              setRelsJson(v)
              setRelsError(null)
            }}
          />
          {relsError && (
            <p className="text-sm font-medium text-destructive">{relsError}</p>
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
              <ClickSpark
                color="hsl(var(--primary))"
                disabled={isPending}
              >
                <Button onClick={handleSubmit} disabled={isPending}>
                  {isPending
                    ? "Saving…"
                    : isEdit
                      ? "Save Changes"
                      : "Create"}
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

// ─── Query ────────────────────────────────────────────────────────────────────

function useEntityType(id: string) {
  return useQuery<EntityTypeDefinition, HttpError>({
    queryKey: ["entity-type", id],
    queryFn: () =>
      http.get<EntityTypeDefinition>(`/api/v1/entity-types/${id}`),
    enabled: !!id,
  })
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function EntityTypeFormPage() {
  const { id } = useParams<{ id?: string }>()
  const isEdit = !!id

  const { data: etd, isLoading, isError } = useEntityType(id ?? "")

  if (isEdit && isLoading) {
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
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-48 w-full" />
          </div>
        </div>
      </div>
    )
  }

  if (isEdit && (isError || !etd)) {
    return (
      <div className="flex flex-col items-center gap-4 py-16 text-center">
        <p className="text-muted-foreground">Entity type not found.</p>
        <Button variant="outline" asChild>
          <Link to="/entity-types">Back to Entity Types</Link>
        </Button>
      </div>
    )
  }

  return (
    <FadeContent duration={0.35} className="space-y-8">
      {/* Breadcrumb */}
      <Button variant="ghost" size="sm" asChild className="-ml-2">
        <Link to={isEdit ? `/entity-types/${id}` : "/entity-types"}>
          <ArrowLeft className="mr-1.5 size-4" />
          {isEdit ? etd!.type_name : "Entity Types"}
        </Link>
      </Button>

      {/* Title */}
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">
          {isEdit ? `Edit: ${etd!.type_name}` : "Create Entity Type"}
        </h1>
        <p className="mt-1 text-sm text-muted-foreground">
          {isEdit
            ? "Update the allowed properties and relations for this entity type."
            : "Define the type name, scope, and the schema for properties and relations."}
        </p>
      </div>

      {/* Form — key forces fresh mount when navigating between different edit targets */}
      <EntityTypeFormContent key={id ?? "new"} etd={etd} />
    </FadeContent>
  )
}
