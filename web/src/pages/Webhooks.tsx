import { useState } from "react"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { Webhook, Plus, Pencil, Trash2, CheckCircle2, XCircle, Clock } from "lucide-react"
import { z } from "zod"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"

import { http } from "@/lib/http-client"
import type { HttpError } from "@/lib/http-client"
import { toastSuccess, toastMutationError } from "@/lib/toast"
import type { WebhookSubscription, PaginatedResponse } from "@/lib/types"
import { useScope } from "@/contexts/ScopeContext"
import { FadeContent, ClickSpark } from "@/components/reactbits"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Modal } from "@/components/ui/modal"
import { ConfirmDialog } from "@/components/ui/confirm-dialog"
import { Skeleton } from "@/components/ui/skeleton"
import { Checkbox } from "@/components/ui/checkbox"
import { RequireRole } from "@/components/auth/RoleGate"

// ─── Schemas ──────────────────────────────────────────────────────────────────

const createSchema = z.object({
  callback_url: z.string().url("Must be a valid URL"),
  secret: z.string().min(16, "Secret must be at least 16 characters"),
})

const editSchema = z.object({
  callback_url: z.string().url("Must be a valid URL"),
  active: z.boolean(),
})

type CreateForm = z.infer<typeof createSchema>
type EditForm = z.infer<typeof editSchema>

// ─── Queries ──────────────────────────────────────────────────────────────────

function useWebhooks(systemId: string) {
  return useQuery<PaginatedResponse<WebhookSubscription>, HttpError>({
    queryKey: ["webhooks", systemId],
    queryFn: () =>
      http.get<PaginatedResponse<WebhookSubscription>>(
        `/api/v1/systems/${systemId}/webhooks?limit=100`
      ),
    staleTime: 30_000,
  })
}

// ─── Create modal ─────────────────────────────────────────────────────────────

interface CreateModalProps {
  open: boolean
  onOpenChange: (v: boolean) => void
  systemId: string
}

function CreateModal({ open, onOpenChange, systemId }: CreateModalProps) {
  const queryClient = useQueryClient()
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<CreateForm>({
    resolver: zodResolver(createSchema),
    defaultValues: { callback_url: "", secret: "" },
  })

  const mutation = useMutation({
    mutationFn: (data: CreateForm) =>
      http.post<WebhookSubscription>(
        `/api/v1/systems/${systemId}/webhooks`,
        data
      ),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["webhooks", systemId] })
      toastSuccess("Webhook subscription created")
      reset()
      onOpenChange(false)
    },
    onError: toastMutationError,
  })

  function onSubmit(data: CreateForm) {
    mutation.mutate(data)
  }

  return (
    <Modal
      open={open}
      onOpenChange={(v) => {
        if (!v) reset()
        onOpenChange(v)
      }}
      title="Register Webhook"
      size="md"
    >
      <form onSubmit={handleSubmit(onSubmit)} className="space-y-4 pt-2">
        <div className="space-y-1.5">
          <Label htmlFor="wh-url">Callback URL</Label>
          <Input
            id="wh-url"
            placeholder="https://example.com/api/events"
            {...register("callback_url")}
          />
          {errors.callback_url && (
            <p className="text-xs text-destructive">{errors.callback_url.message}</p>
          )}
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="wh-secret">Signing Secret</Label>
          <Input
            id="wh-secret"
            type="password"
            placeholder="At least 16 characters"
            {...register("secret")}
          />
          {errors.secret && (
            <p className="text-xs text-destructive">{errors.secret.message}</p>
          )}
          <p className="text-xs text-muted-foreground">
            Used to sign event payloads with HMAC-SHA256. Never returned by the
            API after creation.
          </p>
        </div>

        <div className="flex justify-end gap-2 border-t border-border pt-3">
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
          >
            Cancel
          </Button>
          <ClickSpark
            color="hsl(var(--primary))"
            disabled={mutation.isPending}
          >
            <Button type="submit" disabled={mutation.isPending}>
              {mutation.isPending ? "Creating…" : "Register"}
            </Button>
          </ClickSpark>
        </div>
      </form>
    </Modal>
  )
}

// ─── Edit modal ───────────────────────────────────────────────────────────────

interface EditModalProps {
  open: boolean
  onOpenChange: (v: boolean) => void
  subscription: WebhookSubscription
  systemId: string
}

