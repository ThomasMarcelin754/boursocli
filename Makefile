GOLANGCI    := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
GOVULNCHECK := go run golang.org/x/vuln/cmd/govulncheck@latest
GORELEASER  := go run github.com/goreleaser/goreleaser/v2@latest

.PHONY: build test vet fmt lint vulncheck sec check release-check snapshot docker homebrew

build:
	go build -o boursocli ./cmd/boursocli

test:
	go test ./... -race

vet:
	go vet ./...

fmt:
	gofmt -w internal cmd

lint:
	$(GOLANGCI) run ./...

# Dependency + stdlib CVE scan (Go team's official tool; reachability-aware:
# only flags vulnerabilities our code actually transitively calls).
vulncheck:
	$(GOVULNCHECK) ./...

# Static security (gosec, via golangci-lint) + known-CVE scan.
sec: lint vulncheck

# goreleaser config validation (the "no remote" runtime check only passes
# once a GitHub origin exists — i.e. in CI; the config itself is valid).
release-check:
	$(GORELEASER) check || true

# Build all release artifacts locally without publishing (sanity check).
snapshot:
	$(GORELEASER) release --snapshot --clean --skip=publish

docker:
	docker build -t boursocli:dev .

# Print Homebrew formula fields for a tag: make homebrew VERSION=0.1.0
homebrew:
	scripts/release-homebrew.sh $(VERSION)

# Full pre-commit gate.
check: fmt vet test lint vulncheck
