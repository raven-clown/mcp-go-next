.PHONY: test race lint cover check bench-compare

test:
	go test ./...

race:
	go test -race ./...

lint:
	golangci-lint run ./...

# Passes while the module has no tests yet, so CI is green from the first commit.
cover:
	@if [ -z "$$(find . -name '*_test.go' -not -path './examples/*' | head -n 1)" ]; then \
		echo "coverage: no tests yet"; \
	else \
		go test -coverprofile=coverage.out ./... && \
		go tool cover -func=coverage.out | awk '/^total:/ { sub("%", "", $$3); print "coverage " $$3 "%"; if ($$3 + 0 < 80) { print "coverage is below 80%"; exit 1 } }'; \
	fi

check: lint cover

# Compares mcp-go-next with mark3labs/mcp-go and go-sdk. See docs/benchmarks.md.
bench-compare:
	cd bench && go test -run '^$$' -bench . -benchmem -count 8 > bench.txt && \
		MSYS_NO_PATHCONV=1 go run golang.org/x/perf/cmd/benchstat@v0.0.0-20261009192801-be2c69fb417e -col /lib bench.txt
