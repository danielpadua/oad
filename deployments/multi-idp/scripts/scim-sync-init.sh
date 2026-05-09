#!/bin/sh
# scim-sync-init.sh — deterministic SCIM bootstrap for the multi-idp stack.
#
# This script runs as a one-shot init container. It waits for all
# prerequisites (Authentik users seeded, SCIM provider registered, OAD
# API healthy) and then forces a full SCIM sync via Authentik's provider
# sync API. Finally it polls the OAD database to verify the expected
# entities and relations were provisioned.
#
# Why: Authentik's internal SCIM sync fires as soon as its Celery worker
# starts, which races with blueprint application. The first sync
# typically runs before all Users exist in Authentik, resulting in
# partial provisioning. A second incremental sync rarely catches up
# because Authentik considers objects as "already synced". This script
# eliminates the race by:
#   1. Waiting for all blueprint users to exist in Authentik before
#      triggering any sync.
#   2. Waiting for the OIDC discovery endpoint (unauthenticated) instead
#      of a token-based poll. The token is blueprint-created and only
#      exists after DB migrations + blueprint application (~5-10 min on
#      a cold start). The OIDC endpoint becomes available at the same time.
#   3. Re-running the system task scim_sync:<provider-slug> via
#      POST /events/system_tasks/{uuid}/run/ — which always does a full
#      sync. (Authentik 2024.10.x removed POST /providers/scim/{pk}/sync/.)
#   3. Polling the OAD database with a re-trigger loop until all
#      expected entities and relations are confirmed present.
#
# Required env vars:
#   AUTHENTIK_URL              e.g. http://authentik-server:9000
#   AUTHENTIK_API_TOKEN        API token for akadmin (created by blueprint)
#   OAD_API_URL                e.g. http://api:8080
#   DATABASE_URL               e.g. postgresql://oad:oad@postgres:5432/oad
#
# Expected blueprint topology (configurable via EXPECTED_*):
#   4 Users:  admin@oad.dev, editor@oad.dev, viewer@oad.dev, pdp@oad.dev
#   3 Groups: oad-admin, oad-editor, oad-viewer
#   4 Relations (member_of)
set -eu

# Install curl — not included in postgres:15-alpine by default
apk add --no-cache curl > /dev/null 2>&1

# ── Configuration ────────────────────────────────────────────────────
TIMEOUT="${TIMEOUT:-300}"
POLL_INTERVAL="${POLL_INTERVAL:-5}"
EXPECTED_USERS="${EXPECTED_USERS:-4}"
EXPECTED_GROUPS="${EXPECTED_GROUPS:-3}"
EXPECTED_RELATIONS="${EXPECTED_RELATIONS:-4}"
AUTHENTIK_API="${AUTHENTIK_URL}/api/v3"

# ── Helpers ──────────────────────────────────────────────────────────
log()  { printf '[scim-init] %s\n' "$*"; }
die()  { log "FATAL: $*"; exit 1; }
elapsed() { echo $(( $(date +%s) - START_TIME )); }

check_timeout() {
  if [ "$(elapsed)" -ge "$TIMEOUT" ]; then
    die "Timed out after ${TIMEOUT}s — $1"
  fi
}

authentik_get() {
  curl -sf -H "Authorization: Bearer ${AUTHENTIK_API_TOKEN}" "$@"
}

authentik_post() {
  curl -sf -X POST -H "Authorization: Bearer ${AUTHENTIK_API_TOKEN}" "$@"
}

trigger_full_sync() {
  # Authentik 2024.10.x: re-run the system task instead of the old sync endpoint.
  authentik_post "${AUTHENTIK_API}/events/system_tasks/${SYNC_TASK_UUID}/run/" > /dev/null 2>&1 || true
}

# ── Pre-flight checks ───────────────────────────────────────────────
[ -z "${AUTHENTIK_URL:-}" ]       && die "AUTHENTIK_URL is not set"
[ -z "${AUTHENTIK_API_TOKEN:-}" ] && die "AUTHENTIK_API_TOKEN is not set"
[ -z "${OAD_API_URL:-}" ]        && die "OAD_API_URL is not set"
[ -z "${DATABASE_URL:-}" ]       && die "DATABASE_URL is not set"

START_TIME=$(date +%s)

