# oad:system-admin Role Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Introduce an `oad:system-admin` group that grants scoped management permissions (edit system description, CRUD overlay schemas) to users assigned via `has_role_in` relations on specific systems, without giving them platform-wide admin rights.

**Architecture:** A new migration seeds the `oad:system-admin` built-in group. The `IdentityResolver` maps SCIM group `oad-system-admin` to `oad:system-admin` and sets a new `IsSystemAdmin` flag on `Identity`. Route authorization is restructured so platform admins retain full access while `system-admin` users are constrained to their `AllowedSystems` via a new `RequirePathSystemInScope` middleware. `system/repository.List` gains an optional UUID filter used by the service to return only the caller's allowed systems for non-platform-admins.

**Tech Stack:** Go 1.25, PostgreSQL 15 (pgx/v5), Chi v5 middleware, React 19 + TypeScript, oidc-client-ts

---

## File Map

| File | Action | Purpose |
|---|---|---|
| `migrations/000002_system_admin_role.up.sql` | Create | Seed `oad:system-admin` built-in group entity |
| `migrations/000002_system_admin_role.down.sql` | Create | Roll back the group seed |
| `internal/auth/identity.go` | Modify | Add `IsSystemAdmin bool` field; update `HasRole` hierarchy |
| `internal/auth/identity_test.go` | Modify | Add test cases for `system-admin` role |
| `internal/auth/resolver.go` | Modify | Add SCIM `oad-system-admin` → `oad:system-admin` mapping; set `IsSystemAdmin` in `Resolve` |
| `internal/api/middleware/authz.go` | Modify | Add `RequirePathSystemInScope(pathParam string)` middleware |
| `internal/api/middleware/authz_test.go` | Modify | Add tests for `RequirePathSystemInScope` |
| `internal/system/repository.go` | Modify | Add optional `[]uuid.UUID` filter to `List` |
| `internal/system/service.go` | Modify | Pass `AllowedSystems` filter to `List` for non-platform-admins |
| `internal/api/router.go` | Modify | Restructure `/systems` route authorization |
| `web/src/contexts/AuthContext.tsx` | Modify | Add `isSystemAdmin` derived field to `AuthIdentity` |
| `web/src/components/layout/Sidebar.tsx` | Modify | Role-aware nav: hide admin-only items from non-admins |

---

### Task 1: Migration — seed oad:system-admin group

**Files:**
- Create: `migrations/000002_system_admin_role.up.sql`
- Create: `migrations/000002_system_admin_role.down.sql`

- [ ] **Step 1: Write the up migration**

```sql
-- migrations/000002_system_admin_role.up.sql
-- Add the oad:system-admin built-in group.
-- Users in this group + has_role_in relation on specific systems get
-- scoped management rights (edit description, CRUD overlay schemas).

INSERT INTO entity (type_id, external_id, properties, system_id, is_builtin)
VALUES (
    (SELECT id FROM entity_type_definition WHERE type_name = 'Group'),
    'oad:system-admin',
    '{"displayName":"OAD System Admin","description":"Scoped management access: edit system description and manage overlay schemas for assigned systems."}'::jsonb,
    NULL,
    true
);
```

- [ ] **Step 2: Write the down migration**

```sql
-- migrations/000002_system_admin_role.down.sql
DELETE FROM entity WHERE external_id = 'oad:system-admin' AND is_builtin = true;
```

- [ ] **Step 3: Verify migrations compile (embed picks them up automatically)**

Run: `make build`
Expected: build succeeds with no errors

- [ ] **Step 4: Commit**

```bash
git add migrations/000002_system_admin_role.up.sql migrations/000002_system_admin_role.down.sql
git commit -m "feat(auth): seed oad:system-admin built-in group (migration 000002)"
```

---

### Task 2: Update identity.go — add IsSystemAdmin field and role hierarchy

**Files:**
- Modify: `internal/auth/identity.go`

The current `Identity` struct and `HasRole` in `internal/auth/identity.go:13-40`:

