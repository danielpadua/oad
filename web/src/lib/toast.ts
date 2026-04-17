import { toast } from "sonner"
import { HttpError, NetworkError } from "@/lib/http-client"

// ─── Typed success/error helpers ─────────────────────────────────────────────

export function toastSuccess(message: string, description?: string) {
  toast.success(message, { description })
}

export function toastError(message: string, description?: string) {
  toast.error(message, { description })
}

export function toastInfo(message: string, description?: string) {
  toast.info(message, { description })
}

export function toastWarning(message: string, description?: string) {
  toast.warning(message, { description })
}

// ─── apierr code → user-facing message ───────────────────────────────────────

const API_ERROR_MESSAGES: Record<string, string> = {
  NOT_FOUND: "Resource not found",
  CONFLICT: "A conflict occurred",
  BAD_REQUEST: "Invalid request",
  VALIDATION_FAILED: "Validation failed",
  UNAUTHORIZED: "Session expired — please sign in again",
  FORBIDDEN: "You do not have permission to perform this action",
  UNPROCESSABLE_ENTITY: "The request could not be processed",
  INTERNAL_ERROR: "An unexpected server error occurred",
  SERVICE_UNAVAILABLE: "The service is temporarily unavailable",
  UNKNOWN: "An unexpected error occurred",
}

/**
 * Shows an error toast derived from an HttpError, NetworkError, or generic Error.
 * Maps known apierr codes to friendly messages and surfaces details when available.
 */
export function toastApiError(err: unknown, fallback?: string) {
  if (err instanceof HttpError) {
    const title = API_ERROR_MESSAGES[err.body.code] ?? err.body.message
    const details = Array.isArray(err.body.details)
      ? (err.body.details as string[]).join("; ")
      : typeof err.body.details === "string"
        ? err.body.details
        : undefined

    toast.error(title, {
      description: details ?? (err.body.message !== title ? err.body.message : undefined),
    })
    return
  }

  if (err instanceof NetworkError) {
    toast.error("Connection error", {
      description: "Check your network and try again.",
    })
    return
  }

  const message =
    err instanceof Error ? err.message : fallback ?? "An unexpected error occurred"
  toast.error(message)
}

/**
 * Convenience wrapper for mutation error handlers in TanStack Query.
 * Usage: `onError: toastMutationError`
 */
export const toastMutationError = (err: unknown) => toastApiError(err)
