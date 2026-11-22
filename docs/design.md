# tether design

tether is a Go library for the Model Context Protocol (MCP). It implements protocol revision 2026-07-28 and nothing older in its core. Servers and clients are both first class, the core module depends only on the standard library, and the project is set up so that people outside it can find work and land changes without asking first.

## Why another Go MCP library

Two good libraries exist. `mark3labs/mcp-go` is the most used and moves fast. `modelcontextprotocol/go-sdk` is the official SDK. Both grew up on the handshake era (2024-11-05 to 2025-11-25) and now carry that era and the stateless 2026-07-28 revision in the same code: sessions, the GET stream, server-to-client requests and the new per-request metadata all live side by side. Their issue trackers show where that hurts: races between registration and capability checks, transport edge cases (stdio EOF, SSE line endings, queue limits), notifications arriving after results, repeated JSON parsing on large messages, and auth that breaks against strict servers.

2026-07-28 removed the handshake and sessions. A server is now close to a function from a request to a result. tether is built on that model from the start:

- **Stateless core.** No sessions, no goroutines in the server, no per-connection state. Easy to reason about, easy to scale behind a load balancer, easy to test.
- **Small API in the shape of `net/http`.** A `Handler`, middleware as `func(Handler) Handler`, and typed tools through generics.
- **Correct by test.** The official conformance suite runs in CI from the first release, and every transport has ordering, leak and fuzz tests.
- **Older revisions are an adapter, not a mode.** A separate `legacy` module (v0.2) translates the 2025-11-25 handshake and sessions into per-request metadata at the transport edge. Users who do not need it never compile it.

## Goals

- Full coverage of the 2026-07-28 base protocol, server features and client features, passing every core scenario of the official conformance suite by v0.1. Auth and extension scenarios follow in v0.2 and v0.3, which is what Tier 1 needs.
- stdio and Streamable HTTP transports for both servers and clients.
- Zero third-party dependencies in the core module, including tests.
- Tests that pass with `-race` on Linux, macOS and Windows, with no network and no sleeps.
- A contributor can read `docs/spec-coverage.md`, pick an open row, and find a labeled issue for it.
- Every row of the feature matrix below is covered by a phase in `docs/plans/roadmap.md`. Nothing either competitor does is left out without a written reason.
- A project on `mark3labs/mcp-go` or `modelcontextprotocol/go-sdk` can move by changing import paths, with a tool that does the rewrite and lists anything not covered yet.

## Non-goals

- Revisions before 2026-07-28 in the core. They come through the `legacy` module.
- The deprecated HTTP+SSE transport in the core. It lives in the `legacy` module for projects that still serve it.
- An HTTP framework, router or config system. tether hands you an `http.Handler`.
- Code generation. Wire types are written by hand and checked against the official schema.

## Feature matrix

The baseline is every feature in MCP 2026-07-28. On top of that, every feature either competitor ships is listed, so nothing gets dropped by accident.

**Sources:** `mark3labs/mcp-go` was read at `41451a37` (v1.6.0 line, 2026-10-17). `modelcontextprotocol/go-sdk` was read on main on 2026-11-22 (v1.8.0), from its README, file layout and issue tracker.

**Cell values:**
- "yes" means the code is there.
- "no" means it is missing or an open issue asks for it.
- "unverified" means we could not confirm it from the source and README and still need to check.

The Phase column points at `docs/plans/roadmap.md`.

### Base protocol