```go
type Identity struct {
    ...
    IsPlatformAdmin bool
    ...
}

func (id *Identity) HasRole(role string) bool {
    switch role {
    case "admin":
        return id.IsPlatformAdmin
    case "editor":
        return id.IsPlatformAdmin || id.hasGroup("oad:editor")
    case "viewer":
        return id.IsPlatformAdmin || id.hasGroup("oad:editor") || id.hasGroup("oad:viewer")
    }
    return false
}
```

New hierarchy:
- `admin`: platform-wide admin only (`IsPlatformAdmin`)
- `system-admin`: platform admin OR member of `oad:system-admin`
- `editor`: any of the above OR member of `oad:editor`
- `viewer`: any of the above OR member of `oad:viewer`

- [ ] **Step 1: Add `IsSystemAdmin` to the Identity struct and update HasRole**

Replace `internal/auth/identity.go` lines 13-40:

```go
type Identity struct {
	Subject         string
	Provider        string
	EntityID        uuid.UUID
	Groups          []string
	AllowedSystems  []uuid.UUID
	IsPlatformAdmin bool
	IsSystemAdmin   bool       // true iff "oad:system-admin" is in Groups (or IsPlatformAdmin).
	ActiveSystemID  *uuid.UUID
	AuthMode        string
}

// HasRole reports whether the identity satisfies the named role.
// Role hierarchy (each level includes all levels below):
//
//	"admin"        → IsPlatformAdmin
//	"system-admin" → IsPlatformAdmin or IsSystemAdmin
//	"editor"       → any of the above or "oad:editor" in Groups
//	"viewer"       → any of the above or "oad:viewer" in Groups
func (id *Identity) HasRole(role string) bool {
	switch role {
	case "admin":
		return id.IsPlatformAdmin
	case "system-admin":
		return id.IsPlatformAdmin || id.IsSystemAdmin
	case "editor":
		return id.IsPlatformAdmin || id.IsSystemAdmin || id.hasGroup("oad:editor")
	case "viewer":
		return id.IsPlatformAdmin || id.IsSystemAdmin || id.hasGroup("oad:editor") || id.hasGroup("oad:viewer")
	}
	return false
}
```

- [ ] **Step 2: Run tests to verify no regressions**

Run: `make test`
Expected: all tests pass

- [ ] **Step 3: Commit**

```bash
git add internal/auth/identity.go
git commit -m "feat(auth): add IsSystemAdmin field and update HasRole hierarchy"
```

---

### Task 3: Update identity_test.go — add system-admin test cases

**Files:**
- Modify: `internal/auth/identity_test.go`

- [ ] **Step 1: Write the new test cases for system-admin**

In `TestIdentity_HasRole`, after the existing `{"oad:viewer lacks admin", ...}` case, add:

```go
// system-admin role
{"platform admin has system-admin", true, nil, "system-admin", true},
{"oad:system-admin has system-admin", false, []string{"oad:system-admin"}, "system-admin", true},
{"oad:system-admin has editor (elevated)", false, []string{"oad:system-admin"}, "editor", true},
{"oad:system-admin has viewer (elevated)", false, []string{"oad:system-admin"}, "viewer", true},
{"oad:system-admin lacks admin", false, []string{"oad:system-admin"}, "admin", false},
{"oad:editor lacks system-admin", false, []string{"oad:editor"}, "system-admin", false},
{"oad:viewer lacks system-admin", false, []string{"oad:viewer"}, "system-admin", false},
```

But `HasRole` checks `id.IsSystemAdmin`, not `id.hasGroup("oad:system-admin")` directly — the resolver sets `IsSystemAdmin` from groups. For unit tests of `HasRole`, set both `Groups` and `IsSystemAdmin`:

```go
{"oad:system-admin has system-admin", false, []string{"oad:system-admin"}, "system-admin", true},
```

This test won't pass because `HasRole("system-admin")` checks `id.IsSystemAdmin` not `id.hasGroup(...)`. Add a `isSystemAdmin` field to the test table and set it in the `Identity`:

