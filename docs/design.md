# mcp-go-next design

mcp-go-next is a Go library for the Model Context Protocol (MCP). It implements protocol revision 2026-07-28 and nothing older in its core. Servers and clients are both first class, the core module depends only on the standard library, and the project is set up so that people outside it can find work and land changes without asking first.

The design starts from what real users of the existing Go libraries ran into, not from a list of spec features. The evidence is summarized below. The raw data (every issue in both main libraries, their discussions, GitHub code search counts, and the go.mod files and PRs of the largest MCP projects written in Go) was collected on 2026-11-22.

## What real users told us

### Who builds with Go MCP libraries

- The largest Go MCP projects moved to the official `modelcontextprotocol/go-sdk` in 2026: `github/github-mcp-server` (34.6k stars), `containers/kubernetes-mcp-server`, `stacklok/toolhive`, `grafana/mcp-grafana` and `TykTechnologies/tyk` all depend on go-sdk v1.7 or v1.8. `hashicorp/terraform-mcp-server` is migrating tool by tool. The author of `mark3labs/mcp-go` says the same in discussion #662.
- GitHub code search finds about 67.7k Go files importing go-sdk's `mcp` package and about 24.6k importing mark3labs's `server` package.
- Three kinds of users stand out:
  - **Servers that wrap a large API.** GitHub, Kubernetes, Grafana and Terraform each register dozens to hundreds of tools. This is the largest group and the primary target of this design.
  - **Gateways and proxies.** ToolHive and Tyk aggregate upstream servers, and Grafana proxies tools from other servers.
  - **Hosts and agents.** `mark3labs/mcphost` and `f/mcptools` act as clients of many servers.

### What they use

| API | Files (code search) |
|---|---|
| go-sdk typed `mcp.AddTool` | 13,888 |
| go-sdk raw `server.AddTool` | 2,264 |
| mark3labs `mcp.NewTool` builder | 15,840 |
| go-sdk Streamable HTTP handler / stdio | 6,368 / 4,760 |
| go-sdk in-memory transports (tests) | 7,264 |
| go-sdk stateless option | 3,712 |
| go-sdk middleware / bearer auth | 2,072 / 812 |
| Elicit / CreateMessage / per-session tools | 212 / 96 / 34 |

Typed tools, Streamable HTTP, stdio, in-memory testing, middleware and auth carry almost all real use. Elicitation and sampling are rare but required by the spec, so they are covered without shaping the API around them.

### What hurts, most engagement first

1. **Sessions and scale.**
   - go-sdk #148 (shared session storage, 53 comments).
   - ToolHive had to add Redis session storage.
   - Grafana's proxied tools leaked memory per session: about 300 MB at 1,930 live sessions, and session maps were never freed (mcp-grafana #1001).
2. **Client transport fragility.** go-sdk:
   - #683: a transient error poisons the connection.
   - #1189: `Connect` ignores its deadline.
   - #513: context is not propagated.
   - #579: HTTP errors are hard to handle.
   - #522: the client wants an interface instead of `*http.Client`.
