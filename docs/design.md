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

## Non-goals

- Revisions before 2026-07-28 in the core. They come through the `legacy` module.
- The deprecated HTTP+SSE transport.
- An HTTP framework, router or config system. tether hands you an `http.Handler`.
- Code generation. Wire types are written by hand and checked against the official schema.

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

Separate modules, each with its own `go.mod`, so the core never imports them:

| Module | Version | Purpose |
|---|---|---|
| `github.com/raven-clown/tether/auth` | v0.2 | OAuth 2.1 for servers and clients |
| `github.com/raven-clown/tether/legacy` | v0.2 | 2025-11-25 handshake and session adapter |
| `github.com/raven-clown/tether/ext/tasks` | v0.3 | `io.modelcontextprotocol/tasks` extension |
| `github.com/raven-clown/tether/jsonschema/full` | v0.3 | Full JSON Schema 2020-12 validator behind the `Validator` interface |

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

Unknown methods return -32601.

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
- Issue templates: bug report, feature request, spec gap (names the spec section). A pull request template with a checklist.
- Labels: `good first issue`, `help wanted`, `bug`, `enhancement`, `spec`, `area/server`, `area/client`, `area/stdio`, `area/http`, `area/schema`, `docs`, `breaking`.
- Milestones `v0.1`, `v0.2`, `v0.3`.
- `docs/spec-coverage.md`: one row per spec feature with status and issue link.
- `CHANGELOG.md` in Keep a Changelog form. Releases are tags `vX.Y.Z` on main with notes from the changelog.

### CI

GitHub Actions on every pull request and on main:

- `test`: ubuntu, macos and windows with Go 1.26 and 1.27, `go test -race ./...`
- `lint`: golangci-lint
- `vuln`: govulncheck
- `conformance`: the official suite against both programs
- `coverage`: fails under 80%
- Dependabot for GitHub Actions

## Roadmap

- **v0.1**: everything above. Every 2026-07-28 core conformance scenario passes.
- **v0.2**: `auth` module. Server side: RFC 9728 protected resource metadata, bearer token middleware with audience checks, `WWW-Authenticate` with `resource_metadata`, 403 `insufficient_scope`. Client side: OAuth 2.1 with PKCE, Client ID Metadata Documents, RFC 8707 `resource`, RFC 9207 `iss` checks, refresh before expiry. `legacy` module for 2025-11-25.
- **v0.3**: tasks extension, `jsonschema/full`.
- **v1.0**: API freeze after real use.

## Risks

- **The spec moves.** 2026-07-28 is recent and extensions are still settling. `protocol` is the only place wire shapes live, and the schema comparison test flags drift when the vendored `schema.json` is updated.
- **MRTR replay surprises handler authors.** Covered by docs, an example, and `tethertest.CheckReplaySafe`.
- **Few hosts may speak 2026-07-28 yet.** The `legacy` module in v0.2 covers older clients. Until then tether servers reach only up-to-date clients, which the README states.
- **One maintainer at the start.** The contributor setup, the coverage table and labeled issues exist so that others can pick up work without a meeting.
