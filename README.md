# tether

A Go library for the [Model Context Protocol](https://modelcontextprotocol.io) (MCP), built for revision 2026-07-28 from the ground up.

> **Status: design in progress.** There is no code yet. The design and the first implementation plan are open for comments. Nothing below exists until it is checked off in [docs/spec-coverage.md](docs/spec-coverage.md) (added with the first release).

## Why

MCP 2026-07-28 dropped the initialize handshake and sessions. Every request now carries its own protocol version and client capabilities, so an MCP server can be a plain function from a request to a result. tether is being designed around that model, with these goals:

- **Stateless core.** No sessions and no background goroutines in the server. Easy to test, easy to run behind a load balancer.
- **Small API in the shape of `net/http`.** A `Handler`, middleware, and typed tools through generics.
- **Standard library only** in the core module.
- **Tested against the spec.** The official conformance suite runs in CI, with interop tests against the reference servers and clients.

## Moving from another Go library

tether plans to make the move from [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go) or the official [modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) a matter of changing import paths:

- compatibility packages with the same API as each library, running on tether's core
- a `tether-migrate` tool that rewrites imports and lists anything not supported yet
- a migration guide with side-by-side examples

The design lists what will be drop-in and what will not. None of this is built yet.

## Planned scope

- Server and client
- stdio and Streamable HTTP transports
- Tools, resources, prompts, completion, discovery, subscriptions, multi round-trip input requests, progress, cancellation
- OAuth 2.1 for servers and clients
- An adapter for clients on revision 2025-11-25 and earlier
- Extensions such as tasks

See [docs/design.md](docs/design.md) for the design and [docs/plans](docs/plans) for the implementation plans.

## Contributing

The project is set up so anyone can take part. Comments on the design are welcome now through issues and discussions. A contributor guide comes with the first code.

## License

[Apache-2.0](LICENSE)
