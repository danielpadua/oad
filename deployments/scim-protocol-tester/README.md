# scim-protocol-tester

YAML-driven raw SCIM 2.0 client used to exercise OAD's SCIM ingest at
the protocol level. It is **not** part of `make dev` — its job is to
cover spec-edge cases that are awkward or impossible to drive through
a real IdP's UI/API (malformed PATCH paths, ETag conflicts, deliberately
invalid payloads, ordering anomalies).

Source-of-truth design: [docs/design/scim-ingest.md §12.2](../../docs/design/scim-ingest.md#122-scim-protocol-tester-test-utility).

## Build

```bash
go build -o ./bin/scim-protocol-tester ./deployments/scim-protocol-tester
```

A `make scim-protocol-tester` target is also available.

## Run

```bash
export OAD_SCIM_TOKEN_TEST=<the SCIM tenant token registered in OAD>
./bin/scim-protocol-tester deployments/scim-protocol-tester/scenarios
```

You can pass individual files instead of a directory:

```bash
./bin/scim-protocol-tester \
  deployments/scim-protocol-tester/scenarios/users-roundtrip.yaml \
  deployments/scim-protocol-tester/scenarios/patch-edge-cases.yaml
```

Exit code is `0` when every scenario passes, `1` otherwise.

## Scenario format

```yaml
name: <human-readable label>
target: http://localhost:8080/scim/v2
token: env:OAD_SCIM_TOKEN_TEST       # or a literal string

scenario:
  - name: create user                 # optional; defaults to "<METHOD> <path>"
    method: POST
    path: /Users
    body: { ... JSON-shaped YAML ... }
    headers:                          # optional; Authorization is auto-set
      X-Trace-Id: abc
    expect:
      status: 201
      body:                           # partial-match: every key listed must
        userName: alice               # appear with the same value (extras OK)
      body_contains:                  # raw substring assertions (each must
        - "Alice"                     # appear in the response body)
    capture:                          # extract values for later steps
      userId: id                      # dotted JSON path; "Resources.0.id" works
```

### Variable substitution

Any `{name}` token inside `path` or inside string leaves of `body` is
replaced by a previously captured value. Referencing an unknown variable
fails the step before any HTTP request is sent.

### Assertions

- `expect.status` — exact HTTP status code.
- `expect.body` — partial JSON match. Every key the YAML lists must be
  present in the response with a matching value; the response may carry
  extra keys. Arrays compare element-wise (each expected element
  partial-matches the same-indexed response element).
- `expect.body_contains` — list of substrings that must each appear in
  the raw response body. Useful for asserting `scimType` values.

### Token resolution

`token: env:VAR` reads `$VAR` at runtime. A plain literal is sent
verbatim. An empty `token` skips the `Authorization` header — useful for
asserting unauthenticated rejections.

## Adding scenarios

Each YAML file under `scenarios/` is loaded automatically when the
directory is passed as a CLI argument. Group related steps in one file
so captures (which are scoped per file) stay coherent. Aim for one
clear theme per scenario — e.g. "groups-roundtrip", "patch-edge-cases",
"invalid-payloads".

## Limitations (v1)

- No retries, no parallelism, no per-step timeout (the global `--timeout`
  applies to every request).
- ETag-conditional headers must be set manually via `headers:`; the
  tool does not auto-populate `If-Match` from a previous step.
- Only Bearer-token auth. mTLS is out of scope (SCIM endpoints are
  bearer-only by design).
