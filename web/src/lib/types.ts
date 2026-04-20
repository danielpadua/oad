export interface EntityTypeDefinition {
  id: string
  type_name: string
  allowed_properties: unknown
  allowed_relations: unknown
  scope: "global" | "system_scoped"
  created_at: string
  updated_at: string
}

export interface System {
  id: string
  name: string
  description?: string
  active: boolean
  created_at: string
  updated_at: string
}

export interface SystemOverlaySchema {
  id: string
  system_id: string
  entity_type_id: string
  allowed_overlay_properties: unknown
  created_at: string
  updated_at: string
}

export interface ListResponse<T> {
  items: T[]
  total: number
}

export interface PaginatedResponse<T> {
  items: T[]
  total: number
  limit: number
  offset: number
}

export interface Entity {
  id: string
  type_id: string
  type: string
  external_id: string
  properties: Record<string, unknown>
  system_id?: string
  created_at: string
  updated_at: string
}

export interface Relation {
  id: string
  subject_entity_id: string
  relation_type: string
  target_entity_id: string
  system_id?: string
  created_at: string
}

export interface PropertyOverlay {
  id: string
  entity_id: string
  system_id: string
  properties: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface RelationRef {
  id: string
  relation_type: string
  target_entity_id: string
  system_id?: string
}

export interface MergedEntityView {
  id: string
  type_id: string
  type: string
  external_id: string
  properties: Record<string, unknown>
  relations: RelationRef[]
  system_id?: string
  created_at: string
  updated_at: string
}

export interface BulkImportError {
  index: number
  error: string
}

export interface BulkImportResult {
  total: number
  created: number
  updated: number
  errors: BulkImportError[]
}

export interface AuditLogEntry {
  id: string
  actor: string
  operation: "create" | "update" | "delete"
  resource_type: string
  resource_id: string
  before_value: Record<string, unknown> | null
  after_value: Record<string, unknown> | null
  system_id: string | null
  timestamp: string
}

export interface WebhookSubscription {
  id: string
  system_id: string
  callback_url: string
  active: boolean
  created_at: string
  updated_at: string
}

export interface WebhookDelivery {
  id: string
  subscription_id: string
  audit_log_id: string
  status: "pending" | "delivered" | "failed"
  attempts: number
  next_retry_at: string | null
  last_response_code: number | null
  created_at: string
}
