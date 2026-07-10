# Contributing to ricorda

Thanks for stopping by! This project is young — issues, ideas and PRs are all welcome.

## Dev setup

1. Install Go 1.22+
2. `git clone https://github.com/antosec/ricorda && cd ricorda`
3. `go test ./...`
4. `go run . scan`

## Ground rules

- **Privacy is the product.** Nothing may leave the user's machine, and every command string that gets persisted must pass through `internal/redact` first.
- Keep it dependency-light (currently: cobra, nothing else).
- `go test ./...` must pass on Linux, macOS and Windows — CI checks all three.

## Great first contributions

- A new history source (nushell, xonsh, PowerShell Core on Linux…): implement a parser in `internal/history` plus a table test — ~50 lines total.
- New redaction rules for credential shapes we miss (add a test case first).
- Better struggle heuristics in `internal/analyze` (with tests showing the before/after).
