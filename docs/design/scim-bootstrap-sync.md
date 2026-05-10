# SCIM Bootstrap Sync Design

## Problem Context

During the initial orchestration and startup of the OAD platform's `multi-idp` development environment, Authentik's internal Celery worker executes the SCIM synchronization background task shortly after it boots up. However, the OAD `api` service and the Authentik blueprints that seed users may not be fully initialized and ready to receive these requests or provide all the necessary users. 

This causes a race condition where:
1. Authentik triggers its initial SCIM sync before all blueprint users and groups are provisioned.
2. The initial sync fails or partially completes because the OAD API is not yet healthy or the users do not exist yet.
3. Authentik's incremental SCIM sync logic may not re-sync certain relations properly because it assumes objects were already synchronized, leaving the OAD database missing some critical user-group relationships (e.g. `member_of` relations missing).

## Solution: Deterministic Init Container

To eliminate this race condition and ensure a fully seeded database on every fresh bootstrap, we introduced a deterministic one-shot init container called `scim-sync-init`.

### How it works

The `scim-sync-init` container executes a shell script (`deployments/multi-idp/scripts/scim-sync-init.sh`) that waits for the entire stack to be in a known healthy state before programmatically triggering a full SCIM sync and verifying its completion. The script performs the following sequential steps:

1. **Wait for Authentik API & Token**: It polls the Authentik API until it's reachable and the deterministic API Token (`scim-sync-init-token`), provisioned via the blueprint, is successfully authenticated.
2. **Wait for Blueprint Users**: It repeatedly queries Authentik for the expected number of blueprint users (e.g., `admin@oad.dev`, `viewer@oad.dev`), ensuring the blueprints have fully applied.
3. **Locate SCIM Provider**: It queries Authentik to find the primary key (PK) of the `oad-scim-provider`.
4. **Wait for OAD API Health**: It polls the OAD API's `/health` endpoint until the API is fully started, migrations are complete, and it is ready to process incoming SCIM payloads.
5. **Trigger Full SCIM Sync**: Instead of relying on Authentik's automated task schedule, it calls `POST /api/v3/providers/scim/{pk}/sync/` directly on the `oad-scim-provider`. This endpoint always schedules a **full** sync for that specific provider, unlike re-firing a historical system task (which may run an incremental sync based on prior state).
6. **Verify OAD Database State**: Finally, it connects directly to the PostgreSQL database (`oad`) and counts the SCIM-provisioned `User` entities, `Group` entities, and `member_of` relations. It retries this verification up to a maximum timeout, ensuring the data ingestion is functionally verified. Whenever any count falls short, it re-triggers the provider sync (with a 30-second cooldown) to heal partial states — regardless of which entity type is missing.

### Implementation Details

- **Docker Compose Orchestration**: The `scim-sync-init` container is added to `docker-compose.yml` under `deployments/multi-idp`. It depends on `api` (started) and `authentik-server` (healthy).
- **Security & Tokens**: A deterministic API token is added directly into the `oad.yaml` Authentik blueprint. This avoids the unreliable process of generating tokens via HTTP Basic Auth or relying on initial bootstrap environment variables which can be delayed.
- **CI/CD Reliability**: The CI workflow (`.github/workflows/scim-integration.yml`) is simplified. Instead of polling the database directly, it simply waits for the `scim-sync-init` container to exit successfully, knowing that the container handles all the robustness and verification checks internally.