log "Starting SCIM bootstrap init (timeout=${TIMEOUT}s)"
log "Authentik API: ${AUTHENTIK_API}"
log "OAD API:       ${OAD_API_URL}"

# ── Step 1: Wait for Authentik blueprint to be applied ───────────────
# Poll the OIDC discovery endpoint (unauthenticated) which only returns
# 200 after the blueprint creates the 'oad' OAuth2Provider. This is a
# reliable, no-auth signal that blueprints are applied and the API token
# also exists. Token-based polling fails on fresh starts because the
# token is blueprint-created, and blueprints run after ~5-10 min of
# DB migrations on a cold start.
log "Step 1/6: Waiting for Authentik blueprint (OIDC discovery must return 200)..."
while true; do
  check_timeout "Blueprint never applied — OIDC discovery endpoint never returned 200"
  if curl -sf "${AUTHENTIK_URL}/application/o/oad/.well-known/openid-configuration" > /dev/null 2>&1; then
    log "  ✔ Blueprint applied — OIDC provider 'oad' is ready"
    break
  fi
  sleep "$POLL_INTERVAL"
done

# ── Step 2: Wait for blueprint users ────────────────────────────────
log "Step 2/6: Waiting for ${EXPECTED_USERS} blueprint users in Authentik..."
while true; do
  check_timeout "Expected users never appeared in Authentik"
  # Count users whose username ends with @oad.dev (our blueprint users)
  USERS_JSON=$(authentik_get "${AUTHENTIK_API}/core/users/?page_size=100" 2>/dev/null || echo '{}')
  USER_COUNT=$(echo "$USERS_JSON" | grep -o '"username":"[^"]*@oad.dev"' | wc -l)
  if [ "$USER_COUNT" -ge "$EXPECTED_USERS" ]; then
    log "  ✔ Found ${USER_COUNT}/${EXPECTED_USERS} blueprint users"
    break
  fi
  log "  … found ${USER_COUNT}/${EXPECTED_USERS} users, retrying..."
  sleep "$POLL_INTERVAL"
done

# ── Step 3: Get SCIM provider PK ────────────────────────────────────
log "Step 3/6: Locating SCIM provider 'oad-scim-provider'..."
SCIM_PK=""
while true; do
  check_timeout "SCIM provider never appeared in Authentik"
  PROVIDER_JSON=$(authentik_get "${AUTHENTIK_API}/providers/scim/?search=oad-scim-provider" 2>/dev/null || echo '{}')
  SCIM_PK=$(echo "$PROVIDER_JSON" | grep -o '"pk":[0-9]*' | head -1 | cut -d: -f2)
  if [ -n "$SCIM_PK" ]; then
    log "  ✔ Found SCIM provider pk=${SCIM_PK}"
    break
  fi
  log "  … SCIM provider not found yet, retrying..."
  sleep "$POLL_INTERVAL"
done

# ── Step 4: Wait for OAD API ────────────────────────────────────────
log "Step 4/6: Waiting for OAD API health..."
while true; do
  check_timeout "OAD API never became healthy"
  if curl -sf "${OAD_API_URL}/health" > /dev/null 2>&1; then
    log "  ✔ OAD API is healthy"
    break
  fi
  sleep "$POLL_INTERVAL"
done

# ── Step 5: Get sync task UUID, then trigger full sync ──────────────
# Authentik 2024.10.x removed POST /providers/scim/{pk}/sync/.
# The replacement is POST /events/system_tasks/{uuid}/run/ which always
# re-runs a full sync. The task UUID is stable per provider slug and is
# returned by GET /providers/scim/{pk}/sync/status/.
log "Step 5/6: Locating SCIM sync task UUID for provider pk=${SCIM_PK}..."
SYNC_TASK_UUID=""
while true; do
  check_timeout "SCIM sync task UUID never appeared in system tasks"
  STATUS_JSON=$(authentik_get "${AUTHENTIK_API}/providers/scim/${SCIM_PK}/sync/status/" 2>/dev/null || echo '{}')
  SYNC_TASK_UUID=$(echo "$STATUS_JSON" | grep -o '"uuid":"[^"]*"' | head -1 | sed 's/"uuid":"//;s/"//g')
  if [ -n "$SYNC_TASK_UUID" ]; then
    log "  ✔ Found sync task UUID: ${SYNC_TASK_UUID}"
    break
  fi
  log "  … sync task not available yet, retrying..."
  sleep "$POLL_INTERVAL"
