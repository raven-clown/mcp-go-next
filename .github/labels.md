# Labels and milestones

Labels and milestones live in GitHub, not in the repository. This file is the source of truth for them. After changing it, run the commands below again. `--force` updates a label that already exists.

## Labels

| Label | Color | Description |
|---|---|---|
| `bug` | `d73a4a` | Something does not work as documented |
| `enhancement` | `a2eeef` | New behavior or an improvement |
| `docs` | `0075ca` | Documentation only |
| `breaking` | `b60205` | Changes public API or behavior users rely on |
| `spec` | `5319e7` | Follows or fixes the MCP specification |
| `good first issue` | `7057ff` | Small, well scoped, good for a first contribution |
| `help wanted` | `008672` | Nobody has picked this up yet |
| `area/server` | `c5def5` | Server, registry, tools, resources, prompts |
| `area/client` | `c5def5` | Client and connection |
| `area/stdio` | `c5def5` | stdio transport |
| `area/http` | `c5def5` | Streamable HTTP transport and CORS |
| `area/schema` | `c5def5` | JSON Schema inference and validation |
| `area/auth` | `c5def5` | OAuth and principals |
| `area/legacy` | `c5def5` | Revisions before 2026-07-28 |
| `area/gateway` | `c5def5` | Aggregating upstream servers |
| `area/ext` | `c5def5` | Extensions such as tasks, skills and apps |
| `area/ci` | `c5def5` | CI, tooling and repository setup |
| `priority/p0` | `b60205` | Breaks users now |
| `priority/p1` | `d93f0b` | Needed for the next release |
| `priority/p2` | `fbca04` | Planned |
| `priority/p3` | `c2e0c6` | Nice to have |

GitHub's default `documentation` label is renamed to `docs`, so existing pull requests keep their label.

```shell
gh label edit documentation --name docs --color 0075ca --description "Documentation only"
gh label create bug --color d73a4a --description "Something does not work as documented" --force
gh label create enhancement --color a2eeef --description "New behavior or an improvement" --force
gh label create breaking --color b60205 --description "Changes public API or behavior users rely on" --force
gh label create spec --color 5319e7 --description "Follows or fixes the MCP specification" --force
gh label create "good first issue" --color 7057ff --description "Small, well scoped, good for a first contribution" --force
gh label create "help wanted" --color 008672 --description "Nobody has picked this up yet" --force
gh label create area/server --color c5def5 --description "Server, registry, tools, resources, prompts" --force
gh label create area/client --color c5def5 --description "Client and connection" --force
gh label create area/stdio --color c5def5 --description "stdio transport" --force
gh label create area/http --color c5def5 --description "Streamable HTTP transport and CORS" --force
gh label create area/schema --color c5def5 --description "JSON Schema inference and validation" --force
gh label create area/auth --color c5def5 --description "OAuth and principals" --force
gh label create area/legacy --color c5def5 --description "Revisions before 2026-07-28" --force
gh label create area/gateway --color c5def5 --description "Aggregating upstream servers" --force
gh label create area/ext --color c5def5 --description "Extensions such as tasks, skills and apps" --force
gh label create area/ci --color c5def5 --description "CI, tooling and repository setup" --force
gh label create priority/p0 --color b60205 --description "Breaks users now" --force
gh label create priority/p1 --color d93f0b --description "Needed for the next release" --force
gh label create priority/p2 --color fbca04 --description "Planned" --force
gh label create priority/p3 --color c2e0c6 --description "Nice to have" --force
```

## Milestones

| Milestone | Goal |
|---|---|
| `v0.1` | Phase 1: core for servers with many tools, both transports, resilient client, test kit, cookbook |
| `v0.2` | Phase 2: OAuth 2.1 for servers and clients, and the legacy adapter for older revisions |
| `v0.3` | Phase 3: gateway package, interop tests, published benchmarks |
| `v0.4` | Phase 4: extensions, full JSON Schema, OpenTelemetry, operational middleware |
| `v1.0` | API freeze, full conformance, tier listing application |

`v0.1` exists already. The others:

```shell
gh api repos/raven-clown/mcp-go-next/milestones -f title=v0.2 -f description="Phase 2: OAuth 2.1 for servers and clients, and the legacy adapter for older revisions"
gh api repos/raven-clown/mcp-go-next/milestones -f title=v0.3 -f description="Phase 3: gateway package, interop tests, published benchmarks"
gh api repos/raven-clown/mcp-go-next/milestones -f title=v0.4 -f description="Phase 4: extensions, full JSON Schema, OpenTelemetry, operational middleware"
gh api repos/raven-clown/mcp-go-next/milestones -f title=v1.0 -f description="API freeze, full conformance, tier listing application"
```

## Ruleset

`.github/rulesets/main.json` is the ruleset for `main`. Create it once, and update it by ID after a change:

```shell
gh api -X POST repos/raven-clown/mcp-go-next/rulesets --input .github/rulesets/main.json
gh api repos/raven-clown/mcp-go-next/rulesets --jq '.[] | "\(.id) \(.name)"'
gh api -X PUT repos/raven-clown/mcp-go-next/rulesets/<id> --input .github/rulesets/main.json
```