```go
tests := []struct {
    name            string
    isPlatformAdmin bool
    isSystemAdmin   bool
    groups          []string
    role            string
    want            bool
}{
    {"platform admin has admin", true, false, nil, "admin", true},
    {"platform admin has editor", true, false, nil, "editor", true},
    {"platform admin has viewer", true, false, nil, "viewer", true},
    {"no groups has no admin", false, false, nil, "admin", false},
    {"oad:editor has editor", false, false, []string{"oad:editor"}, "editor", true},
    {"oad:editor has viewer (elevated)", false, false, []string{"oad:editor"}, "viewer", true},
    {"oad:viewer has viewer", false, false, []string{"oad:viewer"}, "viewer", true},
    {"oad:viewer lacks editor", false, false, []string{"oad:viewer"}, "editor", false},
    {"oad:viewer lacks admin", false, false, []string{"oad:viewer"}, "admin", false},
    // system-admin role
    {"platform admin has system-admin", true, false, nil, "system-admin", true},
    {"system-admin flag has system-admin", false, true, nil, "system-admin", true},
    {"system-admin flag has editor (elevated)", false, true, nil, "editor", true},
    {"system-admin flag has viewer (elevated)", false, true, nil, "viewer", true},
    {"system-admin flag lacks admin", false, true, nil, "admin", false},
    {"oad:editor lacks system-admin", false, false, []string{"oad:editor"}, "system-admin", false},
    {"oad:viewer lacks system-admin", false, false, []string{"oad:viewer"}, "system-admin", false},
}

for _, tc := range tests {
    t.Run(tc.name, func(t *testing.T) {
        id := &auth.Identity{
            IsPlatformAdmin: tc.isPlatformAdmin,
            IsSystemAdmin:   tc.isSystemAdmin,
            Groups:          tc.groups,
        }
        if got := id.HasRole(tc.role); got != tc.want {
            t.Errorf("HasRole(%q) = %v, want %v", tc.role, got, tc.want)
        }
    })
}
```

- [ ] **Step 2: Run the failing test first to confirm it fails**

Run: `go test ./internal/auth/... -run TestIdentity_HasRole -v`
Expected: FAIL — test cases for `system-admin` fail because `HasRole("system-admin")` doesn't exist yet (already fixed in Task 2, so should now pass)

- [ ] **Step 3: Run tests**

Run: `make test`
Expected: all pass

- [ ] **Step 4: Commit**

```bash
git add internal/auth/identity_test.go
git commit -m "test(auth): add system-admin HasRole test cases"
```

---

### Task 4: Update resolver.go — set IsSystemAdmin + add SCIM oad-system-admin mapping

**Files:**
- Modify: `internal/auth/resolver.go`

Two changes:
1. In `Resolve()`, set `id.IsSystemAdmin` when `oad:system-admin` is in groups.
2. In `Groups()` SQL query, add a `CASE WHEN` arm for `oad-system-admin` → `oad:system-admin`.

- [ ] **Step 1: Update the Resolve method to set IsSystemAdmin**

In `internal/auth/resolver.go`, the `Resolve` function currently sets `IsPlatformAdmin` via a loop (lines 80-85):

```go
for _, g := range groups {
    if g == "oad:admin" {
        id.IsPlatformAdmin = true
        break
    }
}
```

Replace this loop with:

```go
for _, g := range groups {
    switch g {
    case "oad:admin":
        id.IsPlatformAdmin = true
    case "oad:system-admin":
        id.IsSystemAdmin = true
    }
}
```

- [ ] **Step 2: Update the Groups SQL CASE WHEN to include oad-system-admin**

In `internal/auth/resolver.go`, the `Groups()` method SQL query currently maps:
- `oad-admin` → `oad:admin`
- `oad-editor` → `oad:editor`
- `oad-viewer` → `oad:viewer`

Add the `oad-system-admin` arm:

```go
rows, err := r.pool.Query(ctx,
    `SELECT
       CASE
         WHEN e.external_id LIKE 'scim:%' AND e.properties->>'displayName' = 'oad-admin'        THEN 'oad:admin'
         WHEN e.external_id LIKE 'scim:%' AND e.properties->>'displayName' = 'oad-system-admin' THEN 'oad:system-admin'
         WHEN e.external_id LIKE 'scim:%' AND e.properties->>'displayName' = 'oad-editor'       THEN 'oad:editor'
         WHEN e.external_id LIKE 'scim:%' AND e.properties->>'displayName' = 'oad-viewer'       THEN 'oad:viewer'
         ELSE e.external_id
       END
     FROM relation rel
     JOIN entity e ON e.id = rel.target_entity_id
     JOIN entity_type_definition etd ON etd.id = e.type_id
     WHERE rel.subject_entity_id = $1
       AND rel.relation_type = 'member_of'
       AND etd.type_name = 'Group'`,
    entityID,
)
```

