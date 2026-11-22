# Benchmarks

mcp-go-next is measured side by side with [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go) v1.6.0 and [modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) v1.8.0, on the same inputs and machine. Results are published as they come out, including where mcp-go-next loses or ties. They hold for these versions only: when Dependabot moves `bench/go.mod` to newer releases, the tables are measured again.

So far this covers the parts that exist in mcp-go-next: decoding JSON-RPC messages, and building and checking JSON Schemas. Whole-server numbers (a tool call through each transport, memory per connected caller, `tools/list` with hundreds of tools) come when the server and transports land in later v0.1 tasks.

## How it is measured

The benchmarks live in `bench/`, a separate module with its own `go.mod`, so the core module keeps zero dependencies. Run them with `make bench-compare`, or:

```shell
cd bench
go test -run '^$' -bench . -benchmem -count 8 > new.txt
go run golang.org/x/perf/cmd/benchstat@v0.0.0-20261009192801-be2c69fb417e -col /lib new.txt
```

**Which code is measured:**
- Both other libraries build schemas with `github.com/google/jsonschema-go`.
- To validate tool arguments, go-sdk uses `jsonschema-go` and mark3labs uses `github.com/santhosh-tekuri/jsonschema/v6`.
- Each benchmark starts from the raw bytes a server receives:

| Benchmark | mcp-go-next | go-sdk | mark3labs |
|---|---|---|---|
| Decode a `tools/call` request | `jsonrpc.Decode` | `jsonrpc.DecodeMessage`, which its transports call | `json.Unmarshal` into `mcp.JSONRPCRequest` (no exported decoder) |
| Validate tool arguments | `Schema.Validate(raw)` | `applySchema` from `mcp/tool.go`, step for step (see below) | `jsonschema.UnmarshalJSON` + `Schema.Validate`, as in `server/input_validation.go` |
| Compile a schema | `Basic().Compile` | `json.Unmarshal` into `Schema` + `Resolve` | `NewCompiler`, `AddResource`, `Compile` |
| Infer a schema from a struct | `jsonschema.For` | `jsonschema.For` + `json.Marshal` (shared by both) | same as go-sdk |

go-sdk's `applySchema` runs these steps on every tool call:
1. a nesting depth scan
2. decoding with `segmentio/encoding/json` into a map
3. `ApplyDefaults`
4. `Validate`
5. `json.Marshal` of the result

The benchmark runs the same steps. Arguments that are not an object take its path without defaults.

**Fairness:**
- **Decoding:** for go-sdk the benchmark is the code its server runs. For mark3labs it is less work than its server does: its `HandleMessage` unmarshals each request twice, first an envelope and then the typed request, and the benchmark unmarshals once.
- **Validation:** mark3labs is measured at its validator core. Its server also hashes the tool schema with SHA-256 on every call to find the cached validator. go-sdk is measured along its real path, including the defaults step and the marshal after it. mcp-go-next has no default-filling step.
- **Same verdicts:** `TestSameVerdicts` checks that all three validators accept and reject exactly the same instances, so no library looks faster by checking less.
- **Same schema:** the tool schema is written by hand, so every validator checks the same keywords. It covers `type`, `enum`, length and numeric bounds, `items`, `required` and `additionalProperties: false`.

## Results, 2026-11-22

The machine was an Intel Core i7-14700 on Windows 11, with Go 1.27.4, 8 runs per benchmark and `benchstat`. mcp-go-next was at commit `d10dfd6` (v0.1.0) plus the jsonrpc scanner from this change. Raw output: [benchmarks/2026-11-22-raw.txt](benchmarks/2026-11-22-raw.txt).

Each cell compares the library with mcp-go-next. "3.0× slower" means it took three times as long. A tie means `benchstat` found no significant difference.

### Time per operation

| Benchmark | mcp-go-next | go-sdk | mark3labs |
|---|---|---|---|
| Decode a small `tools/call` (~350 B) | 837 ns | 8.42 µs, 10.1× slower | 5.45 µs, 6.5× slower |
| Decode a 1 MiB `tools/call` | 988 µs | 2.23 ms, 2.3× slower | 1.09 ms, **tie** (p=0.065) |
| Validate typical tool arguments | 3.97 µs | 49.0 µs, 12.4× slower | 8.61 µs, 2.2× slower |
| Validate an array of 5,000 objects | 2.07 ms | 5.08 ms, 2.5× slower | 6.52 ms, 3.2× slower |
| Compile a tool schema | 21.6 µs | 98.9 µs, 4.6× slower | 120 µs, 5.6× slower |
| Infer a schema from a struct | 10.9 µs | 47.7 µs, 4.4× slower | (same as go-sdk) |

### Memory allocated per operation

| Benchmark | mcp-go-next | go-sdk | mark3labs |
|---|---|---|---|
| Decode a small `tools/call` | 867 B, 10 allocs | 64.7 KiB, 12 allocs | 2.57 KiB, 58 allocs |
| Decode a 1 MiB `tools/call` | 1.010 MiB, 11 allocs | 5.01 MiB, 45 allocs | **1.004 MiB**, 37 allocs |
| Validate typical tool arguments | 2.18 KiB, 67 allocs | 39.1 KiB, 247 allocs | 5.77 KiB, 123 allocs |
| Validate an array of 5,000 objects | 2.48 MiB, 45k allocs | 6.18 MiB, 125k allocs | 5.99 MiB, 130k allocs |
| Compile a tool schema | 16.0 KiB, 258 allocs | 56.9 KiB, 1,467 allocs | 111 KiB, 1,762 allocs |
| Infer a schema from a struct | 14.6 KiB, 100 allocs | 42.3 KiB, 318 allocs | (same as go-sdk) |

### Where mcp-go-next does not win

- **Decoding a 1 MiB request against mark3labs:** time and bytes are a tie. Neither difference is significant, and mark3labs allocates 0.64% fewer bytes. Both copy the body once. mcp-go-next makes 11 allocations to mark3labs' 37, but in this run that does not make it measurably faster. A reproduction during review put mcp-go-next 4% to 19% ahead, so call it close.
- **Absolute numbers:** times vary between machines and runs, so compare the ratios. The order of the libraries was the same in every run so far.
- **Other benchmark sets:** the validation number here (3.97 µs) differs from the one in the v0.1.0 notes (about 3 µs). Those notes used `jsonschema/bench_test.go`, which has a different schema and arguments.

## What made the difference

- `jsonrpc.Decode` reads the envelope in one pass with a hand-written scanner. It copies only the fields it keeps, and matches keys exactly. A differential fuzz test checks it against the earlier `encoding/json` decoder, with 31.5 million inputs giving the same result.
- `jsonschema` validation parses arguments with a hand-written parser that applies the depth limit as it goes. It is differential-fuzzed against `encoding/json`, with 4.6 million equal inputs.
- On the success path, validation builds no error paths and sorts no keys. Paths are built only when an error is reported. Keys are sorted only in the second pass that runs after more than 100 errors.
- Schemas are compiled once into a tree of typed nodes, so validating does no map lookups by keyword.
- Validation checks and reports. It does not fill defaults and marshal the result again, which is most of go-sdk's cost here.
