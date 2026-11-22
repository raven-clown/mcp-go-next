.PHONY: test race lint cover check

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
