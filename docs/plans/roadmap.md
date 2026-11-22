# mcp-go-next roadmap

Every row of the feature matrix in `docs/design.md` lands in exactly one phase below. Phase N ships as v0.N. Each phase is useful on its own.

Phase 1 has a full task-by-task plan in `docs/plans/2026-11-22-v0.1.md`. Phases 2 to 4 are outlined here and get full plans when the phase before them is merged, because those plans depend on names and types phase 1 settles.

Each outline task becomes one GitHub issue with labels, a milestone and Project fields. Self-contained tasks get `good first issue`.

## Phase 1: core for servers with many tools (v0.1)

**Rows:** B1, B2 (handshake and sessions), B3 to B14, S1 to S4, S6 to S18, S20 (stdio), C1 to C10, T1 to T6, T9, T10, E4, O2, O4, O5, O7, O8, O9.

**Plan:** `docs/plans/2026-11-22-v0.1.md`. Its "Order of work" section builds servers first, then the `legacy` initialize handshake, then clients, so that mcp-go-next can replace mark3labs/mcp-go in real projects before the rest of phase 1 lands. The handshake moved here from phase 2 because most hosts still speak 2025-11-25.

**Done when:**
- every 2026-07-28 core conformance scenario passes
- CI is green on three systems
- coverage is at least 80%
- every cookbook page has a tested example

## Phase 2: auth and legacy (v0.2)

**Rows:** B2 (the rest), T7, T8, A1 to A8.

1. `auth` module skeleton and CI wiring.
2. Protected resource metadata handler with resource matching per RFC 9728 (A1, go-sdk #1538).
3. Bearer token middleware with a `TokenVerifier` interface and audience check that sets the request principal (A2).
4. `WWW-Authenticate` with `resource_metadata`, 403 `insufficient_scope`, step-up helper (A3, go-sdk #1248).
5. Authorization server metadata discovery (RFC 8414 and OIDC).
6. Client OAuth 2.1 with PKCE, RFC 8707 `resource`, RFC 9207 `iss` check (A4, A7).
7. Client ID Metadata Documents, then pre-registration, then DCR with scopes (A5, A6, go-sdk #1102).
8. Token refresh before expiry, plus a `TokenStore` interface to save and restore token sets (A8, mark3labs #1122, go-sdk #901).
9. Auth conformance suite in CI.
10. `legacy` server-to-client requests mapped onto input-request handlers.
11. `legacy` GET stream, list-changed notifications and resource subscribe mapped onto the broker, on both the server and client side.
12. `legacy` HTTP+SSE transport, server and client (T7).
13. `legacy` resumability with an `EventStore` interface (T8).
14. Legacy conformance scenarios removed from the baseline.

The `legacy` module skeleton and the initialize handshake with `Mcp-Session-Id`, for servers and clients, moved to phase 1 as Tasks L1 to L3.

**Done when:** the baseline holds only phase 4 items, and a 2025-11-25 client works through `legacy`.

## Phase 3: gateways, interop, proof (v0.3)

**Rows:** G1 to G4, O6, O10.

1. Check every "unverified" matrix cell, including the three smaller libraries, against their source, and correct the matrix.
2. `gateway` package: upstream registry and name prefixes (G1).
3. Upstream clients keyed by credential, with a test that memory stays flat as callers grow (G2, mcp-grafana #1001).
4. Upstream list cache and discovery retries (G3, mcp-grafana #1071).
5. Pass-through of input requests, progress and cancellation (G4).
6. `interop` CI job: our client against the reference `everything` server and the TypeScript and Python SDK example servers. Their clients and the Inspector CLI against our server. Results in `docs/interop.md` (O6).
7. Side-by-side benchmarks against mark3labs and go-sdk:
   - `tools/list` with 500 tools
   - a 1 MiB `tools/call`
   - 1,000 concurrent small calls
   - memory per 1,000 callers

   Published in `docs/benchmarks.md` with machine and versions, including results where we lose.
8. `docs/coming-from.md`: equivalent code for common mark3labs and go-sdk patterns (O10).

**Done when:** the interop matrix is green and the benchmark page is published.

## Phase 4: extensions and operations (v0.4)

**Rows:** S5, S19, S20 (HTTP), A9, A10, E1 to E3, O1, O3.

1. `jsonschema/full` module (S5).
2. `middleware` package: result size limit (S19), HTTP concurrency limit (S20), rate limit by principal (O3), `slog` request logging.
3. `otel` module with W3C trace context in `_meta` (O1).
4. `ext/tasks` with a task store interface and the tasks conformance scenarios (E1).
5. `ext/skills` and its conformance scenarios (E2).
6. `ext/apps` (E3).
7. `auth` client credentials, enterprise-managed authorization, token exchange (A9).
8. `auth` DPoP (A10).

**Done when:** the conformance baseline is empty.

## v1.0

1. API review against how phase 1 to 4 users actually call it. Breaking changes are listed before the freeze.
2. Full conformance suite with an empty baseline on every release.
3. Apply for a listing on the MCP SDK tiers page.
4. Publish conformance, interop and benchmark results with the release.

## Row index

| Rows | Phase |
|---|---|
| B1, B3 to B14 | 1 |
| B2 | 1 (handshake and sessions), 2 (the rest) |
| S1 to S4, S6 to S18 | 1 |
| S5, S19 | 4 |
| S20 | 1 (stdio), 4 (HTTP) |
| C1 to C10 | 1 |
| T1 to T6, T9, T10 | 1 |
| T7, T8 | 2 |
| A1 to A8 | 2 |
| A9, A10 | 4 |
| G1 to G4 | 3 |
| E4 | 1 |
| E1 to E3 | 4 |
| O2, O4, O5, O7, O8, O9 | 1 |
| O6, O10 | 3 |
| O1, O3 | 4 |