- [ ] **Step 3: Run tests**

Run: `make test`
Expected: all pass

- [ ] **Step 4: Commit**

```bash
git add internal/auth/resolver.go
git commit -m "feat(auth): set IsSystemAdmin from oad:system-admin group; map SCIM oad-system-admin"
```

---

### Task 5: Add RequirePathSystemInScope middleware

**Files:**
- Modify: `internal/api/middleware/authz.go`
- Modify: `internal/api/middleware/authz_test.go`

`RequirePathSystemInScope` extracts `{pathParam}` from the URL, parses it as UUID, and verifies either:
- The caller is a platform admin, OR
- The UUID is in `identity.AllowedSystems`

This is more targeted than `RequireSystemScope` (which only checks `ActiveSystemID != nil`). It guards `/systems/{system_id}/*` sub-trees from non-platform-admins accessing systems they are not assigned to via `has_role_in`.

- [ ] **Step 1: Write the failing test first**

Append to `internal/api/middleware/authz_test.go`:

```go
func TestRequirePathSystemInScope_PlatformAdminAllowed(t *testing.T) {
	sysID := uuid.MustParse("aaaaaaaa-0000-0000-0000-000000000001")
	handler := middleware.RequirePathSystemInScope("system_id")(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }),
	)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/systems/"+sysID.String(), http.NoBody)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("system_id", sysID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = withIdentity(req, &auth.Identity{IsPlatformAdmin: true})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for platform admin, got %d", rec.Code)
	}
}

func TestRequirePathSystemInScope_AllowedSystem(t *testing.T) {
	sysID := uuid.MustParse("bbbbbbbb-0000-0000-0000-000000000001")
	handler := middleware.RequirePathSystemInScope("system_id")(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }),
	)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/systems/"+sysID.String(), http.NoBody)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("system_id", sysID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = withIdentity(req, &auth.Identity{IsSystemAdmin: true, AllowedSystems: []uuid.UUID{sysID}})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for allowed system, got %d", rec.Code)
	}
}

func TestRequirePathSystemInScope_ForbiddenSystem(t *testing.T) {
	sysID := uuid.MustParse("cccccccc-0000-0000-0000-000000000001")
	otherID := uuid.MustParse("dddddddd-0000-0000-0000-000000000001")
	handler := middleware.RequirePathSystemInScope("system_id")(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }),
	)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/systems/"+sysID.String(), http.NoBody)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("system_id", sysID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = withIdentity(req, &auth.Identity{IsSystemAdmin: true, AllowedSystems: []uuid.UUID{otherID}})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for forbidden system, got %d", rec.Code)
	}
}

func TestRequirePathSystemInScope_InvalidUUID(t *testing.T) {
	handler := middleware.RequirePathSystemInScope("system_id")(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }),
	)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/systems/not-a-uuid", http.NoBody)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("system_id", "not-a-uuid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = withIdentity(req, &auth.Identity{IsPlatformAdmin: true})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid UUID, got %d", rec.Code)
	}
}
```

You also need to add the chi import to `authz_test.go`:
```go
import (
    ...
    "github.com/go-chi/chi/v5"
    ...
)
```

- [ ] **Step 2: Run the test to confirm it fails (function not yet defined)**

Run: `go test ./internal/api/middleware/... -run TestRequirePathSystemInScope -v`
Expected: FAIL — `RequirePathSystemInScope` not defined

- [ ] **Step 3: Implement RequirePathSystemInScope in authz.go**

Append to `internal/api/middleware/authz.go`:

