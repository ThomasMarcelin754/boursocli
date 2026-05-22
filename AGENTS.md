# Repository Guidelines — boursocli

Agent-first Go CLI for a personal BoursoBank account. Read-oriented;
the planned assisted virement is human-in-the-loop (stops at SCA,
never executes or bypasses).

## Project Structure

- `cmd/boursocli/`: entrypoint (`main.go`), `signal.NotifyContext`.
- `internal/cli/`: Cobra commands (1 file = 1 command), `session()` orchestration.
- `internal/auth/`: Chrome cookie extraction (embedded `load.mjs`, bi-domain).
- `internal/client/`: audited HTTP transport, Bearer + Cookie data planes.
- `internal/config/`: `config.json` (0600, atomic writes, redacted output).
- `internal/out/`: agent-first output (JSON stdout, logs stderr, table).
- `internal/htmlx/`: strict HTML parsing (schema drift = loud error).
- `internal/version/`: build metadata (ldflags-injectable).
- Tests: `*_test.go` alongside code.

## Build, Test, Dev

```
make build   # go build ./...
make test    # go test ./... -race
make lint    # golangci-lint (incl. gosec)
make check   # fmt vet test lint vulncheck — full pre-commit gate
```

- Conventional Commits (`feat:`, `fix:`, `docs:`, `chore:`).
- User-facing strings in **French**; command names, flags, identifiers,
  and code comments in **English** (Go convention).

## Safety

- Never auto-execute or retry a virement step (non-idempotent, under SCA).
  Stop at the SCA screen and hand back to the human.
- Personal account: serialize requests at human pace. On throttle, back
  off — never re-authenticate in a loop.
- Never commit secrets (cookies, tokens, IBANs, account keys).

## Install

```
brew install --cask thomasmarcelin754/tap/boursocli   # macOS (pre-built)
go install github.com/thomasmarcelin754/boursocli/cmd/boursocli@latest  # Go devs
```

Release: tag push → goreleaser `homebrew_casks:` auto-pushes Cask to
`ThomasMarcelin754/homebrew-tap`.

## Status

24 read commands built, published (v0.1.1) and validated on a live account
(2026-05-22). Production tooling in place: CI, golangci-lint+gosec,
govulncheck, goreleaser (6 platforms, SBOM, cosign), Homebrew Cask auto-tap,
Dockerfile, unit tests with per-package coverage floors.
Not yet built: assisted `virement` (write, under SCA).