| ID | Feature | mark3labs | go-sdk | tether | Phase | Where tether aims to be better |
|---|---|---|---|---|---|---|
| B1 | 2026-07-28 stateless requests, `_meta` per request | yes | yes | yes | 1 | One code path. Neither competitor can drop the session machinery without breaking users |
| B2 | Revisions 2025-11-25 and older | yes | yes | adapter | 2 | Opt-in `legacy` module, so the core stays free of era branches |
| B3 | `server/discover` | yes | yes | yes | 1 | Answered from the registry snapshot, never stale |
| B4 | Multi round-trip input requests (MRTR) | yes | yes | yes | 1 | Plain function call, `requestState` helpers, and a replay safety check in the test kit |
| B5 | `subscriptions/listen` | yes | unverified | yes | 1 | Bounded broker: a slow listener is closed and never blocks requests (mark3labs queue-at-10 bug) |
| B6 | Cancellation | yes | yes | yes | 1 | Through `ctx` on every transport, tested for leaks |
| B7 | Progress | yes | yes | yes | 1 | Ordering guarantee before the response on every transport, in-memory included |
| B8 | Per-request log level | yes | yes | yes | 1 | No log without an explicit level, as the spec says |
| B9 | Caching hints `ttlMs`, `cacheScope` | yes | yes | yes | 1 | Server-wide defaults with per-result override |
| B10 | Pagination | yes | yes | yes | 1 | Cursors stay valid while tools change (mark3labs #1214) |
| B11 | Standard headers `Mcp-Method`, `Mcp-Name`, `Mcp-Param-*` | yes | yes | yes | 1 | Checked on both sides, base64 values fuzzed |
| B12 | Error codes -32020, -32021, -32022 with data | yes | yes | yes | 1 | Constructors carry the spec's `data` shape |
| B13 | Custom JSON-RPC methods and notifications | unverified | yes | yes | 1 | `srv.HandleMethod` and `conn.Request` are public from day one (go-sdk #745) |
| B14 | Large-message performance | partial | partial | yes | 1 | One envelope decode and one `_meta` decode per message, benchmarked in CI (mark3labs perf draft, go-sdk #1531) |

### Server features

| ID | Feature | mark3labs | go-sdk | tether | Phase | Where tether aims to be better |
|---|---|---|---|---|---|---|
| S1 | Typed tools from Go types | yes | yes | yes | 1 | Generic `AddTool[In, Out]`, stdlib schema inference |
| S2 | Raw schema passthrough | yes | yes | yes | 1 | Byte-for-byte, tested |
| S3 | Structured output and output schema | yes | yes | yes | 1 | Inferred from `Out` |
| S4 | Input validation | yes | yes | yes | 1 | Exact decimal numbers (go-sdk #1310), case-sensitive names (mark3labs #1165) |
| S5 | Full JSON Schema 2020-12 validation | yes | yes | module | 5 | Opt-in module, core stays dependency-free |
| S6 | Resources and templates | yes | yes | yes | 1 | |
| S7 | Prompts | yes | yes | yes | 1 | |
| S8 | Completion | yes | yes | yes | 1 | |
| S9 | Logging to client (deprecated) | yes | yes | yes | 1 | |
| S10 | Registration while serving | race bugs | unverified | yes | 1 | Copy-on-write snapshots (mark3labs #1303, #1305) |
| S11 | Per-user tool sets and tool filters | yes | unverified | yes | 5 | Filter on the request principal from auth middleware, no session table |
| S12 | Hooks around requests | yes | middleware | middleware | 1 | One mechanism, `func(Handler) Handler` |
| S13 | Panic recovery | yes | unverified | yes | 1 | Always on, stack never sent to the client |
| S14 | Tool result size limit | yes | unverified | yes | 5 | Middleware |
| S15 | Concurrency limits, worker pool | yes | unverified | yes | 1 (stdio), 5 (HTTP middleware) | |

### Client features

| ID | Feature | mark3labs | go-sdk | tether | Phase | Where tether aims to be better |
|---|---|---|---|---|---|---|
| C1 | Elicitation form and URL | yes | yes | yes | 1 | Typed `Form[T]` and `Answer[T]` |
| C2 | Sampling (deprecated) | yes | yes | yes | 1 | |
| C3 | Roots (deprecated) | yes | yes | yes | 1 | |
| C4 | Automatic MRTR retry loop | yes | yes | yes | 1 | Round limit, `requestState` carried, clear error when no handler (mark3labs #1286) |
| C5 | Typed calls and pagination iterators | yes | yes | yes | 1 | `Call[Out]`, `iter.Seq2` |
| C6 | Schema passthrough to models | needs re-marshal | yes | yes | 1 | `json.RawMessage` fields (mark3labs tool-schema draft) |

### Transports

| ID | Feature | mark3labs | go-sdk | tether | Phase | Where tether aims to be better |
|---|---|---|---|---|---|---|
| T1 | stdio server | yes | yes | yes | 1 | Final line without newline, drain at EOF (go-sdk #1061) |
| T2 | stdio client and subprocess | yes | yes | yes | 1 | Close stdin, wait, kill, with leak tests |
| T3 | Streamable HTTP server | yes | yes | yes | 1 | Body limit (mark3labs #1107), Origin check by default, JSON or SSE chosen per response |
| T4 | Streamable HTTP client | yes | yes | yes | 1 | Fuzzed SSE reader (mark3labs #1267, #1186), deadlines kept (go-sdk #1189) |
| T5 | In-memory transport | yes, ordering bug | yes | yes | 1 | Real JSON encoding and the same ordering rule |
| T6 | Custom transports | yes | yes | yes | 1 | One small interface over public `jsonrpc` |
| T7 | HTTP+SSE transport (deprecated) | yes | yes | adapter | 2 | In `legacy` only |
| T8 | Resumability, event store (removed in 2026-07-28) | yes | yes | adapter | 2 | In `legacy` only |
| T9 | CORS helpers | yes | unverified | yes | 5 | Middleware |

### Authorization

| ID | Feature | mark3labs | go-sdk | tether | Phase | Where tether aims to be better |
|---|---|---|---|---|---|---|
| A1 | Protected resource metadata (RFC 9728) | yes | yes | yes | 2 | Lenient resource matching per RFC (go-sdk #1538) |
| A2 | Bearer token middleware with audience checks | unverified | yes | yes | 2 | |
| A3 | `WWW-Authenticate`, 403 `insufficient_scope`, step-up | unverified | partial | yes | 2 | Request-time scope challenges (go-sdk #1248) |
| A4 | Client OAuth 2.1 with PKCE | yes | experimental | yes | 2 | Conformance auth suite in CI |
| A5 | Client ID Metadata Documents | unverified | unverified | yes | 2 | Preferred path, as the spec asks |
| A6 | Dynamic client registration (deprecated) | unverified | yes | yes | 2 | Sends scopes (go-sdk #1102) |
| A7 | RFC 8707 `resource`, RFC 9207 `iss` checks | yes | unverified | yes | 2 | |
| A8 | Token refresh before expiry | partial | unverified | yes | 2 | mark3labs #1122 |
| A9 | Client credentials, enterprise-managed authorization, token exchange | unverified | yes | yes | 5 | |
| A10 | DPoP | unverified | unverified | yes | 5 | Conformance has scenarios |

### Extensions

| ID | Feature | mark3labs | go-sdk | tether | Phase | Where tether aims to be better |
|---|---|---|---|---|---|---|
| E1 | Tasks (`io.modelcontextprotocol/tasks`) | yes | no (#626) | yes | 5 | Built on the stateless core with a pluggable task store |
| E2 | Skills over MCP | unverified | unverified | yes | 5 | Conformance has scenarios |
| E3 | MCP Apps (UI) | unverified | unverified | yes | 5 | |
| E4 | Declaring extensions in capabilities | yes | yes | yes | 1 | |

### Operations, testing and project

| ID | Feature | mark3labs | go-sdk | tether | Phase | Where tether aims to be better |
|---|---|---|---|---|---|---|
| O1 | OpenTelemetry traces and metrics | yes (module) | unverified | yes (module) | 5 | Separate module, trace context through `_meta` |
| O2 | Structured logging (`slog`) | yes | yes | yes | 1 | `WithErrorLog`, `slog` adapter in phase 5 |
| O3 | Rate limiting | no (#896) | example | yes | 5 | Middleware keyed by principal |
| O4 | Test kit | yes (`mcptest`) | in-memory | yes | 1 | Pair, leak check, replay check, golden files |
| O5 | Official conformance suite in CI | yes | yes | yes | 1 | Baseline file reviewed on every change |
| O6 | Interop with reference servers and clients | unverified | unverified | yes | 3 | Published matrix in `docs/interop.md` |
| O7 | Fuzz tests | unverified | unverified | yes | 1 | JSON-RPC, SSE, headers |
| O8 | Third-party dependencies in the core | 7 direct | several | 0 | 1 | |
| O9 | Drop-in from mark3labs | n/a | n/a | yes | 3 | See Compatibility |
| O10 | Drop-in from go-sdk | n/a | n/a | yes | 4 | See Compatibility |

### Where "better" is not proven yet

Some things cannot be claimed on day one. Each item says what would earn the claim.

- **Maturity.** Both competitors have years of production use and thousands of dependents. tether has none. Earn it by publishing the conformance pass rate and interop matrix on every release, then real adopters.
- **Official status.** go-sdk is the official SDK and appears on the SDK tiers page. Earn it by passing the full conformance suite and applying for a tier listing at v1.0.
- **Breadth of older revisions.** Both support every revision in their core. tether supports them through `legacy` from phase 2. Earn parity with the upstream test suites, run against `legacy` (phase 3).
- **Performance.** The single-decode design should be faster on large messages, but that is a claim until it is measured. Earn it with the phase 1 benchmarks run side by side with both competitors and published in `docs/benchmarks.md` (phase 3).
- **Contributors.** One maintainer today. Earn it with labeled issues for every open roadmap row and fast reviews.
- **"unverified" cells.** Each one gets checked against the competitor's source before phase 3. The matrix is corrected wherever we were wrong.

## Compatibility and migration

Projects on `mark3labs/mcp-go` or `modelcontextprotocol/go-sdk` move by changing import paths. Code written against tether's own API is the long-term path, and the shims are the bridge.

### Shim modules

| Module | Mirrors | Phase |
|---|---|---|
| `github.com/raven-clown/tether/compat/mark3labs` | `mcp`, `server`, `client`, `client/transport`, `mcptest` of `github.com/mark3labs/mcp-go` | 3 |
| `github.com/raven-clown/tether/compat/gosdk` | `mcp`, `jsonrpc`, `auth`, `oauthex` of `github.com/modelcontextprotocol/go-sdk` | 4 |

- Each shim keeps the upstream package names, exported identifiers and signatures, and runs on tether's core. The session APIs both libraries expose are emulated through `legacy`, so a shim depends on `legacy`.
- Each shim pins the upstream version it mirrors. `internal/cmd/apisurface` lists every exported symbol of that upstream version and of the shim. CI fails when the shim drops a symbol it covered before. The coverage percentage goes in `docs/compat-status.md`.
- Proof of drop-in: CI copies the upstream repositories' own `examples/` at the pinned version, rewrites their imports to the shim, builds them, and runs the ones that have tests.

### What is drop-in and what is not

`docs/compat-status.md` holds the live list, generated from `apisurface` and the example builds. At the start of phase 3 it looks like this:

| Area | Drop-in | Not yet, with the plan |
|---|---|---|
| Tool, resource and prompt registration, typed tools, results | yes | |
| stdio and Streamable HTTP servers and clients | yes | |
| Sampling, elicitation, roots, progress, logging | yes | |
| OAuth client and server options | yes after phase 2 | |
| Session hooks, session-specific tools, session ID managers | emulated through `legacy` | behavior on 2026-07-28 requests differs because sessions do not exist there, and the guide says how |
| SSE server and event stores | through `legacy` | |
| Package internals, unexported behavior, exact error strings | no | not part of the contract |
| Upstream symbols added after the pinned version | no | the shim is bumped each tether minor release |

### Migration tool

`cmd/tether-migrate` uses `go/ast` and `go/format` from the standard library:

- `tether-migrate ./...` rewrites upstream imports to the shim paths and prints every use of a symbol the shim does not cover, with file and line.
- `tether-migrate -native ./...` (phase 5) rewrites the common patterns to tether's own API and leaves a `// tether-migrate:` note where a person has to decide.
- `-dry-run` prints a diff instead of writing.

### Migration guides

`docs/migrate/mark3labs.md` and `docs/migrate/go-sdk.md` show the same server and client side by side in the upstream API, the shim and native tether, and list every behavior difference.

### Interop tests

A CI job `interop` (phase 3) runs:

- tether's client against the reference `everything` server and the TypeScript and Python SDK example servers
- the TypeScript and Python SDK clients against tether's conformance server
- the MCP Inspector CLI against tether's server

The results table is published in `docs/interop.md` on every release.

## Module and packages

Module path `github.com/raven-clown/tether`. Supported Go versions are the two latest releases (1.26 and 1.27 today).

| Package | Purpose |
|---|---|
| `tether` | `Server`, `Client`, `Conn`, `Handler`, `Middleware`, `Request`, registration of tools, resources, prompts and completions, typed tool helpers, `Error` |
| `protocol` | Wire types for 2026-07-28: requests, results, content blocks, capabilities, `_meta` keys, error codes |
| `jsonrpc` | JSON-RPC 2.0 messages, decoding and encoding. Public so that third-party transports can implement `Transport` |
| `jsonschema` | Schema inference from Go types and struct tags, the `Validator` interface, a basic validator |
| `transport/stdio` | stdio server (`Serve`) and client (`Command`, `Pipe`) |
| `transport/streamable` | Streamable HTTP server (`Handler`) and client (`Dial`) |
| `tethertest` | In-memory client and server pair, goroutine leak check, golden helpers, MRTR replay check |
| `conformance/server`, `conformance/client` | Programs driven by the official conformance suite |
| `examples/...` | Small complete programs |
| `middleware` | Phase 5: rate limit, CORS, result size limit, tool filter, concurrency limit, `slog` logging |
| `cmd/tether-migrate` | Phase 3: import rewriter and report for projects moving from other libraries |

Phase N ships as v0.N. Separate modules, each with its own `go.mod`, so the core never imports them:

| Module | Phase | Purpose |
|---|---|---|
| `github.com/raven-clown/tether/auth` | 2 | OAuth 2.1 for servers and clients |
| `github.com/raven-clown/tether/legacy` | 2 | Revisions 2025-11-25 and older: handshake, sessions, HTTP+SSE, resumability |
| `github.com/raven-clown/tether/compat/mark3labs` | 3 | API of `mark3labs/mcp-go` on tether |
| `github.com/raven-clown/tether/compat/gosdk` | 4 | API of `modelcontextprotocol/go-sdk` on tether |
| `github.com/raven-clown/tether/ext/tasks` | 5 | `io.modelcontextprotocol/tasks` extension |
| `github.com/raven-clown/tether/ext/skills` | 5 | Skills over MCP |
| `github.com/raven-clown/tether/ext/apps` | 5 | MCP Apps (UI) |
| `github.com/raven-clown/tether/jsonschema/full` | 5 | Full JSON Schema 2020-12 validator behind the `Validator` interface |
| `github.com/raven-clown/tether/otel` | 5 | OpenTelemetry traces and metrics middleware |

Dependency rules:

- `protocol` imports only the standard library.
- `jsonrpc` imports only the standard library.
- `tether` imports `protocol`, `jsonrpc` and `jsonschema`.
- Transports import `tether` and never the other way around.
- Library packages never print and never touch `os.Stdout` or `os.Stderr`. The stdio server writes only to the writer it is given.

## Server

### Handler and middleware

```go
type Handler interface {
	Handle(ctx context.Context, req *Request) (protocol.Result, error)
}

type HandlerFunc func(ctx context.Context, req *Request) (protocol.Result, error)

type Middleware func(Handler) Handler
```

`*Server` implements `Handler`. `srv.Use(mw...)` wraps the method dispatch, so middleware sees every request after version and capability checks and before the method handler. Logging, rate limits, tracing and auth are all middleware. tether ships none in the core except panic recovery, which is always on.

### Request

`*tether.Request` is built once per message by the transport:

- `Method string`
- `Params json.RawMessage`, decoded lazily by the method handler into its own type
- `Meta`: protocol version, client capabilities, client info, log level, progress token, input responses, plus the raw `_meta` object for extension keys
- `Header http.Header`, set by HTTP transports and nil on stdio

The envelope is decoded once. `_meta` is decoded once and shared. Params stay raw until a handler asks for them, so a 1 MiB `tools/call` is parsed one time on its way to the tool.

### Dispatch

For each request the server:

1. Rejects an unsupported protocol version with -32022 (`UnsupportedProtocolVersion`).
2. Takes a snapshot of the registry.
3. Checks client capabilities required by the method and returns -32021 (`MissingRequiredClientCapability`) when one is missing.
4. Runs middleware, then the method handler.
5. Sets `resultType` on the result (`complete` unless the handler asked for input), and `ttlMs` and `cacheScope` on list and read results from server options.

Unknown methods return -32601. `srv.HandleMethod(name, h)` registers a handler for a method the spec does not define, such as an extension method, and goes through the same checks and middleware. On the client, `conn.Request(ctx, method, params, &out)` and `conn.Notify(ctx, method, params)` send custom messages.

### Registry

Tools, resources, resource templates, prompts and completion handlers live in a registry that is copied on write. Registration takes a lock, builds a new snapshot and swaps it in atomically. A request uses the snapshot it started with from dispatch to result. Registering or removing a tool while requests run is safe and never seen half done.

Lists are sorted by name. Pagination cursors are opaque, encode the last name returned, and stay valid when items are added or removed between pages.

### Tools

```go
func AddTool[In, Out any](s *Server, name, description string, fn ToolFunc[In, Out], opts ...ToolOption)
type ToolFunc[In, Out any] func(ctx context.Context, req *Request, in In) (Out, error)

func AddRawTool(s *Server, tool protocol.Tool, fn RawToolFunc)
type RawToolFunc func(ctx context.Context, req *Request, args json.RawMessage) (*protocol.CallToolResult, error)
```

- `AddTool` infers `inputSchema` from `In` and `outputSchema` from `Out` through `jsonschema`. Struct tags `json`, `jsonschema:"description=...,enum=a|b,minimum=0"` and Go types decide the schema.
- When `Out` is a struct, the result carries it in `structuredContent` and a JSON text copy in `content`. When `Out` is `*protocol.CallToolResult`, it is returned as is.
- Arguments are validated against the input schema before `fn` runs. A validation failure is a tool error result, not a protocol error.
- `AddRawTool` takes a schema as `json.RawMessage` and passes it through byte for byte. Gateways and proxies use it.
- Tool annotations, including `x-mcp-header` parameters, are set through `ToolOption`.

### Errors

- A plain `error` returned from a tool becomes a `CallToolResult` with `isError: true` and the error text, as the spec asks for tool execution errors.
- A `*tether.Error` (code, message, data) becomes a JSON-RPC error. Constructors exist for every code in the spec: `ErrInvalidParams`, `ErrResourceNotFound` (-32602 in 2026-07-28), `ErrHeaderMismatch` (-32020) and the rest.
- A panic in a handler is recovered and returned as -32603 with a generic message. The panic value and stack go to an optional `ErrorLog` callback, never to the client.

### Multi round-trip requests (MRTR)

2026-07-28 replaces server-to-client requests with `InputRequiredResult`. tether lets a handler write this as a plain call:

```go
answer, err := tether.Elicit(ctx, req, "confirm", tether.Form[Confirm]{Message: "Delete 3 files?"})
if err != nil {
	return out, err
}
```

`Elicit` is a function and not a method because Go methods cannot take type parameters.

- If the request's `inputResponses` holds the key `confirm`, `Elicit` decodes it into `Confirm` and returns it.
- If not, `Elicit` records an input request under that key and returns `tether.ErrInputRequired`. The server collects every request recorded during the call and returns one `InputRequiredResult` with all of them.
- `req.Sample` and `req.Roots` work the same way. Both features are deprecated in the spec and stay supported until it removes them.
- The spec allows input requests only on `tools/call`, `prompts/get` and `resources/read`. Calling `Elicit`, `Sample` or `Roots` while serving any other method returns an error.
- `req.State()` and `req.SetState(s)` read and set the opaque `requestState` string, so a handler can carry its own state between rounds instead of recomputing it.
- An input request needs the matching client capability (`elicitation`, `sampling` or `roots`) in the request's `_meta`. Without it, `Elicit` returns -32021 instead of asking.

A handler that needs several answers calls `Elicit` for each before checking errors and returns `errors.Join` of them, so all requests go out in one round.

Because the client retries the whole request, a handler runs from the top on each round. Side effects must come after the last `Elicit`. The docs say so on `Elicit`, and `tethertest.CheckReplaySafe` runs a handler through every round and fails when a side effect hook fires more than once.

### Progress, logging, cancellation

- `req.Progress(done, total, message)` sends `notifications/progress` when the request carried a progress token and does nothing otherwise.
- `req.Log(level, data)` sends `notifications/message` when the level is at or above the request's `io.modelcontextprotocol/logLevel`. A request without a log level gets no log messages at all, as the spec requires.
- Both go through a `Notifier` the transport puts on the request. Every transport delivers a request's notifications before its response.
- Cancellation arrives as `ctx` cancellation: `notifications/cancelled` on stdio, a closed response stream on HTTP.

### Subscriptions

`subscriptions/listen` is a long-lived request. A `broker` inside the server holds the listeners and is the only stateful part of the server.

- `srv.NotifyToolsChanged()`, `NotifyPromptsChanged()`, `NotifyResourcesChanged()` and `NotifyResourceUpdated(uri)` publish to the broker. Registry changes publish list-changed events on their own.
- A listen request first gets `notifications/subscriptions/acknowledged` with the subset of the filter the server supports, then only the notification types the client asked for. Every notification on the stream carries `io.modelcontextprotocol/subscriptionId` set to the listen request's ID.
- When the server ends a stream gracefully it sends the `subscriptions/listen` result with the same `subscriptionId` in `_meta`. On stdio it also sends `notifications/cancelled` with that ID, the one case where a server sends a cancellation.
- Each listener has a bounded buffer (default 64). A listener that falls behind is closed with an error and the client is expected to listen again. Publishing never blocks and never fails a request.
- Listeners end when their context ends. `srv.Close()` ends all of them.

### Discovery

`server/discover` is answered from the registry and server info: capabilities, extensions, supported protocol versions. Nothing to register by hand.

## Client

```go
c := tether.NewClient(tether.Info{Name: "agent", Version: "0.1.0"}, tether.ClientOptions{
	OnElicit: askUser,
})
conn, err := c.Connect(ctx, stdio.Command("weather-server"))
out, err := tether.Call[ForecastOut](ctx, conn, "forecast", ForecastIn{City: "Bangkok"})
for ev, err := range conn.Listen(ctx, tether.ListenTools|tether.ListenResources) {
	...
}
```

- `Connect` does no handshake. It returns a `*Conn` bound to a transport. `conn.Discover(ctx)` is optional and caches the result.
- Every request carries `_meta` with protocol version, client capabilities and client info.
- Typed helpers: `tether.Call[Out]`, `conn.ListTools`, `conn.ReadResource`, `conn.GetPrompt`, `conn.Complete`, plus iterators that walk pagination (`conn.Tools(ctx)` returns `iter.Seq2[protocol.Tool, error]`).
- **MRTR loop.** On `input_required` the client calls `OnElicit`, `OnSample` or `OnRoots` for each input request, then retries with `inputResponses` and the `requestState` it was given. It stops after `MaxInputRounds` (default 8). MRTR has no way to answer an input request with an error, so an input request the client has no handler for fails the call with a method-not-found `*tether.Error`. The client advertises a capability only when its handler is set, so a well-behaved server never asks.
- `protocol.Tool` keeps `inputSchema` and `outputSchema` as `json.RawMessage`, exactly as received, for clients that hand them to a model.
- For Streamable HTTP, `tools/call` needs `Mcp-Param-*` headers for arguments the tool marks with `x-mcp-header`. When the transport says it needs them, the connection fetches the tool list once, caches it until `ttlMs` runs out or a tools list-changed event arrives, and passes the headers to the transport through the context.
- `tether.WithProgress(fn)` on a call receives that call's progress notifications.

### Transport interface

```go
type Transport interface {
	RoundTrip(ctx context.Context, msg *jsonrpc.Message, notify func(*jsonrpc.Message)) (*jsonrpc.Message, error)
	Notify(ctx context.Context, msg *jsonrpc.Message) error
	Close() error
}
```

One request, one call. `notify` receives notifications tied to that request, in order, before `RoundTrip` returns. `Close` returns only after every goroutine the transport started has exited.

## Transports

### stdio

Server: `stdio.Serve(ctx, h tether.Handler, r io.Reader, w io.Writer, opts...)`.

- Newline-delimited JSON-RPC with a maximum line size (default 16 MiB).
- A final line without a trailing newline is still read.
- On EOF the server stops reading, waits for in-flight requests to write their responses, then returns.
- Requests run concurrently up to a limit (default 16). Writes go through one writer goroutine, and a request's notifications are queued ahead of its response.

Client: `stdio.Command(name, args...)` starts a process. `stdio.Pipe(r, w)` wraps existing streams.

- Responses are matched by ID. Progress notifications are matched by the progress token the client set, and listen notifications by `subscriptionId`. stdio has no other way to tie a notification to a request, so anything else (log messages, for example) goes to an optional `WithUnrouted` callback.
- Shutdown closes stdin, waits up to a timeout (default 5s), then kills the process.
- stderr goes to an optional callback and is never treated as an error.

### Streamable HTTP

Server: `streamable.Handler(h tether.Handler, opts...)` returns an `http.Handler`.

- POST only. GET and DELETE return 405. `Mcp-Session-Id` and `Last-Event-ID` are ignored.
- `Origin` is checked against an allowlist (default: localhost, 127.0.0.1, [::1], and requests without `Origin`). A mismatch returns 403.
- `MCP-Protocol-Version`, `Mcp-Method`, `Mcp-Name` and `Mcp-Param-*` are checked against the body. A mismatch returns 400 with -32020. Base64 encoded values (`=?base64?...?=`) are decoded first.
- Request bodies are limited (default 4 MiB). Larger bodies get 413.
- An unsupported protocol version returns 400 and an unknown method returns 404, each with the JSON-RPC error in the body.
- A notification gets 202. A request gets `application/json` when the handler sent no notifications, and switches to `text/event-stream` on the first notification, so notifications go out before the result.
- The handler's `ctx` ends when the client closes the stream.

Client: `streamable.Dial(url, opts...)` returns a `Transport`.

- Sends `Accept: application/json, text/event-stream` and all required MCP headers on every request, base64 encoding non-ASCII values.
- Takes a `Doer` interface (`Do(*http.Request) (*http.Response, error)`), so callers bring their own auth, retries and tracing. The default is `http.DefaultClient`.
- The SSE reader handles a UTF-8 BOM and CR, LF and CRLF line endings, and is fuzzed.
- Respects context deadlines on connect, on headers and on each event.

### In-memory

`tethertest.NewPair(h)` returns a client `Conn` wired to a handler through real JSON encoding and the same ordering rule as the other transports. Tests of servers and clients use it.

## Testing

- Tests come with every change and are written first. Table-driven where there is a table.
- No network in unit tests (`httptest` is fine), no bare `time.Sleep`, every test passes on Windows.
- `tethertest.CheckNoLeaks(t)` runs in every transport test and fails on goroutines left behind.
- Golden wire tests: example messages from the spec live in `testdata/spec/` and are checked for round trips. A test compares the fields of `protocol` types with the official `schema.json`, which is vendored in `testdata/`.
- Fuzz tests for the JSON-RPC decoder, the SSE reader and header decoding.
- Benchmarks for a 1 MiB `tools/call` through each transport.
- The conformance job runs the official suite (`npx @modelcontextprotocol/conformance`) against `conformance/server` and `conformance/client`. Known failures go in `conformance-baseline.yml`. For v0.1 the baseline may hold only scenarios for revisions before 2026-07-28 (they belong to `legacy`), auth (v0.2) and extensions such as tasks and skills (v0.3), each with a comment saying which.
- Coverage stays at 80% or more.

## Project and contributor setup

- License Apache-2.0 with a `NOTICE` file. No CLA and no DCO sign-off; Apache-2.0 section 5 covers contributions.
- `AGENTS.md`: commands, layout and rules for anyone working in the repository, kept current in the same commit as any change to them.
- `CONTRIBUTING.md`: setup, running tests and the conformance suite locally, how to pick an issue, branch names, Conventional Commits, review expectations.
- `CODE_OF_CONDUCT.md` (Contributor Covenant 2.1) and `SECURITY.md` (GitHub private vulnerability reporting).
- Issue forms: bug report, feature request, spec gap (names the spec section), plus `config.yml` that turns off blank issues and points questions to Discussions. A pull request template with a checklist.
- `CODEOWNERS` naming `@raven-clown` for everything, so every PR asks for review.
- Labels:
  - kind: `bug`, `enhancement`, `docs`, `breaking`, `spec`
  - help: `good first issue`, `help wanted`
  - area: `area/server`, `area/client`, `area/stdio`, `area/http`, `area/schema`, `area/auth`, `area/legacy`, `area/compat`, `area/ext`, `area/ci`
  - priority: `priority/p0` (breaks users), `priority/p1` (next release), `priority/p2` (planned), `priority/p3` (nice to have)
- Milestones `v0.1` to `v0.5` and `v1.0`, one per phase.
- A GitHub Project "tether roadmap" with one issue per plan task and per roadmap row, fields Phase, Area and Priority, and board columns Todo, In progress, In review and Done.
- Repository settings:
  - Discussions on
  - private vulnerability reporting on
  - approval required before workflows run on pull requests from all outside contributors
  - squash merge only
  - delete branches after merge
- Ruleset on `main`:
  - pull request required
  - required status checks `test`, `lint`, `vuln`, `coverage` and `conformance`
  - conversations resolved before merge
  - linear history
  - no force push and no deletion
  - the maintainer may bypass only for the release tag
- `docs/spec-coverage.md`: one row per spec feature with status and issue link.
- `CHANGELOG.md` in Keep a Changelog form. Releases are tags `vX.Y.Z` on main with notes from the changelog.

### CI

GitHub Actions on every pull request and on main:

- `test`: ubuntu, macos and windows with Go 1.26 and 1.27, `go test -race ./...`
- `lint`: golangci-lint
- `vuln`: govulncheck
- `conformance`: the official suite against both programs
- `coverage`: fails under 80%
- `interop` (phase 3): reference servers and clients, see Compatibility
- `compat` (phase 3): `apisurface` check and upstream example builds against each shim
- Dependabot for GitHub Actions and for the Go modules outside the core

## Roadmap

`docs/plans/roadmap.md` maps every matrix row to a phase and outlines the tasks for each phase. In short:

- **Phase 1 (v0.1)**: the core, both transports, server and client, test kit. Every 2026-07-28 core conformance scenario passes.
- **Phase 2 (v0.2)**: `auth` and `legacy`. Auth conformance suite passes, older revisions work.
- **Phase 3 (v0.3)**: `compat/mark3labs`, `tether-migrate`, migration guide, interop job, side-by-side benchmarks.
- **Phase 4 (v0.4)**: `compat/gosdk` and its guide.
- **Phase 5 (v0.5)**: extensions (tasks, skills, apps), `jsonschema/full`, `otel`, `middleware`, remaining auth extensions, `tether-migrate -native`.
- **v1.0**: API freeze, full conformance suite, tier listing application.

## Risks

- **The spec moves.** 2026-07-28 is recent and extensions are still settling. `protocol` is the only place wire shapes live, and the schema comparison test flags drift when the vendored `schema.json` is updated.
- **MRTR replay surprises handler authors.** Covered by docs, an example, and `tethertest.CheckReplaySafe`.
- **Few hosts may speak 2026-07-28 yet.** The `legacy` module in v0.2 covers older clients. Until then tether servers reach only up-to-date clients, which the README states.
- **One maintainer at the start.** The contributor setup, the coverage table and labeled issues exist so that others can pick up work without a meeting.
- **Scope.** Covering every row and two shims is years of work for one person. Phases ship on their own, each is useful without the next, and the matrix shows what is not done yet instead of hiding it.
- **Shims chase moving targets.** Both upstream libraries release often. Shims pin a version, `apisurface` shows the gap after each upstream release, and bumping the pin is a labeled `good first issue` style task.
