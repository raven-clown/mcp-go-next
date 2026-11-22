# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html). Versions are `v0.<phase>.<n>`: every release in phase 1 of the roadmap is `v0.1.x`.

Each line ends with the pull requests that made the change, such as `(#12)` or `(#12, #15)`. The release notes link them and credit their authors. A pre-release tag such as `v0.2.0-rc.1` needs its own section, `## [0.2.0-rc.1]`.

## [Unreleased]

### Changed

- `jsonrpc.Decode` reads the envelope in one pass and copies only the fields it keeps. A small `tools/call` decodes in under 1 µs with 10 allocations, about half the time it took before. A 1 MiB request now decodes as fast as mark3labs/mcp-go, where it was about 35% slower. The numbers are in `docs/benchmarks.md`. (#75)

### Added

- `docs/benchmarks.md` compares mcp-go-next with mark3labs/mcp-go and the official go-sdk on the same inputs. It covers decoding, validating, compiling and inferring schemas, and lists where mcp-go-next does not win. The comparison code is in `bench/`, a separate module, and runs with `make bench-compare`. (#75)

## [0.1.0] - 2026-11-22

The first building blocks. There is no server or client yet: this release has the wire format, the protocol types and JSON Schema support that the server and client are built on.

### Added

- `jsonrpc` package: decodes and encodes JSON-RPC 2.0 messages, one per frame.
  - Request IDs keep the exact bytes the peer sent, so `1.0` and `9007199254740993` come back unchanged.
  - Batches, trailing data, keys in the wrong case and malformed error objects are refused, with the error code to send back.
  - `Encode` refuses any message that `Decode` would reject.
  - (#67, #70)
- `protocol` package: the base wire types of MCP 2026-07-28.
  - Capabilities, request `_meta`, results, cache fields and log levels.
  - Checked field by field against the official `schema.json`, which is vendored with all 131 official examples.
  - (#68)
- `protocol` feature types: tools, resources, prompts, completion, discovery, subscriptions, multi round-trip input requests, sampling, elicitation, roots and notifications.
  - Every official example round-trips. One example breaks the schema by leaving out `ttlMs` and `cacheScope`; the test notes it, and the types still follow the schema.
  - Required arrays are never written as `null`.
  - Tool blocks may only appear in sampling, so content cannot nest without limit.
  - Tokens and request IDs accept only strings and numbers.
  - (#69)
- `jsonschema` package, part 1: builds a JSON Schema from Go types.
  - It follows `encoding/json`: tags, omitempty, embedded structs and `,string`.
  - Pointers, slices and maps accept `null`, so what Go writes for ordinary types passes its own schema.
  - Recursive types use `$defs`.
  - `jsonschema` tags set descriptions, enums, bounds and patterns.
  - A type can supply its own schema with no reflection.
  - (#71)
- `jsonschema` package, part 2: a bounded validator for tool arguments.
  - Numbers are compared exactly.
  - Depth, steps, number length, error count and error size are all capped.
  - Only local `$ref` is allowed, so nothing is fetched from the network.
  - Typical tool arguments validate in about 3 µs.
  - (#71, #72)

[Unreleased]: https://github.com/raven-clown/mcp-go-next/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/raven-clown/mcp-go-next/releases/tag/v0.1.0