```go
// RequirePathSystemInScope returns middleware that verifies the UUID in the
// given URL path parameter is within the caller's AllowedSystems.
// Platform admins bypass the check. Returns 403 if the system is not allowed,
// 400 if the path parameter is not a valid UUID.
// Must be chained after Authentication middleware.
func RequirePathSystemInScope(pathParam string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity, ok := auth.IdentityFromContext(r.Context())
			if !ok {
				response.Error(w, apierr.Unauthorized("missing identity"))
				return
			}
			if identity.IsPlatformAdmin {
				next.ServeHTTP(w, r)
				return
			}
			rawID := chi.URLParam(r, pathParam)
			systemID, err := uuid.Parse(rawID)
			if err != nil {
				response.Error(w, apierr.BadRequest(pathParam+" must be a valid UUID"))
				return
			}
			for _, allowed := range identity.AllowedSystems {
				if allowed == systemID {
					next.ServeHTTP(w, r)
					return
				}
			}
			response.Error(w, apierr.Forbidden("access denied to system "+rawID))
		})
	}
}
```

Add the chi import to `authz.go`:
```go
import (
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/google/uuid"

    "github.com/danielpadua/oad/internal/api/response"
    "github.com/danielpadua/oad/internal/apierr"
    "github.com/danielpadua/oad/internal/auth"
)
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/api/middleware/... -run TestRequirePathSystemInScope -v`
Expected: all 4 new tests PASS

Run: `make test`
Expected: all tests pass

- [ ] **Step 5: Commit**

```bash
git add internal/api/middleware/authz.go internal/api/middleware/authz_test.go
git commit -m "feat(authz): add RequirePathSystemInScope middleware"
```

---

### Task 6: Update system/repository.go — add optional ID filter to List

**Files:**
- Modify: `internal/system/repository.go`

The `Repository` interface and `pgxRepository.List` currently take no filter. We need to accept an optional `[]uuid.UUID` that, when non-nil, restricts results to those IDs.

- [ ] **Step 1: Update the Repository interface and implementation**

In `internal/system/repository.go`, change:

```go
// Repository interface — change List signature:
List(ctx context.Context, q db.DBTX) ([]*System, error)
```
to:
```go
List(ctx context.Context, q db.DBTX, allowedIDs []uuid.UUID) ([]*System, error)
```

Update `pgxRepository.List`:

```go
func (r *pgxRepository) List(ctx context.Context, q db.DBTX, allowedIDs []uuid.UUID) ([]*System, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if len(allowedIDs) > 0 {
		rows, err = q.Query(ctx,
			`SELECT e.id,
			        COALESCE(e.properties->>'name', '')        AS name,
			        COALESCE(e.properties->>'description', '') AS description,
			        COALESCE((e.properties->>'active')::bool, true) AS active,
			        e.created_at,
			        e.updated_at
			 FROM entity e
			 JOIN entity_type_definition t ON t.id = e.type_id
			 WHERE t.type_name = $1
			   AND e.id = ANY($2)
			 ORDER BY e.properties->>'name'`,
			systemTypeName, allowedIDs,
		)
	} else {
		rows, err = q.Query(ctx,
			`SELECT e.id,
			        COALESCE(e.properties->>'name', '')        AS name,
			        COALESCE(e.properties->>'description', '') AS description,
			        COALESCE((e.properties->>'active')::bool, true) AS active,
			        e.created_at,
			        e.updated_at
			 FROM entity e
			 JOIN entity_type_definition t ON t.id = e.type_id
			 WHERE t.type_name = $1
			 ORDER BY e.properties->>'name'`,
			systemTypeName,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("querying systems: %w", err)
	}
	defer rows.Close()

	var result []*System
	for rows.Next() {
		s := &System{}
		if err := rows.Scan(&s.ID, &s.Name, &s.Description, &s.Active,
			&s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning system: %w", err)
		}
		result = append(result, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating systems: %w", err)
	}
	return result, nil
}
```

You also need to add `"github.com/jackc/pgx/v5"` to the imports (for `pgx.Rows`) — it may already be present as an indirect import. Check and add if missing.

- [ ] **Step 2: Run build to catch type errors**

Run: `make build`
Expected: compile error on `service.go` (calling old `List` signature) — fix in Task 7

- [ ] **Step 3: Commit**

