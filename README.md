# mcp-go-next

A Go library for the [Model Context Protocol](https://modelcontextprotocol.io) (MCP), built for revision 2026-07-28 from the ground up.

> **Status: design in progress.** There is no code yet. The design and the first implementation plan are open for comments. Nothing below exists until it is checked off in `docs/spec-coverage.md`, which comes with the first release.

## Why

The design starts from what people building MCP servers in Go ran into with today's libraries: issue trackers, discussions, and the pull requests of the largest Go MCP projects. The evidence is in [docs/design.md](docs/design.md). The goals that came out of it:

- **Servers with many tools stay fast and small.** Typed tools from Go structs, toolsets chosen per request, and a `tools/list` that is encoded once and served from cache.
- **No sessions to scale.** Each request carries who is calling, and state across calls lives in owned, expiring handles with a pluggable store.
- **A client that keeps working.** Transient errors fail one call, not the connection. Safe calls retry. Deadlines hold.
- **Plain `http.Handler`.** Mount it in net/http, Gin, chi or echo. CORS for browser clients is built in.
- **Standard library only** in the core module.
- **Tested against the spec.** The official conformance suite will run in CI.

## Planned scope

- Server and client
- stdio and Streamable HTTP transports
- Tools, toolsets, resources, prompts, completion, discovery, subscriptions, input requests, progress, cancellation
- OAuth 2.1 for servers and clients
- An adapter for clients on revision 2025-11-25 and earlier
- A gateway package for aggregating upstream servers
- Extensions such as tasks, skills and apps

See [docs/design.md](docs/design.md) for the design, [docs/plans](docs/plans) for the roadmap and implementation plans, and [docs/benchmarks.md](docs/benchmarks.md) for measurements against mark3labs/mcp-go and the official go-sdk.

## Contributing

Anyone can take part. Comments on the design are welcome now through issues and [discussions](https://github.com/raven-clown/mcp-go-next/discussions). See [CONTRIBUTING.md](CONTRIBUTING.md) for how to set up and send a pull request, and the [roadmap board](https://github.com/users/raven-clown/projects/9) for work you can pick up.

## License

[Apache-2.0](LICENSE)
