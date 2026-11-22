# Benchmarks

mcp-go-next is measured side by side with [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go) v1.6.0 and [modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) v1.8.0, on the same inputs and machine. Results are published as they come out, including where mcp-go-next loses or ties.

So far this covers the parts that exist in mcp-go-next: decoding JSON-RPC messages, and building and checking JSON Schemas. Whole-server numbers (a tool call through each transport, memory per connected caller, `tools/list` with hundreds of tools) come when the server and transports land in later v0.1 tasks.

## How it is measured

The benchmarks live in `bench/`, a separate module with its own `go.mod`, so the core module keeps zero dependencies. Run them with:

```shell
cd bench
go test -run '^$' -bench . -benchmem -count 8 > new.txt
go run golang.org/x/perf/cmd/benchstat@latest -col /lib new.txt
```

**Which code is measured:**
- Both other libraries build schemas with `github.com/google/jsonschema-go`.
- To validate tool arguments, go-sdk uses `jsonschema-go`, and mark3labs uses `github.com/santhosh-tekuri/jsonschema/v6`.
- Each benchmark starts from the raw bytes a server receives and calls the library code that handles them:

| Benchmark | mcp-go-next | go-sdk | mark3labs |
|---|---|---|---|
| Decode a `tools/call` request | `jsonrpc.Decode` | `jsonrpc.DecodeMessage` | `json.Unmarshal` into `mcp.JSONRPCRequest` (no exported decoder) |
| Validate tool arguments | `Schema.Validate(raw)` | `json.Unmarshal` + `Resolved.Validate` | `jsonschema.UnmarshalJSON` + `Schema.Validate` |
| Compile a schema | `Basic().Compile` | `json.Unmarshal` into `Schema` + `Resolve` | `NewCompiler`, `AddResource`, `Compile` |
| Infer a schema from a struct | `jsonschema.For` | `jsonschema.For` + `json.Marshal` (shared by both) | same as go-sdk |

**Fairness:**
- Only each library's core is timed. Their servers do more work per call. mark3labs, for example, hashes the tool schema with SHA-256 on every call to find its cached validator. These numbers therefore favor the other two libraries.
- `TestSameVerdicts` checks that all three validators accept and reject exactly the same instances, so no library looks faster by checking less.
- The tool schema is written by hand, so every validator checks the same keywords. It covers `type`, `enum`, length and numeric bounds, `items`, `required` and `additionalProperties: false`.

## Results, 2026-11-22

The machine was an Intel Core i7-14700 on Windows 11, with Go 1.27.4, 8 runs per benchmark and `benchstat`. mcp-go-next was at commit `d10dfd6` (v0.1.0) plus the jsonrpc scanner from this change. Raw output: [benchmarks/2026-11-22-raw.txt](benchmarks/2026-11-22-raw.txt).

Each cell shows how the library compares with mcp-go-next. "3.0× slower" means it took three times as long.

### Time per operation

| Benchmark | mcp-go-next | go-sdk | mark3labs |
|---|---|---|---|
| Decode a small `tools/call` (~350 B) | 1.08 µs | 15.7 µs, 14.6× slower | 9.40 µs, 8.7× slower |
| Decode a 1 MiB `tools/call` | 1.67 ms | 4.28 ms, 2.6× slower | 1.93 ms, 1.16× slower |
| Validate typical tool arguments | 4.61 µs | 14.0 µs, 3.0× slower | 13.7 µs, 3.0× slower |
| Validate an array of 5,000 objects | 3.15 ms | 9.64 ms, 3.1× slower | 9.77 ms, 3.1× slower |
| Compile a tool schema | 33.8 µs | 196 µs, 5.8× slower | 203 µs, 6.0× slower |
| Infer a schema from a struct | 18.9 µs | 85.3 µs, 4.5× slower | (same as go-sdk) |

### Memory allocated per operation

| Benchmark | mcp-go-next | go-sdk | mark3labs |
|---|---|---|---|
| Decode a small `tools/call` | 867 B, 10 allocs | 64.7 KiB, 12 allocs | 2.57 KiB, 58 allocs |
| Decode a 1 MiB `tools/call` | 1.010 MiB, 11 allocs | 5.01 MiB, 45 allocs | **1.004 MiB**, 37 allocs |
| Validate typical tool arguments | 2.18 KiB, 67 allocs | 5.58 KiB, 171 allocs | 5.76 KiB, 123 allocs |
| Validate an array of 5,000 objects | 2.48 MiB, 45k allocs | 6.31 MiB, 140k allocs | 5.99 MiB, 130k allocs |
| Compile a tool schema | 16.0 KiB, 258 allocs | 56.9 KiB, 1,467 allocs | 111 KiB, 1,762 allocs |
| Infer a schema from a struct | 14.6 KiB, 100 allocs | 42.3 KiB, 318 allocs | (same as go-sdk) |

### Where mcp-go-next does not win

- **Bytes for a 1 MiB request:** mark3labs allocates 0.66% fewer bytes. That is a tie: both copy the 1 MiB body once. mcp-go-next makes 11 allocations to mark3labs' 37 and is 16% faster.
- **Machine:** absolute times vary between machines and runs, so compare the ratios, not the raw numbers. Across three runs on this machine the ratios moved but kept their order. Validation was 2.5× to 3.1× faster than both libraries, and compiling 5.2× to 6.4× faster.

## What made the difference

- `jsonrpc.Decode` reads the envelope in one pass with a hand-written scanner. It copies only the fields it keeps, and matches keys exactly. A differential fuzz test checks it against the earlier `encoding/json` decoder, with 31.5 million inputs giving the same result.
- `jsonschema` validation parses arguments with a hand-written parser that applies the depth limit as it goes. It is differential-fuzzed against `encoding/json`, with 4.6 million equal inputs.
- Validation builds an error's path only when an error is reported, and never sorts keys on the way.
- Schemas are compiled into a tree of typed nodes once, so validating does no map lookups by keyword.