```bash
git add internal/system/repository.go
git commit -m "feat(system): add optional ID filter to repository.List"
```

---

### Task 7: Update system/service.go — filter List by AllowedSystems

**Files:**
- Modify: `internal/system/service.go`

The `List` method must pass `nil` (platform admin — no filter) or `identity.AllowedSystems` (scoped user — filter to their systems).

- [ ] **Step 1: Update service.List to pass the filter**

Replace `internal/system/service.go` lines 70-79:

```go
// List returns systems visible to the caller.
// Platform admins see all systems. Non-platform-admins see only systems
// they have a has_role_in relation to (their AllowedSystems).
func (s *Service) List(ctx context.Context) ([]*System, error) {
	var allowedIDs []uuid.UUID
	if identity, ok := auth.IdentityFromContext(ctx); ok && !identity.IsPlatformAdmin {
		allowedIDs = identity.AllowedSystems
		if allowedIDs == nil {
			allowedIDs = []uuid.UUID{} // empty slice → query with no-match filter
		}
	}
	items, err := s.repo.List(ctx, s.pool, allowedIDs)
	if err != nil {
		return nil, fmt.Errorf("listing systems: %w", err)
	}
	if items == nil {
		items = []*System{}
	}
	return items, nil
}
```

Note: passing an empty `[]uuid.UUID{}` (not nil) triggers the `AND e.id = ANY($2)` branch, returning 0 rows for users with no system assignments. Passing `nil` returns all systems (platform admin path).

- [ ] **Step 2: Run build**

Run: `make build`
Expected: success

- [ ] **Step 3: Run tests**

Run: `make test`
Expected: all pass

- [ ] **Step 4: Commit**

```bash
git add internal/system/service.go
git commit -m "feat(system): filter List by AllowedSystems for non-platform-admins"
```

---

### Task 8: Restructure /systems routes in router.go

**Files:**
- Modify: `internal/api/router.go`

Current `/systems` block has a blanket `r.Use(middleware.RequireRole("admin"))` that blocks all non-admins. New structure:

| Method | Path | Authorization |
|---|---|---|
| `GET /systems/` | list | `RequireAnyRole("admin","system-admin","editor","viewer")` — service filters |
| `POST /systems/` | create | `RequirePlatformAdmin` |
| `GET /systems/{system_id}` | detail | `RequireAnyRole("admin","system-admin","editor","viewer")` + `RequirePathSystemInScope("system_id")` for non-admins |
| `PATCH /systems/{system_id}` | update | `RequireAnyRole("admin","system-admin")` + `RequirePathSystemInScope("system_id")` |
| `GET /systems/{system_id}/overlay-schemas/*` | read | `RequireAnyRole("admin","system-admin")` + `RequirePathSystemInScope("system_id")` |
| `POST/PUT/DELETE /systems/{system_id}/overlay-schemas/*` | write | `RequireAnyRole("admin","system-admin")` + `RequirePathSystemInScope("system_id")` |

- [ ] **Step 1: Replace the /systems route block**

In `internal/api/router.go`, replace lines 141-177 (the entire `/systems` route block):

