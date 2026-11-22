# Contributing to mcp-go-next

Thanks for wanting to help. Bug reports, ideas, docs fixes and code are all welcome.

## Before you start

- Look through the [open issues](https://github.com/raven-clown/mcp-go-next/issues) and the [roadmap board](https://github.com/users/raven-clown/projects/9) first. Every task in the plan has an issue. Issues labelled `good first issue` are small and well scoped, and `help wanted` marks work nobody has picked up yet.
- `docs/spec-coverage.md` (added with the first release) shows which parts of the MCP spec are done and which are open.
- For anything bigger than a small fix, comment on the issue or open a discussion before writing code, so we agree on the approach first.
- Say on the issue that you are working on it, so two people do not do the same work.
- Questions go to [Discussions](https://github.com/raven-clown/mcp-go-next/discussions), not issues.
- Security problems never go in a public issue. See [SECURITY.md](SECURITY.md).

## Set up

You need the latest Go release (1.27.4 today, the version in `go.mod`). `golangci-lint` v2 is optional but CI runs it. Node LTS is only needed to run the conformance suite.

```shell
git clone https://github.com/raven-clown/mcp-go-next
cd mcp-go-next
go build ./...
go test ./...
make race       # go test -race ./...
make check      # lint plus the 80% coverage gate
```

Once the conformance programs land, `make conformance` runs the official MCP conformance suite against them.

The design is in [docs/design.md](docs/design.md), the roadmap in [docs/plans/roadmap.md](docs/plans/roadmap.md), and [AGENTS.md](AGENTS.md) lists the package layout and the rules below in more detail.

## Rules for code

- Write the test first. Table-driven where there is a table.
- No network in tests except `httptest`, and no bare `time.Sleep`. Poll with a deadline instead.
- Every test must pass on Windows, macOS and Linux. Build paths with `filepath`.
- The core module has no third-party dependencies, tests included. Code that needs one goes in `examples/` or a separate module.
- Library packages never print and never touch `os.Stdout` or `os.Stderr`.
- Keep comments short and about why, not what.

## Pull requests

1. Fork the repository and create a branch named `feat/<topic>`, `fix/<topic>`, `docs/<topic>`, `ci/<topic>` or `chore/<topic>`.
2. Use [Conventional Commits](https://www.conventionalcommits.org/) messages, for example `fix(stdio): read a final line without a newline`.
3. Add a line to `CHANGELOG.md` under Unreleased for anything users will notice.
4. Keep one topic per pull request. Small pull requests get reviewed faster.
5. Fill in the pull request template and link the issue with `Closes #123`.

CI runs the tests with `-race` on all three systems and two Go versions, plus lint, coverage and govulncheck. Workflows from outside contributors wait for a maintainer to approve them, so a pending check right after you open the pull request is normal.

Review looks for:
- tests written first and covering the change
- no new dependencies in the core module
- Windows passing
- docs and CHANGELOG updated where users will notice

Pull requests are squash merged, so the title becomes the commit on main. Please write it as a Conventional Commit too.

## Code of conduct

Everyone taking part agrees to the [Code of Conduct](CODE_OF_CONDUCT.md).

## License

Contributions are licensed under the [Apache License 2.0](LICENSE), as section 5 of the license says. There is no CLA and no sign-off to add.
