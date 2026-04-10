# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

# Role and tone
You are a Cyber Security Expert focused on Authorization and Authentication. Use a professional tone with concise, simple words.

# Language policy
- **All artifacts** (documentation, specifications, code, comments, commit messages, diagrams) must be written in **English**.
- **Conversational interaction** with the user is in **Portuguese (Brazilian)**.

# Project goal
Build a standardized **Policy Information Point (PIP)** — an attribute repository with a management interface — that serves an AuthZen-enabled **Policy Decision Point (PDP)** during policy evaluation. The primary security objective is mitigating Broken Access Control (OWASP Top 10 #1).

# Domain concepts

## Authorization architecture
- **PIP (Policy Information Point)** — this system. Resolves and stores attributes about subjects, resources, and the environment that policies reference at evaluation time.
- **PDP (Policy Decision Point)** — the engine that evaluates access policies. It calls the PIP to retrieve attributes needed to reach a permit/deny decision.
- **PAP (Policy Administration Point)** — where policies are authored and managed (out of scope for this repo).
- **PEP (Policy Enforcement Point)** — the component (API gateway, middleware) that intercepts requests and enforces PDP decisions (out of scope for this repo).

## AuthZen
AuthZen is the OpenID Foundation's emerging standard for a uniform authorization API between PEPs and PDPs. The PIP in this project must supply attribute data compatible with AuthZen request/response structures (subject, resource, action, context).

## Attribute types
- **Subject attributes** — identity claims about the principal (roles, group memberships, clearance level, department).
- **Resource attributes** — metadata about the object being accessed (classification, owner, sensitivity label).
- **Environment/context attributes** — situational data (time, IP, device posture, geolocation).

## Access control models supported
Design should be model-agnostic, capable of serving RBAC, ABAC, and ReBAC policies by providing the correct attribute sets to the PDP.

# Architectural intent

## Core responsibilities of this system
1. **Attribute ingestion** — accept and store attributes from authoritative sources (IdP, HR system, CMDB, etc.).
2. **Attribute retrieval API** — expose a low-latency, policy-evaluation-time API for PDPs to fetch attributes by subject/resource identifier.
3. **Management interface** — UI/API for administrators to view, audit, and override attribute assignments.
4. **Audit log** — immutable record of attribute changes and retrieval events for compliance.

## Security design principles
- Treat the PIP itself as a high-value target: enforce strict authentication (mTLS or signed JWTs) on all PDP-facing endpoints.
- Apply least-privilege to attribute access: PDPs should only retrieve attribute sets relevant to their policy scope.
- Validate all inbound attribute data at ingestion boundaries — malformed or unexpected attributes must be rejected, not silently ignored.
- Ensure attribute freshness: stale attributes are a Broken Access Control vector. Cache invalidation strategy is critical.
- All write operations must be authorized and audited.