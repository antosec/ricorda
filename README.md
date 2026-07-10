# ricorda

> **The cheatsheet that writes itself.** `ricorda` reads your shell history — locally, offline — finds the commands you fought for, and turns them into personal, per-tool markdown cheatsheets.

[![CI](https://github.com/antosec/ricorda/actions/workflows/ci.yml/badge.svg)](https://github.com/antosec/ricorda/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/antosec/ricorda.svg)](https://pkg.go.dev/github.com/antosec/ricorda)

You fixed that `ffmpeg` incantation three weeks ago. It took six attempts, two trips to `--help` and one to Stack Overflow. Today you need it again and it's… gone, buried under four thousand lines of history.

Generic cheatsheets ([tldr](https://github.com/tldr-pages/tldr), [cheat.sh](https://github.com/chubin/cheat.sh)) tell you what *everyone* needs. Interactive ones ([navi](https://github.com/denisidoro/navi), [cheat](https://github.com/cheat/cheat)) make you write the sheets yourself. **Your history already knows what *you* need — ricorda just writes it down.**

## How it works

1. `ricorda scan` reads every shell history it can find — **PowerShell, bash, zsh, fish** — 100% locally.
2. It detects *struggle*: chains of near-identical retries (the last attempt is the one that worked), trips to `--help` / `man` / `tldr`, and your most-repeated non-trivial commands.
3. It writes one markdown sheet per tool into `~/.ricorda/sheets/`, redacting anything that looks like a secret first.
4. Your own notes below the keep-marker in each sheet survive every rescan.

## Install

With Go 1.22+:

```
go install github.com/antosec/ricorda@latest
```

Prebuilt binaries, Homebrew and Scoop packages are on the roadmap.

## Quickstart

```
$ ricorda scan
  pwsh     2140 commands  (C:\Users\you\AppData\...\ConsoleHost_history.txt)
  bash     1512 commands  (/home/you/.bash_history)

Scanned 3652 commands across 2 source(s).
Wrote 14 cheatsheet(s) → /home/you/.ricorda/sheets

Your biggest fights:
  docker         7 hard-won command(s)
  ffmpeg         3 hard-won command(s)

Try: ricorda docker

$ ricorda docker
# docker — personal cheatsheet

## Hard-won (you fought for these)

- `docker run -it --rm -v $PWD:/app node:20 bash`  — took 4 attempts
- `docker buildx build --platform linux/amd64,linux/arm64 -t me/app .`  — took 3 attempts

## Greatest hits

- `docker compose up -d`  — ×42
```

## Privacy, by construction

- **Nothing ever leaves your machine.** No telemetry, no cloud, no accounts. `scan` prints only aggregate counts; your commands go into the sheets, which are files you own.
- **Secrets are redacted before touching disk**: `--password`/`--token`-style flags, `VAR_TOKEN=…` assignments, bearer headers, and well-known credential shapes (GitHub, Slack, AWS, Stripe, JWT).
- Delete `~/.ricorda` and every trace is gone.

## Commands

| Command | What it does |
|---|---|
| `ricorda scan` | Read your history and (re)build your sheets |
| `ricorda` | List your sheets |
| `ricorda <tool>` | Show your personal sheet for a tool |
| `ricorda edit <tool>` | Open a sheet in `$EDITOR` — notes below the keep-marker survive rescans |

## Roadmap

- [ ] Shell hooks for exit-code-aware struggle detection (know *which* attempt failed)
- [ ] `ricorda export --format navi|tldr` for interop with existing tools
- [ ] Fuzzy-search across all sheets (`ricorda find <text>`)
- [ ] Optional local-LLM annotations via ollama — explanations without breaking the zero-cloud promise
- [ ] More shells: nushell, xonsh
- [ ] Homebrew tap / Scoop bucket / prebuilt releases

## Contributing

Issues, ideas and PRs are very welcome — see [CONTRIBUTING.md](CONTRIBUTING.md). New history-source parsers are a perfect first contribution.

## Why "ricorda"?

It's Italian for *"remember!"* — the imperative. Your shell already remembers everything; ricorda makes it useful.

## License

[MIT](LICENSE)