function EditModal({
  open,
  onOpenChange,
  subscription,
  systemId,
}: EditModalProps) {
  const queryClient = useQueryClient()
  const {
    register,
    handleSubmit,
    watch,
    setValue,
    formState: { errors },
  } = useForm<EditForm>({
    resolver: zodResolver(editSchema),
    defaultValues: {
      callback_url: subscription.callback_url,
      active: subscription.active,
    },
  })

  const activeVal = watch("active")

  const mutation = useMutation({
    mutationFn: (data: EditForm) =>
      http.patch<WebhookSubscription>(
        `/api/v1/systems/${systemId}/webhooks/${subscription.id}`,
        data
      ),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["webhooks", systemId] })
      toastSuccess("Webhook updated")
      onOpenChange(false)
    },
    onError: toastMutationError,
  })

  return (
    <Modal
      open={open}
      onOpenChange={onOpenChange}
      title="Edit Webhook"
      size="md"
    >
      <form
        onSubmit={handleSubmit((data) => mutation.mutate(data))}
        className="space-y-4 pt-2"
      >
        <div className="space-y-1.5">
          <Label htmlFor="wh-edit-url">Callback URL</Label>
          <Input
            id="wh-edit-url"
            placeholder="https://example.com/api/events"
            {...register("callback_url")}
          />
          {errors.callback_url && (
            <p className="text-xs text-destructive">{errors.callback_url.message}</p>
          )}
        </div>

        <div className="flex items-center gap-2">
          <Checkbox
            id="wh-active"
            checked={activeVal}
            onCheckedChange={(v) => setValue("active", !!v)}
          />
          <Label htmlFor="wh-active" className="text-sm">
            Active — deliver events to this endpoint
          </Label>
        </div>

        <div className="rounded-lg border border-amber-500/30 bg-amber-500/10 px-3 py-2 text-xs text-amber-700 dark:text-amber-400">
          To rotate the signing secret, delete this subscription and create a new one.
        </div>

        <div className="flex justify-end gap-2 border-t border-border pt-3">
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
          >
            Cancel
          </Button>
          <ClickSpark
            color="hsl(var(--primary))"
            disabled={mutation.isPending}
          >
            <Button type="submit" disabled={mutation.isPending}>
              {mutation.isPending ? "Saving…" : "Save Changes"}
            </Button>
          </ClickSpark>
        </div>
      </form>
    </Modal>
  )
}

// ─── Subscription card ────────────────────────────────────────────────────────

interface SubscriptionCardProps {
  sub: WebhookSubscription
  systemId: string
}