3. **Tool API ergonomics and speed with many tools.**
   - go-sdk #396 and #243, and mark3labs #104 and #108.
   - github-mcp-server forked go-sdk to cache schemas and encoded `tools/list` responses (#1530, #3407), and added a code generator for 94 input types (#1535).
4. **Per-call context from HTTP.** Grafana replaced transport hooks with one middleware that reads auth and org ID from headers on each call. "How do I read a header in a tool" and "how do I add Gin-style middleware" are among the most asked questions in mark3labs discussions, and many are unanswered.
5. **Auth.** mark3labs #540, go-sdk #901 (save and restore OAuth sessions), plus discussions on bearer middleware, token refresh and DNS rebinding.
6. **Notifications that never arrive.** mark3labs #334, #42 and #321; go-sdk #666 and #745.
7. **Extensibility.** go-sdk #954 (custom JSON-RPC methods, 22 comments) and #745 (custom notifications).
8. **CORS for browser clients.** go-sdk sets no CORS headers, which caused a regression in github-mcp-server (#2359, #3167). Grafana wrote its own.
9. **Docs and examples.** go-sdk #720 and #33, and dozens of unanswered Q&A threads.
10. **Gateways.** ToolHive's virtual MCP aggregates upstreams. Grafana retries transient discovery failures and had to stop building one client per session.

Production guidance from the Agentic AI Foundation (July 2026) points the same way:
- Replace session state with server-issued handles that have an owner, a lifetime and a cleanup path, and check ownership on every call.
- Give each caller only the tools it needs, because 40 tools cost 10 to 15 KB of schema per turn.
- Bound schema depth and validation time, and never fetch a network `$ref`.
- Mark per-user results with a private cache scope.
- Carry W3C trace context in `_meta`.

## Goals

- Make the common path short for servers with many tools: typed tools from structs, toolsets chosen per request, and `tools/list` served from a pre-encoded cache.
- Replace sessions with things that scale: a principal on every request and an owned, expiring handle store with a pluggable backend.
- A client that survives transient failures, keeps deadlines, and returns typed errors.
- Full coverage of the 2026-07-28 base protocol, server features and client features, passing every core scenario of the official conformance suite by v0.1. Auth and extension scenarios follow in later phases.
- stdio and Streamable HTTP transports for both servers and clients, mountable in any Go HTTP router.
- Zero third-party dependencies in the core module, including tests.
- Tests that pass with `-race` on Linux, macOS and Windows, with no network and no sleeps.
- A cookbook that answers the questions users actually ask, each answer backed by a tested example.
- Every row of the feature matrix is covered by a phase in `docs/plans/roadmap.md`.

## Non-goals

- API compatibility with `mark3labs/mcp-go` or `modelcontextprotocol/go-sdk`. mcp-go-next has its own API. A guide shows the equivalent code, but no shims are planned.
- Revisions before 2026-07-28 in the core. They come through the `legacy` module.
- The deprecated HTTP+SSE transport in the core. It lives in `legacy`.
- An HTTP framework, router or config system. mcp-go-next hands you an `http.Handler`.
- Mandatory code generation. Schemas come from Go types at registration, or from a type that provides its own.

## Feature matrix

The baseline is every feature in MCP 2026-07-28. On top of that comes every feature the two main Go libraries ship and every need the user research found.

**Sources:**
- `mark3labs/mcp-go` was read at `41451a37` (v1.6.0 line, 2026-10-17).
- `modelcontextprotocol/go-sdk` was read on main on 2026-11-22 (v1.8.0), from its README, file layout and issue tracker.

**Cell values:**
- "yes" means the code is there.
- "no" means it is missing or an open issue asks for it.
- "unverified" means we could not confirm it from the source and README and still need to check.

The Phase column points at `docs/plans/roadmap.md`, and the last column cites the user evidence behind the row.

### Base protocol

| ID | Feature | mark3labs | go-sdk | mcp-go-next | Phase | Evidence and where mcp-go-next aims to be better |
|---|---|---|---|---|---|---|
| B1 | 2026-07-28 stateless requests, `_meta` per request | yes | yes | yes | 1 | One code path with no session machinery |
| B2 | Revisions 2025-11-25 and older | yes | yes | adapter | 2 | Opt-in `legacy` module |
| B3 | `server/discover` | yes | yes | yes | 1 | Answered from the registry snapshot |
| B4 | Multi round-trip input requests | yes | yes | yes | 1 | Plain function call, `requestState` helpers, replay check in the test kit |
| B5 | `subscriptions/listen` | yes | unverified | yes | 1 | Bounded broker, a slow listener never blocks requests (mark3labs queue limit) |
| B6 | Cancellation | yes | yes | yes | 1 | Through `ctx` on every transport, leak tested |
| B7 | Progress | yes | yes | yes | 1 | Delivered before the response on every transport (mark3labs #334, #42) |
| B8 | Per-request log level | yes | yes | yes | 1 | No log without an explicit level |
| B9 | Caching hints `ttlMs`, `cacheScope` | yes | yes | yes | 1 | Private by default when results depend on the caller (AAIF) |
| B10 | Pagination | yes | yes | yes | 1 | Cursors stay valid while tools change (mark3labs #1214) |
| B11 | Standard headers `Mcp-Method`, `Mcp-Name`, `Mcp-Param-*` | yes | yes | yes | 1 | Checked on both sides, fuzzed |
| B12 | Error codes -32020, -32021, -32022 with data | yes | yes | yes | 1 | |
| B13 | Custom JSON-RPC methods and notifications | unverified | yes | yes | 1 | go-sdk #954, #745 |
| B14 | Large-message performance | partial | partial | yes | 1 | One decode per message, benchmarked (go-sdk #1531) |

### Server features

| ID | Feature | mark3labs | go-sdk | mcp-go-next | Phase | Evidence and where mcp-go-next aims to be better |
|---|---|---|---|---|---|---|
| S1 | Typed tools from Go structs | partial | yes | yes | 1 | The most used tool API (13.9k files). Schema built once at registration |
| S2 | Raw schema passthrough | yes | yes | yes | 1 | Byte for byte, for gateways |
| S3 | Structured output and output schema | yes | yes | yes | 1 | |
| S4 | Input validation | yes | yes | yes | 1 | Exact decimals (go-sdk #1310), case-sensitive names (mark3labs #1165) |
| S5 | Full JSON Schema 2020-12 validation | yes | yes | module | 4 | Core stays dependency-free |
| S6 | Resources and templates | yes | yes | yes | 1 | Same ergonomics as tools (kubernetes-mcp-server #1169) |
| S7 | Prompts | yes | yes | yes | 1 | |
| S8 | Completion | yes | yes | yes | 1 | |
| S9 | Logging to client (deprecated) | yes | yes | yes | 1 | |
| S10 | Registration while serving | race bugs | unverified | yes | 1 | Copy-on-write snapshots (mark3labs #1303, #1305) |
| S11 | Toolsets chosen per request | session tools | no | yes | 1 | github-mcp-server toolsets, AAIF tool pruning |
| S12 | Pre-encoded, cached `tools/list` | no | no | yes | 1 | github-mcp-server #1530, #3407 |
| S13 | Schema from a type's own method, no reflection | no | fork only | yes | 1 | github-mcp-server #1535 |
| S14 | Schema safety bounds (depth, validation time, no network `$ref`) | unverified | unverified | yes | 1 | AAIF |
| S15 | Principal on every request | context funcs | via middleware | yes | 1 | mcp-grafana #1180, mark3labs discussions #384, #506 |
| S16 | Owned, expiring handles with pluggable store | no | no | yes | 1 | go-sdk #148, mcp-grafana #1001, AAIF |
| S17 | Middleware around requests | hooks | yes | yes | 1 | go-sdk #174 |
| S18 | Panic recovery | yes | unverified | yes | 1 | |
| S19 | Tool result size limit | yes | unverified | yes | 4 | |
| S20 | Concurrency limits | yes | unverified | yes | 1 (stdio), 4 (HTTP) | |

### Client features

| ID | Feature | mark3labs | go-sdk | mcp-go-next | Phase | Evidence and where mcp-go-next aims to be better |
|---|---|---|---|---|---|---|
| C1 | Elicitation form and URL | yes | yes | yes | 1 | Typed `Form[T]` and `Answer[T]` |
| C2 | Sampling (deprecated) | yes | yes | yes | 1 | |
| C3 | Roots (deprecated) | yes | yes | yes | 1 | |
| C4 | Automatic input-request retry loop | yes | yes | yes | 1 | Clear error when no handler (mark3labs #1286) |
| C5 | Typed calls and pagination iterators | yes | yes | yes | 1 | |
| C6 | Schema passthrough to models | needs re-marshal | yes | yes | 1 | |
| C7 | Transient errors never break the connection | unverified | no (#683) | yes | 1 | go-sdk #683 |
| C8 | Retries with backoff for safe methods | unverified | no | yes | 1 | mcp-grafana #1071 |
| C9 | Typed HTTP errors with status and body | unverified | no (#579) | yes | 1 | go-sdk #579 |
| C10 | Deadlines kept everywhere | unverified | no (#1189) | yes | 1 | go-sdk #1189, #513 |

### Transports and HTTP

| ID | Feature | mark3labs | go-sdk | mcp-go-next | Phase | Evidence and where mcp-go-next aims to be better |
|---|---|---|---|---|---|---|
| T1 | stdio server | yes | yes | yes | 1 | Final line without newline, drain at EOF (go-sdk #1061) |
| T2 | stdio client and subprocess | yes | yes | yes | 1 | |
| T3 | Streamable HTTP server | yes | yes | yes | 1 | Body limit (mark3labs #1107), Origin check by default |
| T4 | Streamable HTTP client | yes | yes | yes | 1 | Fuzzed SSE reader (mark3labs #1267, #1186), `Doer` interface (go-sdk #522) |
| T5 | In-memory transport for tests | yes, ordering bug | yes | yes | 1 | Heavily used (7.3k files). Real JSON encoding and the ordering rule |
| T6 | Custom transports | yes | yes | yes | 1 | |
| T7 | HTTP+SSE transport (deprecated) | yes | yes | adapter | 2 | `legacy` only |
| T8 | Resumability and event store (removed in 2026-07-28) | yes | yes | adapter | 2 | `legacy` only |
| T9 | CORS built in, including `Mcp-*` headers | yes | no | yes | 1 | github-mcp-server #2359, #3167 |
| T10 | Mounts in any router (net/http, Gin, chi, echo) | partial | yes | yes | 1 | mark3labs discussions #308, #336, #562 |

### Authorization

| ID | Feature | mark3labs | go-sdk | mcp-go-next | Phase | Evidence and where mcp-go-next aims to be better |
|---|---|---|---|---|---|---|
| A1 | Protected resource metadata (RFC 9728) | yes | yes | yes | 2 | Resource matching per RFC (go-sdk #1538) |
| A2 | Bearer token middleware with audience checks, sets the principal | unverified | yes | yes | 2 | go-sdk discussions #223, #299 |
| A3 | `WWW-Authenticate`, 403 `insufficient_scope`, step-up | unverified | partial | yes | 2 | go-sdk #1248 |
| A4 | Client OAuth 2.1 with PKCE | yes | experimental | yes | 2 | mark3labs #540 |
| A5 | Client ID Metadata Documents | unverified | unverified | yes | 2 | |
| A6 | Dynamic client registration (deprecated) | unverified | yes | yes | 2 | Sends scopes (go-sdk #1102) |
| A7 | RFC 8707 `resource`, RFC 9207 `iss` checks | yes | unverified | yes | 2 | |
| A8 | Token refresh before expiry, saved and restored token sets | partial | no (#901) | yes | 2 | mark3labs #1122, go-sdk #901 |
| A9 | Client credentials, enterprise-managed authorization, token exchange | unverified | yes | yes | 4 | |
| A10 | DPoP | unverified | unverified | yes | 4 | |

### Gateways

| ID | Feature | mark3labs | go-sdk | mcp-go-next | Phase | Evidence and where mcp-go-next aims to be better |
|---|---|---|---|---|---|---|
| G1 | Aggregate upstream servers with name prefixes | no | no | yes | 3 | ToolHive virtual MCP |
| G2 | Upstream clients shared per credential, never per caller | no | no | yes | 3 | mcp-grafana #1001 |
| G3 | Upstream discovery cache with retries | no | no | yes | 3 | mcp-grafana #1071 |
| G4 | Pass through input requests, progress and cancellation to upstreams | no | no | yes | 3 | |

### Extensions

| ID | Feature | mark3labs | go-sdk | mcp-go-next | Phase | Evidence and where mcp-go-next aims to be better |
|---|---|---|---|---|---|---|
| E1 | Tasks (`io.modelcontextprotocol/tasks`) | yes | no (#626) | yes | 4 | |
| E2 | Skills over MCP | unverified | unverified | yes | 4 | Used by github-mcp-server (#3685) |
| E3 | MCP Apps (UI) | unverified | unverified | yes | 4 | Used by mcp-grafana (#1309) |
| E4 | Declaring extensions in capabilities | yes | yes | yes | 1 | |

### Operations, testing and docs

| ID | Feature | mark3labs | go-sdk | mcp-go-next | Phase | Evidence and where mcp-go-next aims to be better |
|---|---|---|---|---|---|---|
| O1 | OpenTelemetry traces and metrics, W3C trace context in `_meta` | yes (module) | unverified | yes (module) | 4 | go-sdk #59, AAIF |
| O2 | In-flight and per-method metrics hook | unverified | unverified | yes | 1 | kubernetes-mcp-server #1502 |
| O3 | Rate limiting by principal | no (#896) | example | yes | 4 | |
| O4 | Test kit | yes | in-memory | yes | 1 | Pair, leak check, replay check, golden files |
| O5 | Official conformance suite in CI | yes | yes | yes | 1 | |
| O6 | Interop with reference servers and clients | unverified | unverified | yes | 3 | |
| O7 | Fuzz tests | unverified | unverified | yes | 1 | |
| O8 | Third-party dependencies in the core | 7 direct | several | 0 | 1 | |
| O9 | Cookbook of tested answers to real questions | partial | partial (#720) | yes | 1 | Unanswered discussions in both repos |
| O10 | Equivalent-code guide for users of other libraries | n/a | n/a | yes | 3 | Terraform's tool-by-tool migration |

### Other Go libraries

Three smaller libraries also exist. Their feature cells are unverified and get checked in phase 3:

- `metoro-io/mcp-golang` (1.2k stars) aims at writing servers in few lines.
- `ThinkInAIXYZ/go-mcp` (681 stars) is a general SDK.
- `panyam/mcpkit` (7 stars) claims a 100% conformance tier check and ships OAuth, tasks, skills, apps and OpenTelemetry.

mcpkit shows that conformance alone does not draw users. The focus here is the problems above.

### Where "better" is not proven yet

- **Maturity and trust.** go-sdk is official and used by the largest projects. mcp-go-next has no users. Earn it with published conformance, interop and benchmark results on every release, and a first real adopter.
- **Performance.** The cached list and single-decode design should beat both on servers with many tools, but that is a claim until measured. Earn it with side-by-side benchmarks in phase 3, published even where we lose.
- **Older revisions.** Both support them in their core. mcp-go-next needs `legacy` (phase 2).
- **Unverified cells.** Each one gets checked against the source before phase 3, and the matrix is corrected.
- **Contributors.** One maintainer today. Earn it with labeled issues for every open row and fast reviews.

## Module and packages

Module path `github.com/raven-clown/mcp-go-next`. The root package is named `mcp`. mcp-go-next targets the latest Go release only (1.27.4 today). `go.mod` names it, CI reads it from there, and a `chore/` pull request bumps it after each Go release.

| Package | Purpose |
|---|---|
| `mcp` (module root) | `Server`, `Client`, `Conn`, `Handler`, `Middleware`, `Request`, `Principal`, toolsets, `Handles`, registration, typed tool helpers, `Error` |
| `protocol` | Wire types for 2026-07-28 |
| `jsonrpc` | JSON-RPC 2.0 messages, decoding and encoding, public for custom transports |
| `jsonschema` | Schema inference from Go types, `SchemaProvider`, `Validator`, a bounded basic validator |
| `transport/stdio` | stdio server (`Serve`) and client (`Command`, `Pipe`) |
| `transport/streamable` | Streamable HTTP server (`Handler`, with CORS) and client (`Dial`) |
| `mcptest` | In-memory pair, leak check, replay check, golden helpers |
| `gateway` | Phase 3: aggregate upstream servers |
| `middleware` | Phase 4: rate limit, result size limit, HTTP concurrency limit, `slog` request logging |
| `conformance/server`, `conformance/client` | Programs driven by the official conformance suite |
| `examples/...`, `docs/cookbook/` | Tested examples and the cookbook |

Phase N ships as v0.N. Separate modules, each with its own `go.mod`, so the core never imports them:

| Module | Phase | Purpose |
|---|---|---|
| `.../mcp-go-next/auth` | 2 | OAuth 2.1 for servers and clients |
| `.../mcp-go-next/legacy` | 2 | Revisions 2025-11-25 and older: handshake, sessions, HTTP+SSE, resumability |
| `.../mcp-go-next/ext/tasks` | 4 | Tasks extension |
| `.../mcp-go-next/ext/skills` | 4 | Skills over MCP |
| `.../mcp-go-next/ext/apps` | 4 | MCP Apps |
| `.../mcp-go-next/jsonschema/full` | 4 | Full JSON Schema 2020-12 validator |
| `.../mcp-go-next/otel` | 4 | OpenTelemetry middleware |

Dependency rules:

- `protocol` and `jsonrpc` import only the standard library.
- `mcp` imports `protocol`, `jsonrpc` and `jsonschema`.
- Transports and `gateway` import `mcp`, never the other way around.
- Library packages never print and never touch `os.Stdout` or `os.Stderr`.

## Server

### Handler and middleware

```go
type Handler interface {
	Handle(ctx context.Context, req *Request) (protocol.Result, error)
}
type HandlerFunc func(ctx context.Context, req *Request) (protocol.Result, error)
type Middleware func(Handler) Handler
```

`*mcp.Server` implements `Handler`. `srv.Use(mw...)` wraps the method dispatch, so middleware sees every request after the version check and before the method handler. Auth, logging, metrics and rate limits are middleware. Panic recovery is always on.

### Request and principal

`*mcp.Request` is built once per message by the transport:

- `Method`, and `Params` (raw, decoded lazily by the method handler)
- `Meta`: protocol version, client capabilities, client info, log level, progress token, input responses, the raw `_meta`
- `Header http.Header` on HTTP transports, nil on stdio
- `Principal()`: who is calling, set by auth middleware with `req.SetPrincipal(p)`

```go
type Principal struct {
	Subject string
	Issuer  string
	Scopes  []string
	Claims  map[string]any
}
```

The owner key of a principal is the issuer's length, a colon, the issuer, a space and the subject, such as `19:https://idp.example alice`. The length keeps two principals from sharing a key when an issuer has a space in it. A request without a principal has the empty owner, which is what local stdio servers get.

The envelope and `_meta` are decoded once. Params stay raw until a handler asks for them.

### Dispatch

For each request the server:

1. Rejects an unsupported protocol version with -32022.
2. Takes a snapshot of the registry.
3. Runs middleware.
4. Resolves the request's toolsets.
5. Runs the method handler.
6. Stamps `resultType`, `ttlMs`, `cacheScope` and server info on the result.

Unknown methods return -32601. `srv.HandleMethod(name, h)` registers a method the spec does not define, through the same checks and middleware. The client sends custom messages with `conn.Request` and `conn.Notify`.

### Registry

Tools, resources, templates, prompts and completion live in a copy-on-write registry. A request keeps the snapshot it started with. Lists are sorted by name, and cursors stay valid when items change between pages.

### Tools

```go
func AddTool[In, Out any](s *Server, name, description string, fn ToolFunc[In, Out], opts ...ToolOption)
type ToolFunc[In, Out any] func(ctx context.Context, req *Request, in In) (Out, error)
func AddRawTool(s *Server, tool protocol.Tool, fn RawToolFunc)
```

- The input and output schemas are built **once at registration** from the Go types. If `In` or `Out` implements `jsonschema.SchemaProvider` (`MCPSchema() json.RawMessage`), that schema is used and no reflection runs for it.
- When `Out` is a struct, the result carries it in `structuredContent` with a JSON text copy in `content`. `*protocol.CallToolResult` is returned as is.
- Arguments are validated before `fn` runs. A failure is a tool error result.
- `AddRawTool` passes a `json.RawMessage` schema through byte for byte.
- Annotations, `x-mcp-header` parameters, toolsets and icons are set with `ToolOption`.

### Toolsets

```go
mcp.AddTool(srv, "list_issues", "List issues", listIssues, mcp.InToolset("issues"))
srv.DescribeToolset("issues", "Read and write issues")
srv = mcp.NewServer(info, mcp.WithToolsetSelector(func(ctx context.Context, req *mcp.Request) []string {
	return strings.Split(req.Header.Get("X-Toolsets"), ",")
}))
```

- A tool without `InToolset` is in the `default` toolset, which every request sees.
- The selector runs once per request. With no selector, every toolset is visible.
- When the selector's result contains `mcp.ReadOnlyToolsets` (the name `"read-only"`), only tools whose `readOnlyHint` is true stay visible inside the chosen toolsets.
- A tool outside the request's toolsets is invisible in `tools/list` and answers `tools/call` with the same error as an unknown tool. Hiding never leaks existence.
- `WithToolFilter(func(ctx, *Request, protocol.Tool) bool)` is the per-tool escape hatch. Requests that go through it skip the list cache, and the docs say so.

### Pre-encoded list cache

Each registry snapshot keeps the encoded `tools/list` pages per toolset combination, built on first use. The cache key is the sorted set of toolset names plus the read-only flag. A registry change builds a new snapshot, so stale pages are never served. Resources, templates and prompts use the same cache.

### Handles instead of sessions

```go
carts := mcp.NewHandles[Cart](mcp.HandleTTL(30*time.Minute), mcp.HandleLimit(10_000), mcp.OnExpire(releaseCart))
id, err := carts.Create(ctx, req, cart)
cart, err := carts.Get(ctx, req, id)
err = carts.Update(ctx, req, id, cart)
err = carts.Delete(ctx, req, id)
```

- An ID is 128 random bits, base64url encoded.
- Every call checks that the stored owner equals the request's principal owner key. An unknown or expired ID returns `mcp.ErrHandleNotFound`. A foreign one returns `mcp.ErrHandleNotFound` as well, so IDs cannot be probed.
- The memory store bounds both count and age. When the limit is reached, the oldest handle is evicted and `OnExpire` runs.
- `mcp.WithHandleStore(s)` plugs in any backend that implements `HandleStore`:

```go
type HandleStore interface {
	Put(ctx context.Context, id, owner string, value []byte, ttl time.Duration) error
	Get(ctx context.Context, id string) (owner string, value []byte, err error)
	Delete(ctx context.Context, id string) error
}
```

  Values are JSON encoded for every store. `Update` keeps the deadline `Create` set: it calls `Put` with ttl 0, which replaces the value of an existing entry only, like `SET XX KEEPTTL` in Redis. The cookbook has a Redis example outside the core module.

### Cache scope

List and read results get `cacheScope: private` by default when the request has a principal, or when a toolset selector or tool filter is set. Otherwise they get the server default (`WithCache`). A handler can still set its own.

### Errors

- A plain `error` from a tool becomes a result with `isError: true`.
- A `*mcp.Error` becomes a JSON-RPC error. There are constructors for every spec code.
- A panic becomes -32603 with a generic message. The stack goes to `WithErrorLog`.

### Multi round-trip requests

```go
answer, err := mcp.Elicit(ctx, req, "confirm", mcp.Form[Confirm]{Message: "Delete 3 files?"})
```

- With an answer under the key in `inputResponses`, `Elicit` returns it. Without one, it records a request and returns `mcp.ErrInputRequired`. The server collects all recorded requests into one `InputRequiredResult`.
- `req.Sample` and `req.Roots` work the same way. Input requests are allowed only on `tools/call`, `prompts/get` and `resources/read`, and need the matching client capability (else -32021).
- `req.State()` and `req.SetState(s)` carry `requestState`.
- Several answers: call `Elicit` for each and return `errors.Join` of the errors.
- The handler runs from the top each round, so side effects go after the last `Elicit`. `mcptest.CheckReplaySafe` checks it.

### Progress, logging, cancellation, metrics

- `req.Progress` sends only when the request has a progress token. `req.Log` sends only at or above the request's log level, and never without one.
- Every transport delivers a request's notifications before its response, and nothing after it.
- Cancellation arrives as `ctx` cancellation.
- `WithObserver(o)` gets `OnStart(req)` and `OnEnd(req, result, err, duration)` for every request, so metrics such as in-flight counts need no middleware.

### Subscriptions

`subscriptions/listen` streams through a bounded broker:
- The ack comes first.
- Only the notification types the client asked for are sent.
- `subscriptionId` is on every notification.
- A graceful end returns the listen result, and on stdio also sends `notifications/cancelled`.
- A slow listener is closed. Publishing never blocks. Registry changes publish list-changed events.

### Discovery

`server/discover` is answered from the registry and server info.

## Client

```go
c := mcp.NewClient(mcp.Info{Name: "agent", Version: "0.1.0"}, mcp.ClientOptions{OnElicit: askUser})
conn, err := c.Connect(ctx, streamable.Dial("https://example.com/mcp"))
out, err := mcp.Call[ForecastOut](ctx, conn, "forecast", ForecastIn{City: "Bangkok"})
```

- `Connect` does no I/O. Every request carries `_meta` with version, capabilities and client info.
- Typed helpers, pagination iterators, custom requests, and an automatic input-request loop with a round limit. A missing handler fails with method not found.
- **Resilience:**
  - A transport error fails only that call. The `Conn` stays usable until `Close`.
  - `ClientOptions.Retry` (default 3 attempts, exponential backoff from 100 ms to 2 s with jitter, injectable clock) retries network errors, 429 (honoring `Retry-After`), 502, 503 and 504 for the safe methods: `server/discover`, the four list methods, `resources/read`, `prompts/get` and `completion/complete`. `tools/call` is retried only for tools whose cached annotations say `idempotentHint: true`.
  - Every call keeps its context deadline, including connect, headers and each streamed event.
- Errors:
  - `*mcp.Error` for JSON-RPC errors.
  - `*streamable.HTTPError{Status, Header, Body}` (body up to 4 KiB) for HTTP failures without a JSON-RPC body.
  - `mcp.ErrClosed` after `Close`.
- For Streamable HTTP, `Mcp-Param-*` headers come from a tool list cached per `ttlMs` and invalidated by list-changed events.

### Transport interface

```go
type Transport interface {
	RoundTrip(ctx context.Context, msg *jsonrpc.Message, notify func(*jsonrpc.Message)) (*jsonrpc.Message, error)
	Notify(ctx context.Context, msg *jsonrpc.Message) error
	Close() error
}
```

## Transports

### stdio

- **Server:** `stdio.Serve(ctx, h, r, w, opts...)`.
  - Newline-delimited, with a line limit.
  - Reads a final line that has no newline.
  - Drains in-flight requests at EOF, up to a drain timeout.
  - Concurrency limit and a single writer.
- **Client:** `stdio.Command` and `stdio.Pipe`.
  - Matches responses by ID, progress by token and listen notifications by subscription ID.
  - Anything else goes to `WithUnrouted`.
  - Shutdown is close stdin, wait, kill.

### Streamable HTTP

**Server:** `streamable.Handler(h, opts...)` is a plain `http.Handler` that mounts anywhere.
- POST only. GET and DELETE return 405.
- Origin allowlist (localhost by default) and 403 on mismatch.
- Body limit (default 4 MiB).
- Standard headers checked against the body, -32020 on mismatch.
- 202 for notifications. JSON, or SSE once the handler sends a notification.
- Status mapping: 400 for -32020, -32021 and -32022, 404 for -32601.

**CORS:** `streamable.WithCORS(origins...)`, on for the localhost origins by default.
- Preflight `OPTIONS` gets 204.
- `Access-Control-Allow-Headers` lists `Content-Type`, `Authorization`, `MCP-Protocol-Version`, `Mcp-Method`, `Mcp-Name`, and every `Mcp-Param-*` header the server's tools declare.
- `WWW-Authenticate` is exposed.

**Client:** `streamable.Dial(url, opts...)`.
- Required headers with base64 for non-ASCII values.
- A `Doer` interface for auth, tracing and custom transports.
- A fuzzed SSE reader.
- Typed HTTP errors.

### In-memory

`mcptest.NewPair` wires a client to a handler through real JSON encoding with the same ordering rule.

## Gateway (phase 3)

```go
gw := gateway.New(
	gateway.Upstream("github", streamable.Dial(githubURL), gateway.Prefix("gh_")),
	gateway.Upstream("grafana", stdio.Command("mcp-grafana")),
)
srv.Mount(gw)
```

- Tool, prompt and resource names get the upstream's prefix.
- Upstream clients are keyed by credential (`gateway.CredentialKey(func(*Request) string)`), never by caller, so memory does not grow with the number of callers.
- Upstream lists are cached per `ttlMs`, and discovery retries with backoff.
- Input requests, progress and cancellation pass through in both directions.

## Testing

- Tests come with every change and are written first, table-driven where there is a table.
- No network in unit tests, no bare sleeps, Windows passes.
- Leak check in every transport test.
- Golden wire tests against the vendored spec examples, and a field comparison with `schema.json`.
- Fuzz tests for JSON-RPC, SSE and headers.
- Benchmarks:
  - a 1 MiB `tools/call`
  - `tools/list` with 500 tools across 20 toolsets, cold and cached
  - handle create and get under contention
- Conformance in CI with a reviewed baseline.
- Coverage of 80% or more.

## Docs

- `README.md`: what it is and a 20-line server.
- `docs/getting-started.md`.
- `docs/cookbook/`: one page per real question, each backed by a program under `examples/` that CI builds and tests:
  - mount in net/http, Gin, chi and echo
  - read headers and the principal in a tool
  - bearer auth
  - toolsets per user
  - handles for state across calls
  - progress
  - elicitation
  - CORS for browser clients
  - running several instances behind a load balancer
  - testing with `mcptest`
  - a Redis handle store
- `docs/coming-from.md` (phase 3): equivalent code for common mark3labs and go-sdk patterns, side by side. A guide, not a shim.
- `docs/spec-coverage.md`: one row per spec feature with status and issue.

## Project and contributor setup

- License Apache-2.0 with `NOTICE`. No CLA and no DCO.
- `AGENTS.md`, `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md` (Contributor Covenant 2.1), `SECURITY.md` (private vulnerability reporting).
- Issue forms (bug, feature, spec gap, `config.yml` pointing questions to Discussions), a pull request template, `CODEOWNERS` with `@raven-clown`.
- Labels:
  - kind: `bug`, `enhancement`, `docs`, `breaking`, `spec`
  - help: `good first issue`, `help wanted`
  - area: `area/server`, `area/client`, `area/stdio`, `area/http`, `area/schema`, `area/auth`, `area/legacy`, `area/gateway`, `area/ext`, `area/ci`
  - priority: `priority/p0` to `priority/p3`
- Milestones `v0.1` to `v0.4` and `v1.0`.
- A GitHub Project "mcp-go-next roadmap" with one issue per plan task and roadmap task, and the fields Phase, Area and Priority.
- Repository settings:
  - Discussions on
  - private vulnerability reporting on
  - approval before workflows run for all outside contributors
  - squash merge only
  - delete branch on merge
- Ruleset on `main`:
  - pull request required
  - required checks `test`, `lint`, `vuln`, `coverage`, then `conformance` once it exists
  - conversations resolved before merge
  - linear history
  - no force push and no deletion
- `CHANGELOG.md` in Keep a Changelog form, releases as `vX.Y.Z` tags.

### CI

- `test`: ubuntu, macos and windows with the Go version from `go.mod`, `-race`
- `lint`, `vuln`, `coverage`
- `conformance`
- `interop` from phase 3
- `bench` on pull requests touching hot paths, reporting the change against main
- Dependabot for Actions and for modules outside the core

## Roadmap

`docs/plans/roadmap.md` maps every matrix row to a phase:

- **Phase 1 (v0.1):** the core, toolsets and list cache, principal and handles, both transports with CORS, a resilient client, the test kit, the cookbook. Every 2026-07-28 core conformance scenario passes.
- **Phase 2 (v0.2):** `auth` and `legacy`.
- **Phase 3 (v0.3):** `gateway`, interop, side-by-side benchmarks, matrix verification, the coming-from guide.
- **Phase 4 (v0.4):** extensions, `jsonschema/full`, `otel`, `middleware`, remaining auth.
- **v1.0:** API freeze, full conformance, tier listing application.

## Risks

- **The spec moves.** `protocol` is the only place wire shapes live, and the schema comparison test flags drift.
- **Input-request replay surprises handler authors.** Docs, an example and `mcptest.CheckReplaySafe`.
- **Few hosts may speak 2026-07-28 yet.** `legacy` in phase 2 covers older clients, and the README says so until then.
- **The official SDK has momentum.** The largest projects just finished moving to go-sdk and will not move again without a clear gain. mcp-go-next earns a place by solving the problems they reported, with measurements, and by being easy to try in one tool at a time.
- **One maintainer and a large scope.** Phases ship on their own, the matrix shows what is not done, and labeled issues let others help.
