import { useState } from "react"
import { Link } from "react-router-dom"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { Plus, Pencil, PowerOff, Power, Eye, Network } from "lucide-react"
import { useTranslation } from "react-i18next"

import { http } from "@/lib/http-client"
import type { HttpError } from "@/lib/http-client"
import { toastSuccess, toastMutationError } from "@/lib/toast"
import type { System, ListResponse } from "@/lib/types"
import { useAuth } from "@/contexts/AuthContext"
import { FadeContent, SpotlightCard, ClickSpark } from "@/components/reactbits"
import { Modal } from "@/components/ui/modal"
import { Drawer } from "@/components/ui/drawer"
import { ConfirmDialog } from "@/components/ui/confirm-dialog"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form"

// ─── Schemas ──────────────────────────────────────────────────────────────────

const createSchema = z.object({
  name: z.string().min(1, "Name is required"),
  description: z.string().optional(),
})

const editSchema = z.object({
  name: z.string().min(1, "Name is required"),
  description: z.string().optional(),
})

type CreateValues = z.infer<typeof createSchema>
type EditValues = z.infer<typeof editSchema>

// ─── Queries ─────────────────────────────────────────────────────────────────

function useSystems() {
  return useQuery<ListResponse<System>, HttpError>({
    queryKey: ["systems"],
    queryFn: () => http.get<ListResponse<System>>("/api/v1/systems"),
  })
}

function useCreateSystem() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (data: { name: string; description?: string }) =>
      http.post<System>("/api/v1/systems", data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["systems"] })
      toastSuccess("System registered")
    },
    onError: toastMutationError,
  })
}

function usePatchSystem() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({
      id,
      data,
    }: {
      id: string
      data: { name?: string; description?: string; active?: boolean }
    }) => http.patch<System>(`/api/v1/systems/${id}`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["systems"] })
      toastSuccess("System updated")
    },
    onError: toastMutationError,
  })
}

// ─── Register Modal ───────────────────────────────────────────────────────────

function RegisterSystemModal({
  open,
  onOpenChange,
}: {
  open: boolean
  onOpenChange: (o: boolean) => void
}) {
  const { t } = useTranslation("systems")
  const { t: tc } = useTranslation()
  const createMutation = useCreateSystem()
  const form = useForm<CreateValues>({
    resolver: zodResolver(createSchema),
    defaultValues: { name: "", description: "" },
  })

  function onSubmit(values: CreateValues) {
    createMutation.mutate(
      { name: values.name, description: values.description || undefined },
      {
        onSuccess: () => {
          form.reset()
          onOpenChange(false)
        },
      }
    )
  }

  return (
    <Modal
      open={open}
      onOpenChange={onOpenChange}
      title={t("form.register.title")}
      description={t("form.register.description")}
      footer={
        <div className="flex w-full flex-row items-center justify-between">
          <Button
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={createMutation.isPending}
          >
            {tc("actions.cancel")}
          </Button>
          <ClickSpark
            color="hsl(var(--primary))"
            disabled={createMutation.isPending}
          >
            <Button
              onClick={form.handleSubmit(onSubmit)}
              disabled={createMutation.isPending}
            >
              {createMutation.isPending
                ? t("form.submit.registering")
                : t("form.submit.register")}
            </Button>
          </ClickSpark>
        </div>
      }
    >
      <Form {...form}>
        <div className="space-y-4 py-2">
          <FormField
            control={form.control}
            name="name"
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t("form.fields.name")}</FormLabel>
                <FormControl>
                  <Input placeholder={t("form.fields.namePlaceholder")} {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name="description"
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t("form.fields.description")}</FormLabel>
                <FormControl>
                  <Textarea
                    placeholder={t("form.fields.descriptionPlaceholder")}
                    rows={3}
                    {...field}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        </div>
      </Form>
    </Modal>
  )
}

// ─── Edit Drawer ──────────────────────────────────────────────────────────────

