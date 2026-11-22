# AGENTS.md

Instructions for working in this repository. Keep this file current: when a command, the layout or a rule changes, update it in the same commit.

## Project

mcp-go-next is a Go library for the Model Context Protocol (MCP), revision 2026-07-28, with no older revision in its core. Servers and clients are both first class, and the core module depends only on the standard library. The design starts from what users of the existing Go MCP libraries ran into. It is in `docs/design.md`, the phased roadmap is in `docs/plans/roadmap.md`, and the task-by-task plan for v0.1 is in `docs/plans/2026-11-22-v0.1.md`.

## Commands

```shell
go build ./...
go test ./...
golangci-lint run ./...
make race       # go test -race ./...
make check      # lint plus coverage of 80% or more
```

On Windows without a C compiler, run `go test ./...` locally. CI runs `-race` on Linux, macOS and Windows.

## Layout

The root package is `mcp`. The packages below arrive task by task through the v0.1 plan.

| Package | Purpose |
|---|---|
| `mcp` (module root) | `Server`, `Client`, `Conn`, `Handler`, `Middleware`, `Request`, `Principal`, toolsets, `Handles`, registration, typed tool helpers, `Error` |
| `protocol` | Wire types for 2026-07-28 |
| `jsonrpc` | JSON-RPC 2.0 messages, public for custom transports |
| `jsonschema` | Schema inference from Go types, `SchemaProvider`, `Validator`, a bounded basic validator |
| `transport/stdio` | stdio server and client |
| `transport/streamable` | Streamable HTTP server (with CORS) and client |
| `mcptest` | In-memory pair, leak check, replay check, golden helpers |
| `conformance/` | Programs driven by the official conformance suite |
| `examples/` | A separate module with tested examples and the cookbook programs |

## Rules

- Tests with every change, written first. Table-driven where there is a table. No network outside `httptest`. No bare `time.Sleep`; poll with a deadline.
- Every test passes on Windows. Build paths with `filepath`.
- No `require` lines in the core `go.mod`, tests included. Third-party code goes in `examples/` or a separate module.
- `protocol` and `jsonrpc` import only the standard library. Transports import `mcp`; `mcp` never imports a transport.
- Library packages never print and never touch `os.Stdout` or `os.Stderr`.
- No `exec` of any program except the test binary itself (helper process pattern) in `transport/stdio` tests.
- Work on a branch named `feat/<topic>`, `fix/<topic>`, `ci/<topic>`, `docs/<topic>` or `chore/<topic>`. Commit with Conventional Commits messages and stage files by name.
- Every pull request gets labels, a milestone and an assignee, and links its issue.
- Every pull request gets a code review before it is merged, even when CI is green. Fix or answer each finding first, and list them in the pull request.
- Merge with a squash: `--subject "<pr title> (#<n>)" --body ""`, so no trailer lands on main.
- Release with a tag `vX.Y.Z` on main, with notes from that version's CHANGELOG section.
- Never mention AI, assistants or code generation in code, comments, docs, commit messages, pull requests or issues. Never add `Co-Authored-By`, `Generated with` or similar trailers.
- No em-dashes. Keep comments short and about why.
- Update `CHANGELOG.md` under Unreleased for anything users will notice.