done
authentik_post "${AUTHENTIK_API}/events/system_tasks/${SYNC_TASK_UUID}/run/" > /dev/null \
  || die "Failed to trigger SCIM sync task ${SYNC_TASK_UUID}"
log "  ✔ Full sync triggered"

# ── Step 6: Verify OAD database state ───────────────────────────────
# Poll until all expected entities and relations appear. If any count
# falls short, re-trigger the sync (with a 30s cooldown) and keep
# polling. This covers all partial-failure scenarios: missing users,
# missing groups, or missing member_of relations.
log "Step 6/6: Verifying OAD database state..."

VERIFY_RETRIES=24  # 24 × 5s = 120s max verification wait
VERIFY_OK=false
LAST_SYNC_AT=0

for i in $(seq 1 $VERIFY_RETRIES); do
  check_timeout "Database verification timed out"

  # Count User entities
  ACTUAL_USERS=$(psql "$DATABASE_URL" -tAc \
    "SELECT count(*) FROM entity e
     JOIN entity_type_definition t ON t.id = e.type_id
     WHERE t.type_name = 'User'
       AND e.external_id LIKE 'scim:authentik:%'
       AND e.properties->>'userName' LIKE '%@oad.dev'" 2>/dev/null || echo "0")

  # Count custom Group entities (our oad-* groups)
  ACTUAL_GROUPS=$(psql "$DATABASE_URL" -tAc \
    "SELECT count(*) FROM entity e
     JOIN entity_type_definition t ON t.id = e.type_id
     WHERE t.type_name = 'Group'
       AND e.external_id LIKE 'scim:authentik:%'
       AND e.properties->>'displayName' LIKE 'oad-%'" 2>/dev/null || echo "0")

  # Count member_of relations
  ACTUAL_RELATIONS=$(psql "$DATABASE_URL" -tAc \
    "SELECT count(*) FROM relation
     WHERE relation_type = 'member_of'
       AND system_id IS NULL" 2>/dev/null || echo "0")

  log "  Check ${i}/${VERIFY_RETRIES}: users=${ACTUAL_USERS}/${EXPECTED_USERS} groups=${ACTUAL_GROUPS}/${EXPECTED_GROUPS} relations=${ACTUAL_RELATIONS}/${EXPECTED_RELATIONS}"

  if [ "$ACTUAL_USERS" -ge "$EXPECTED_USERS" ] && \
     [ "$ACTUAL_GROUPS" -ge "$EXPECTED_GROUPS" ] && \
     [ "$ACTUAL_RELATIONS" -ge "$EXPECTED_RELATIONS" ]; then
    VERIFY_OK=true
    break
  fi

  # Re-trigger a fresh full sync if 30s have elapsed since the last one.
  # This handles all partial-failure cases regardless of which count is low.
  NOW=$(date +%s)
  if [ $(( NOW - LAST_SYNC_AT )) -ge 30 ]; then
    log "  ⚠ Counts not yet complete — re-triggering full sync..."
    trigger_full_sync
    LAST_SYNC_AT=$NOW
  fi

  sleep "$POLL_INTERVAL"
done

if [ "$VERIFY_OK" = "true" ]; then
  log ""
  log "═══════════════════════════════════════════════════════"
  log "  ✔ SCIM bootstrap complete!"
  log "    Users:     ${ACTUAL_USERS}/${EXPECTED_USERS}"
  log "    Groups:    ${ACTUAL_GROUPS}/${EXPECTED_GROUPS}"
  log "    Relations: ${ACTUAL_RELATIONS}/${EXPECTED_RELATIONS}"
  log "═══════════════════════════════════════════════════════"
  exit 0
else
  log ""
  log "═══════════════════════════════════════════════════════"
  log "  ✗ SCIM bootstrap INCOMPLETE after ${TIMEOUT}s"
  log "    Users:     ${ACTUAL_USERS}/${EXPECTED_USERS}"
  log "    Groups:    ${ACTUAL_GROUPS}/${EXPECTED_GROUPS}"
  log "    Relations: ${ACTUAL_RELATIONS}/${EXPECTED_RELATIONS}"
  log "═══════════════════════════════════════════════════════"
  exit 1
fi