```go
		// Systems and their overlay schemas.
		// Platform admins have full access.
		// system-admin users can list/view/patch (description only) and manage overlay schemas
		// for systems assigned via has_role_in.
		// editor and viewer users can list and view their assigned systems (read-only).
		r.Route("/systems", func(r chi.Router) {
			// List: all roles that have system assignments can see their systems.
			// Service filters by AllowedSystems for non-platform-admins.
			r.With(middleware.RequireAnyRole("admin", "system-admin", "editor", "viewer")).
				Get("/", deps.SystemHandler.List)

			// Create: registering a new system is a platform-level action.
			r.With(middleware.RequirePlatformAdmin).Post("/", deps.SystemHandler.Create)

			r.Route("/{system_id}", func(r chi.Router) {
				// Detail and patch: require at minimum system-admin or editor role,
				// and the system must be in the caller's AllowedSystems (or platform admin).
				r.With(
					middleware.RequireAnyRole("admin", "system-admin", "editor", "viewer"),
					middleware.RequirePathSystemInScope("system_id"),
				).Get("/", deps.SystemHandler.GetByID)

				r.With(
					middleware.RequireAnyRole("admin", "system-admin"),
					middleware.RequirePathSystemInScope("system_id"),
				).Patch("/", deps.SystemHandler.Patch)

				// Overlay schemas: system-admin and above can manage them.
				r.Route("/overlay-schemas", func(r chi.Router) {
					r.Use(middleware.RequireAnyRole("admin", "system-admin"))
					r.Use(middleware.RequirePathSystemInScope("system_id"))
					r.Get("/", deps.OverlaySchemaHandler.List)
					r.Post("/", deps.OverlaySchemaHandler.Create)
					r.Get("/{schema_id}", deps.OverlaySchemaHandler.GetByID)
					r.Put("/{schema_id}", deps.OverlaySchemaHandler.Update)
					r.Delete("/{schema_id}", deps.OverlaySchemaHandler.Delete)
				})

				// Webhook subscriptions: platform admin scoped via RequireSystemScope.
				r.Route("/webhooks", func(r chi.Router) {
					r.Use(middleware.RequireRole("admin"))
					r.Use(middleware.RequireSystemScope("system_id"))
					r.Get("/", deps.WebhookHandler.List)
					r.Post("/", deps.WebhookHandler.Create)

					r.Route("/{webhook_id}", func(r chi.Router) {
						r.Get("/", deps.WebhookHandler.GetByID)
						r.Patch("/", deps.WebhookHandler.Update)
						r.Delete("/", deps.WebhookHandler.Delete)
					})
				})
			})
		})
```

- [ ] **Step 2: Run build**

Run: `make build`
Expected: success

- [ ] **Step 3: Run tests**

Run: `make test`
Expected: all pass

- [ ] **Step 4: Commit**

```bash
git add internal/api/router.go
git commit -m "feat(router): restructure /systems routes for system-admin and editor access"
```

---

### Task 9: Frontend — add isSystemAdmin to AuthIdentity

**Files:**
- Modify: `web/src/contexts/AuthContext.tsx`

- [ ] **Step 1: Add isSystemAdmin to the AuthIdentity type and derive it from groups**

In `web/src/contexts/AuthContext.tsx`, update the `AuthIdentity` interface and `fetchMeIdentity`:

```typescript
export interface AuthIdentity {
  sub: string;
  email?: string;
  name?: string;
  isPlatformAdmin: boolean;
  isSystemAdmin: boolean;
  /** Built-in group external IDs (e.g. "oad:admin", "oad:system-admin", "oad:editor", "oad:viewer"). */
  groups: string[];
  allowedSystems: string[];
  /** Currently active system UUID; null means no system scope selected. */
  activeSystemId: string | null;
}
```

In `fetchMeIdentity`, derive `isSystemAdmin`:

```typescript
async function fetchMeIdentity(user: User): Promise<AuthIdentity | null> {
  try {
    const me = await http.get<MeResponse>("/api/v1/me", { token: user.access_token });
    const activeSystemId = me.allowed_systems[0] ?? null;
    return {
      sub: me.sub,
      isPlatformAdmin: me.is_platform_admin,
      isSystemAdmin: !me.is_platform_admin && me.groups.includes("oad:system-admin"),
      groups: me.groups,
      allowedSystems: me.allowed_systems,
      activeSystemId,
    };
  } catch {
    return null;
  }
}
```

- [ ] **Step 2: Build frontend to check for TypeScript errors**

Run: `make web-build`
Expected: success

- [ ] **Step 3: Commit**

```bash
git add web/src/contexts/AuthContext.tsx
git commit -m "feat(ui): add isSystemAdmin derived field to AuthIdentity"
```

---

### Task 10: Frontend — role-aware Sidebar navigation

**Files:**
- Modify: `web/src/components/layout/Sidebar.tsx`

Platform admins see all nav items. System-admins and editors see: Systems, Entities, Overlays, Webhooks (within their system scope), Audit Log. Viewers see the same but without write actions (enforced server-side; nav items are still shown for convenience).

- [ ] **Step 1: Update Sidebar to filter admin-only items**

