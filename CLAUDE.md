# CLAUDE.md — boursocli

Instructions for Claude Code / AI agents working in this repository.
Human contributor guidelines are in `AGENTS.md`.

## Build & test

```
make build   # go build ./...
make test    # go test ./... -race
make lint    # golangci-lint (incl. gosec)
make check   # fmt vet test lint vulncheck — full pre-commit gate
```

## Commit style

Conventional Commits (`feat:`, `fix:`, `chore:`, `docs:`).

## What this is

Agent-first Go CLI for a personal BoursoBank account. Read-oriented.
The planned assisted virement is human-in-the-loop and **stops at the
SCA screen — it never executes or bypasses strong authentication**.

## Safety rules

- Never auto-execute, auto-retry, or bypass a virement step.
- Serialize requests at human pace. On throttle, back off — never
  re-authenticate in a loop.
- Never commit secrets (cookies, bearer tokens, IBANs, account keys).
  Use obvious placeholders in code and tests.