function SubscriptionCard({ sub, systemId }: SubscriptionCardProps) {
  const queryClient = useQueryClient()
  const [editOpen, setEditOpen] = useState(false)
  const [deleteOpen, setDeleteOpen] = useState(false)

  const toggleMutation = useMutation({
    mutationFn: () =>
      http.patch<WebhookSubscription>(
        `/api/v1/systems/${systemId}/webhooks/${sub.id}`,
        { active: !sub.active }
      ),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["webhooks", systemId] })
      toastSuccess(sub.active ? "Webhook deactivated" : "Webhook activated")
    },
    onError: toastMutationError,
  })

  const deleteMutation = useMutation({
    mutationFn: () =>
      http.del(`/api/v1/systems/${systemId}/webhooks/${sub.id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["webhooks", systemId] })
      toastSuccess("Webhook deleted")
    },
    onError: toastMutationError,
  })

  return (
    <div className="rounded-lg border border-border bg-card px-4 py-4 space-y-3">
      {/* Top row */}
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            {sub.active ? (
              <Badge variant="default" className="gap-1 text-xs">
                <CheckCircle2 className="size-3" />
                Active
              </Badge>
            ) : (
              <Badge variant="secondary" className="gap-1 text-xs">
                <XCircle className="size-3" />
                Inactive
              </Badge>
            )}
            <code className="font-mono text-sm break-all">{sub.callback_url}</code>
          </div>
          <p className="mt-1 text-xs text-muted-foreground">
            ID:{" "}
            <span className="font-mono">{sub.id.slice(0, 8)}…</span>
            {" · "}
            Created {new Date(sub.created_at).toLocaleDateString()}
            {" · "}
            Updated {new Date(sub.updated_at).toLocaleString()}
          </p>
        </div>

        <RequireRole role="admin">
          <div className="flex shrink-0 gap-1">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => toggleMutation.mutate()}
              disabled={toggleMutation.isPending}
              title={sub.active ? "Deactivate" : "Activate"}
            >
              {sub.active ? (
                <XCircle className="size-3.5 text-muted-foreground" />
              ) : (
                <CheckCircle2 className="size-3.5 text-green-600" />
              )}
              <span className="sr-only">{sub.active ? "Deactivate" : "Activate"}</span>
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setEditOpen(true)}
            >
              <Pencil className="size-3.5" />
              <span className="sr-only">Edit</span>
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setDeleteOpen(true)}
            >
              <Trash2 className="size-3.5 text-destructive" />
              <span className="sr-only">Delete</span>
            </Button>
          </div>
        </RequireRole>
      </div>

      {/* Delivery history placeholder */}
      <div className="rounded-md border border-dashed border-border px-3 py-2 flex items-center gap-2 text-xs text-muted-foreground">
        <Clock className="size-3.5 shrink-0" />
        Delivery history endpoint is not yet available — recent attempts will appear here once FR-WHK-004 backend support ships.
      </div>

      {editOpen && (
        <EditModal
          open={editOpen}
          onOpenChange={setEditOpen}
          subscription={sub}
          systemId={systemId}
        />
      )}

      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title="Delete webhook?"
        description={`Remove subscription for "${sub.callback_url}"? All delivery history will also be deleted.`}
        confirmLabel="Delete"
        isLoading={deleteMutation.isPending}
        onConfirm={async () => {
          await deleteMutation.mutateAsync()
          setDeleteOpen(false)
        }}
      />
    </div>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function Webhooks() {
  const { activeSystemId } = useScope()
  const [createOpen, setCreateOpen] = useState(false)

  const { data, isLoading } = useWebhooks(activeSystemId ?? "")
  const subscriptions = data?.items ?? []

  return (
    <FadeContent duration={0.35} className="space-y-6">
      {/* Header */}
      <div className="flex items-start justify-between gap-4">
        <div>
          <div className="flex items-center gap-3">
            <Webhook className="size-6 text-muted-foreground" />
            <h1 className="text-2xl font-semibold tracking-tight">Webhooks</h1>
          </div>
          <p className="mt-1 text-sm text-muted-foreground">
            {activeSystemId
              ? "Manage event delivery subscriptions for this system."
              : "Select a system scope to manage webhook subscriptions."}
          </p>
        </div>

        {activeSystemId && (
          <RequireRole role="admin">
            <ClickSpark color="hsl(var(--primary))">
              <Button onClick={() => setCreateOpen(true)}>
                <Plus className="mr-1.5 size-4" />
                Register Webhook
              </Button>
            </ClickSpark>
          </RequireRole>
        )}
      </div>

      {/* No scope */}
      {!activeSystemId && (
        <div className="rounded-lg border border-border bg-muted/30 py-12 text-center">
          <Webhook className="mx-auto size-8 text-muted-foreground/50" />
          <p className="mt-3 text-sm font-medium">No system scope selected</p>
          <p className="mt-1 text-xs text-muted-foreground">
            Use the system selector in the top bar to choose a system. Webhooks
            are scoped to a specific system.
          </p>
        </div>
      )}

      {/* Subscription list */}
      {activeSystemId && (
        <div className="space-y-3">
          {isLoading ? (
            <div className="space-y-3">
              {Array.from({ length: 2 }).map((_, i) => (
                <Skeleton key={i} className="h-24 w-full rounded-lg" />
              ))}
            </div>
          ) : subscriptions.length === 0 ? (
            <div className="rounded-lg border border-dashed border-border py-14 text-center">
              <Webhook className="mx-auto size-8 text-muted-foreground/50" />
              <p className="mt-3 text-sm font-medium">No webhook subscriptions</p>
              <p className="mt-1 text-xs text-muted-foreground">
                Register a callback URL to receive real-time attribute change events.
              </p>
            </div>
          ) : (
            <div className="space-y-3">
              <p className="text-sm text-muted-foreground">
                {subscriptions.length}{" "}
                {subscriptions.length === 1 ? "subscription" : "subscriptions"}
              </p>
              {subscriptions.map((sub) => (
                <SubscriptionCard
                  key={sub.id}
                  sub={sub}
                  systemId={activeSystemId}
                />
              ))}
            </div>
          )}
        </div>
      )}

      {activeSystemId && (
        <CreateModal
          open={createOpen}
          onOpenChange={setCreateOpen}
          systemId={activeSystemId}
        />
      )}
    </FadeContent>
  )
}