```tsx
import { useAuth } from "@/contexts/AuthContext";

export function Sidebar({ onNavigate }: SidebarProps) {
  const { t } = useTranslation();
  const { identity } = useAuth();

  const isPlatformAdmin = identity?.isPlatformAdmin ?? false;

  const allNavItems = [
    { to: "/", icon: LayoutDashboard, label: t("nav.dashboard"), adminOnly: false },
    { to: "/entity-types", icon: FolderTree, label: t("nav.entityTypes"), adminOnly: true },
    { to: "/systems", icon: Network, label: t("nav.systems"), adminOnly: false },
    { to: "/entities", icon: Users, label: t("nav.entities"), adminOnly: false },
    { to: "/users", icon: UserCog, label: t("nav.usersAdmin"), adminOnly: true },
    { to: "/overlays", icon: Layers, label: t("nav.overlays"), adminOnly: false },
    { to: "/webhooks", icon: Webhook, label: t("nav.webhooks"), adminOnly: false },
    { to: "/audit", icon: ScrollText, label: t("nav.auditLog"), adminOnly: false },
    { to: "/settings", icon: Settings, label: t("nav.settings"), adminOnly: true },
  ];

  const navItems = allNavItems.filter((item) => !item.adminOnly || isPlatformAdmin);

  return (
    <aside className="flex w-16 flex-col items-center border-r border-border bg-card py-4 sm:w-16">
      <Dock orientation="vertical" magnification={52} distance={100} className="gap-1">
        {navItems.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.to === "/"}
            title={item.label}
            onClick={onNavigate}
            className={({ isActive }) =>
              cn(
                "flex h-full w-full items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground",
                isActive &&
                  "bg-primary text-primary-foreground hover:bg-primary/90 hover:text-primary-foreground",
              )
            }
          >
            <item.icon className="h-5 w-5 shrink-0" />
            <span className="sr-only">{item.label}</span>
          </NavLink>
        ))}
      </Dock>
    </aside>
  );
}
```

- [ ] **Step 2: Build and verify**

Run: `make web-build`
Expected: success

- [ ] **Step 3: Commit**

```bash
git add web/src/components/layout/Sidebar.tsx
git commit -m "feat(ui): hide admin-only nav items from non-platform-admin users"
```

---

### Task 11: Full verification

- [ ] **Step 1: Run full test suite with race detector**

Run: `make test`
Expected: all tests pass, no race conditions

- [ ] **Step 2: Build final binary**

Run: `make build`
Expected: binary at `./bin/oad` compiled successfully

- [ ] **Step 3: Final commit (if any stragglers)**

```bash
git status
# if clean, nothing to do
```

---

## Self-Review

### Spec coverage

| Requirement | Task |
|---|---|
| Seed `oad:system-admin` built-in group | Task 1 |
| `HasRole("system-admin")` check | Task 2 |
| `IsSystemAdmin` flag in Identity | Task 2, Task 4 |
| SCIM `oad-system-admin` → `oad:system-admin` group mapping | Task 4 |
| `RequirePathSystemInScope` middleware | Task 5 |
| System list filtered by AllowedSystems for non-platform-admins | Task 6, Task 7 |
| `/systems/{id}` and overlay-schemas accessible to system-admin | Task 8 |
| Webhooks remain platform-admin + RequireSystemScope only | Task 8 |
| `isSystemAdmin` in frontend AuthIdentity | Task 9 |
| Sidebar hides Entity Types, Users Admin, Settings from non-admins | Task 10 |

### Placeholder scan

No TBD, TODO, or incomplete steps — all code blocks are complete.

### Type consistency

- `AllowedSystems []uuid.UUID` in `Identity` is consistent with `allowedIDs []uuid.UUID` in `repository.List`.
- `RequirePathSystemInScope` uses `chi.URLParam` (chi v5 API) — same package already used in existing handler code.
- `IsSystemAdmin bool` added to `Identity` struct in Task 2 and read in `Resolve` in Task 4 and `RequirePathSystemInScope` in Task 5.
- `HasRole("system-admin")` added in Task 2; tested in Task 3; used as role name string in Task 8 `RequireAnyRole("admin", "system-admin")`.