function EditSystemDrawer({
  system,
  open,
  nameReadOnly,
  onOpenChange,
}: {
  system: System | undefined
  open: boolean
  nameReadOnly: boolean
  onOpenChange: (o: boolean) => void
}) {
  const { t } = useTranslation("systems")
  const { t: tc } = useTranslation()
  const patchMutation = usePatchSystem()
  const form = useForm<EditValues>({
    resolver: zodResolver(editSchema),
    values: { name: system?.name ?? "", description: system?.description ?? "" },
  })

  function onSubmit(values: EditValues) {
    if (!system) return
    const data: { name?: string; description?: string } = {
      description: values.description || undefined,
    }
    if (!nameReadOnly) data.name = values.name
    patchMutation.mutate(
      { id: system.id, data },
      { onSuccess: () => onOpenChange(false) }
    )
  }

  return (
    <Drawer
      open={open}
      onOpenChange={onOpenChange}
      title={`${t("form.edit.title")}: ${system?.name}`}
      width="md"
      footer={
        <div className="flex w-full gap-2 justify-end">
          <Button
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={patchMutation.isPending}
          >
            {tc("actions.cancel")}
          </Button>
          <ClickSpark
            color="hsl(var(--primary))"
            disabled={patchMutation.isPending}
          >
            <Button
              onClick={form.handleSubmit(onSubmit)}
              disabled={patchMutation.isPending}
            >
              {patchMutation.isPending
                ? t("form.submit.saving")
                : t("form.submit.save")}
            </Button>
          </ClickSpark>
        </div>
      }
    >
      <Form {...form}>
        <div className="space-y-4">
          <FormField
            control={form.control}
            name="name"
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t("form.fields.name")}</FormLabel>
                <FormControl>
                  <Input {...field} disabled={nameReadOnly} readOnly={nameReadOnly} />
                </FormControl>
                {nameReadOnly && (
                  <p className="text-xs text-muted-foreground">
                    {t("form.fields.renameRestricted")}
                  </p>
                )}
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name="description"
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t("form.fields.description")}</FormLabel>
                <FormControl>
                  <Textarea rows={3} {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        </div>
      </Form>
    </Drawer>
  )
}

// ─── System Card ──────────────────────────────────────────────────────────────

