import { http, HttpResponse } from "msw";
import type {
  EntityTypeDefinition,
  System,
  Entity,
  PropertyOverlay,
  WebhookSubscription,
  WebhookDelivery,
  AuditLogEntry,
  ListResponse,
  PaginatedResponse,
} from "@/lib/types";

// ─── Fixtures ─────────────────────────────────────────────────────────────────

export const mockEntityType: EntityTypeDefinition = {
  id: "et-1",
  type_name: "user",
  allowed_properties: { type: "object", properties: { email: { type: "string" } } },
  allowed_relations: ["member_of"],
  scope: "global",
  created_at: "2024-01-01T00:00:00Z",
  updated_at: "2024-01-01T00:00:00Z",
};

export const mockSystem: System = {
  id: "sys-1",
  name: "acme-hr",
  description: "HR system",
  active: true,
  created_at: "2024-01-01T00:00:00Z",
  updated_at: "2024-01-01T00:00:00Z",
};

export const mockEntity: Entity = {
  id: "ent-1",
  type_id: "et-1",
  type: "user",
  external_id: "alice@example.com",
  properties: { email: "alice@example.com" },
  created_at: "2024-01-01T00:00:00Z",
  updated_at: "2024-01-01T00:00:00Z",
};

export const mockOverlay: PropertyOverlay = {
  id: "ovl-1",
  entity_id: "ent-1",
  system_id: "sys-1",
  properties: { "acme_hr:department": "Engineering" },
  created_at: "2024-01-01T00:00:00Z",
  updated_at: "2024-01-01T00:00:00Z",
};

export const mockWebhook: WebhookSubscription = {
  id: "wh-1",
  system_id: "sys-1",
  callback_url: "https://hooks.example.com/oad",
  active: true,
  created_at: "2024-01-01T00:00:00Z",
  updated_at: "2024-01-01T00:00:00Z",
};

export const mockDelivery: WebhookDelivery = {
  id: "del-1",
  subscription_id: "wh-1",
  audit_log_id: "aud-1",
  status: "delivered",
  attempts: 1,
  next_retry_at: null,
  last_response_code: 200,
  created_at: "2024-01-01T00:00:00Z",
};

export const mockAuditEntry: AuditLogEntry = {
  id: "aud-1",
  actor: "admin@example.com",
  operation: "create",
  resource_type: "entity",
  resource_id: "ent-1",
  before_value: null,
  after_value: { external_id: "alice@example.com" },
  system_id: null,
  timestamp: "2024-01-01T00:00:00Z",
};

// ─── Handlers ─────────────────────────────────────────────────────────────────

export const handlers = [
  // Entity Types
  http.get("/api/v1/entity-types", () =>
    HttpResponse.json<ListResponse<EntityTypeDefinition>>({
      items: [mockEntityType],
      total: 1,
    }),
  ),
  http.get("/api/v1/entity-types/:id", ({ params }) => {
    if (params["id"] === mockEntityType.id) {
      return HttpResponse.json(mockEntityType);
    }
    return HttpResponse.json({ code: "NOT_FOUND", message: "Not found" }, { status: 404 });
  }),
  http.post("/api/v1/entity-types", () => HttpResponse.json(mockEntityType, { status: 201 })),
  http.put("/api/v1/entity-types/:id", () => HttpResponse.json(mockEntityType)),
  http.delete("/api/v1/entity-types/:id", () => new HttpResponse(null, { status: 204 })),

  // Systems
  http.get("/api/v1/systems", () =>
    HttpResponse.json<ListResponse<System>>({ items: [mockSystem], total: 1 }),
  ),
  http.get("/api/v1/systems/:id", ({ params }) => {
    if (params["id"] === mockSystem.id) return HttpResponse.json(mockSystem);
    return HttpResponse.json({ code: "NOT_FOUND", message: "Not found" }, { status: 404 });
  }),
  http.post("/api/v1/systems", () => HttpResponse.json(mockSystem, { status: 201 })),
  http.patch("/api/v1/systems/:id", () => HttpResponse.json(mockSystem)),

  // Entities
  http.get("/api/v1/entities", () =>
    HttpResponse.json<PaginatedResponse<Entity>>({
      items: [mockEntity],
      total: 1,
      limit: 20,
      offset: 0,
    }),
  ),
  http.get("/api/v1/entities/:id", ({ params }) => {
    if (params["id"] === mockEntity.id) return HttpResponse.json(mockEntity);
    return HttpResponse.json({ code: "NOT_FOUND", message: "Not found" }, { status: 404 });
  }),
  http.post("/api/v1/entities", () => HttpResponse.json(mockEntity, { status: 201 })),
  http.put("/api/v1/entities/:id", () => HttpResponse.json(mockEntity)),
  http.delete("/api/v1/entities/:id", () => new HttpResponse(null, { status: 204 })),
  http.post("/api/v1/entities/bulk", () =>
    HttpResponse.json({ total: 1, created: 1, updated: 0, errors: [] }),
  ),

  // Overlays
  http.get("/api/v1/overlays", () =>
    HttpResponse.json<PaginatedResponse<PropertyOverlay>>({
      items: [mockOverlay],
      total: 1,
      limit: 20,
      offset: 0,
    }),
  ),
  http.post("/api/v1/overlays", () => HttpResponse.json(mockOverlay, { status: 201 })),
  http.put("/api/v1/overlays/:id", () => HttpResponse.json(mockOverlay)),
  http.delete("/api/v1/overlays/:id", () => new HttpResponse(null, { status: 204 })),

  // Webhooks
  http.get("/api/v1/systems/:systemId/webhooks", () =>
    HttpResponse.json<PaginatedResponse<WebhookSubscription>>({
      items: [mockWebhook],
      total: 1,
      limit: 20,
      offset: 0,
    }),
  ),
  http.post("/api/v1/systems/:systemId/webhooks", () =>
    HttpResponse.json(mockWebhook, { status: 201 }),
  ),
  http.patch("/api/v1/systems/:systemId/webhooks/:id", () => HttpResponse.json(mockWebhook)),
  http.delete("/api/v1/systems/:systemId/webhooks/:id", () =>
    new HttpResponse(null, { status: 204 }),
  ),
  http.get("/api/v1/systems/:systemId/webhooks/:id/deliveries", () =>
    HttpResponse.json<PaginatedResponse<WebhookDelivery>>({
      items: [mockDelivery],
      total: 1,
      limit: 20,
      offset: 0,
    }),
  ),

  // Audit log
  http.get("/api/v1/audit", () =>
    HttpResponse.json<PaginatedResponse<AuditLogEntry>>({
      items: [mockAuditEntry],
      total: 1,
      limit: 20,
      offset: 0,
    }),
  ),

  // Stats / dashboard
  http.get("/api/v1/stats", () =>
    HttpResponse.json({ total_entities: 42, active_systems: 3, pending_webhooks: 1 }),
  ),
];