function SystemCard({
  system,
  canToggleActive,
  onEdit,
  onActivate,
  onDeactivate,
}: {
  system: System
  canToggleActive: boolean
  onEdit: (s: System) => void
  onActivate: (s: System) => void
  onDeactivate: (s: System) => void
}) {
  const { t } = useTranslation("systems")
  const { t: tc } = useTranslation()

  return (
    <SpotlightCard
      spotlightColor={
        system.active
          ? "rgba(129,140,248,0.08)"
          : "rgba(100,116,139,0.06)"
      }
    >
      <div className="flex items-start justify-between gap-3">
        <div className="flex items-center gap-3">
          <div className="flex size-10 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
            <Network className="size-5" />
          </div>
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <p className="truncate font-semibold">{system.name}</p>
              <Badge variant={system.active ? "default" : "secondary"}>
                {system.active ? tc("status.active") : tc("status.inactive")}
              </Badge>
            </div>
            {system.description && (
              <p className="mt-0.5 line-clamp-2 text-sm text-muted-foreground">
                {system.description}
              </p>
            )}
            <p className="mt-1 text-xs text-muted-foreground">
              Registered {new Date(system.created_at).toLocaleDateString()}
            </p>
          </div>
        </div>
      </div>
      <div className="mt-4 flex items-center gap-1 border-t border-border pt-3">
        <Button variant="ghost" size="sm" asChild>
          <Link to={`/systems/${system.id}`}>
            <Eye className="mr-1 size-3.5" />
            {t("actions.view")}
          </Link>
        </Button>
        <Button variant="ghost" size="sm" onClick={() => onEdit(system)}>
          <Pencil className="mr-1 size-3.5" />
          {t("actions.edit")}
        </Button>
        {canToggleActive &&
          (system.active ? (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => onDeactivate(system)}
            >
              <PowerOff className="mr-1 size-3.5 text-destructive" />
              {t("actions.deactivate")}
            </Button>
          ) : (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => onActivate(system)}
            >
              <Power className="mr-1 size-3.5 text-primary" />
              {t("actions.activate")}
            </Button>
          ))}
      </div>
    </SpotlightCard>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function Systems() {
  const { t } = useTranslation("systems")
  const { data, isLoading } = useSystems()
  const patchMutation = usePatchSystem()
  const { identity } = useAuth()
  const isPlatformAdmin = identity?.systemId == null

  const [registerOpen, setRegisterOpen] = useState(false)
  const [editTarget, setEditTarget] = useState<System | undefined>()
  const [deactivateTarget, setDeactivateTarget] = useState<System | undefined>()

  function handleActivate(system: System) {
    patchMutation.mutate({ id: system.id, data: { active: true } })
  }

  const systems = data?.items ?? []
  const activeSystems = systems.filter((s) => s.active)
  const inactiveSystems = systems.filter((s) => !s.active)

  return (
    <FadeContent duration={0.4} className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">{t("title")}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{t("subtitle")}</p>
        </div>
        {isPlatformAdmin && (
          <Button onClick={() => setRegisterOpen(true)}>
            <Plus className="mr-1.5 size-4" />
            {t("register")}
          </Button>
        )}
      </div>

      {/* Cards */}
      {isLoading ? (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-32 w-full rounded-xl" />
          ))}
        </div>
      ) : systems.length === 0 ? (
        <div className="flex flex-col items-center gap-4 py-16 text-center">
          <Network className="size-10 text-muted-foreground/40" />
          <div>
            <p className="font-medium">{t("empty")}</p>
            {isPlatformAdmin && (
              <p className="text-sm text-muted-foreground">{t("emptyHint")}</p>
            )}
          </div>
          {isPlatformAdmin && (
            <Button onClick={() => setRegisterOpen(true)}>
              <Plus className="mr-1.5 size-4" />
              {t("register")}
            </Button>
          )}
        </div>
      ) : (
        <div className="space-y-6">
          {activeSystems.length > 0 && (
            <section>
              <h2 className="mb-3 text-sm font-medium uppercase tracking-widest text-muted-foreground">
                {t("sections.active", { count: activeSystems.length })}
              </h2>
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                {activeSystems.map((sys) => (
                  <SystemCard
                    key={sys.id}
                    system={sys}
                    canToggleActive={isPlatformAdmin}
                    onEdit={setEditTarget}
                    onActivate={handleActivate}
                    onDeactivate={setDeactivateTarget}
                  />
                ))}
              </div>
            </section>
          )}
          {inactiveSystems.length > 0 && (
            <section>
              <h2 className="mb-3 text-sm font-medium uppercase tracking-widest text-muted-foreground">
                {t("sections.inactive", { count: inactiveSystems.length })}
              </h2>
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                {inactiveSystems.map((sys) => (
                  <SystemCard
                    key={sys.id}
                    system={sys}
                    canToggleActive={isPlatformAdmin}
                    onEdit={setEditTarget}
                    onActivate={handleActivate}
                    onDeactivate={setDeactivateTarget}
                  />
                ))}
              </div>
            </section>
          )}
        </div>
      )}

      {/* Register modal */}
      <RegisterSystemModal
        open={registerOpen}
        onOpenChange={setRegisterOpen}
      />

      {/* Edit drawer */}
      <EditSystemDrawer
        system={editTarget}
        open={!!editTarget}
        nameReadOnly={!isPlatformAdmin}
        onOpenChange={(o) => !o && setEditTarget(undefined)}
      />

      {/* Deactivate confirmation */}
      <ConfirmDialog
        open={!!deactivateTarget}
        onOpenChange={(o) => !o && setDeactivateTarget(undefined)}
        title={t("deactivate.title", { name: deactivateTarget?.name })}
        description={t("deactivate.description")}
        confirmLabel={t("actions.deactivate")}
        isLoading={patchMutation.isPending}
        onConfirm={async () => {
          if (deactivateTarget) {
            await patchMutation.mutateAsync({
              id: deactivateTarget.id,
              data: { active: false },
            })
            setDeactivateTarget(undefined)
          }
        }}
      />
    </FadeContent>
  )
}
