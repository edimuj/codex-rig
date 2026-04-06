# codex-rig

[![Go 1.24+](https://img.shields.io/badge/Go-1.24%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Status](https://img.shields.io/badge/status-active-2ea44f)](#status)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)
[![UX](https://img.shields.io/badge/UX-CLI--first-24292f)](#why-use-it)
[![Rig%20Mode](https://img.shields.io/badge/rigs-share%20%7C%20isolate%20%7C%20inherit-blue)](#policy-model)

A rig manager for Codex that kills config roulette.

`codex-rig` gives you one clean command path for multi-context Codex work: create rigs, bind repos, share only what should be shared, and verify state before it bites you.

## Why Use It

- Stop exporting `CODEX_HOME` by hand and hoping for the best.
- Make repos self-describing with a `.codex-rig` marker.
- Control policy per category: `auth`, `skills`, `plugins`, `mcp`, `history/logs`.
- Get built-in `codex-rig-awareness` in every rig by default.
- Stack rig-specific guidance on top of global `AGENTS` rules automatically.
- Catch drift early with `doctor` and `diff`.
- Keep inherited skills/plugins while preserving local overrides.

## Core Idea

A rig is a named Codex environment with policy.

`codex-rig` does not reimplement Codex internals. It gives you deterministic orchestration:

- resolve effective rig (explicit flag, project marker, or current rig)
- assemble launch env (`CODEX_HOME`, `CODEX_RIG`, `CODEX_RIG_ROOT`)
- enforce policy state idempotently before launch

## Quick Start

```bash
# 1) Create a rig
codex-rig create default

# 2) Make it current for this shell/session
codex-rig use default

# 3) Check effective state
codex-rig status

# 4) Launch Codex through the rig
codex-rig launch -- --help
```

## Terminal Demo

```bash
$ codex-rig create build
created rig "build" at ~/.codex-rig/rigs/build
codex_home=~/.codex-rig/rigs/build/codex-home

$ codex-rig use build
project marker: /path/to/repo/.codex-rig
current rig: build

$ codex-rig inherit skills plugins
rig=build mode=inherited categories=plugins,skills

$ codex-rig auth link --source global
rig=build auth_mode=shared source=global target=~/.codex/auth.json

$ codex-rig doctor
doctor: OK
```

![codex-rig terminal demo](docs/demo.gif)

## Real Workflow Example

```bash
# Create two rigs for different work modes
codex-rig create build
codex-rig create review

# Bind this repo to build rig
codex-rig use build

# Inherit shared skills/plugins, keep history isolated
codex-rig inherit skills plugins
codex-rig isolate history/logs

# Link auth in review rig to build rig (or use global)
codex-rig use review
codex-rig auth link --from-rig build

# Validate
codex-rig doctor
codex-rig diff --all
```

## Policy Model

### Categories

- `auth`
- `skills`
- `plugins`
- `mcp`
- `history/logs`

### Modes

- `shared`: local category path is symlinked to source.
- `isolated`: local category path is owned by the rig.
- `inherited` (skills/plugins only): local directory inherits shared entries while preserving local overrides.

## Auth Modes

- Shared from global source: `global`
- Shared from another rig: `rig:<name>`
- Fully isolated auth file per rig

```bash
codex-rig auth status
codex-rig auth link --source global
codex-rig auth link --from-rig build
codex-rig auth unlink
```

## Project RC File

`codex-rig` uses `.codex-rig` at repo root as the project rig selector:

```ini
rig=default
```

Manage it directly:

```bash
codex-rig rc                 # show marker + effective rig
codex-rig rc set default     # write/update marker
codex-rig rc init            # create marker only if missing
codex-rig rc clear           # remove marker
```

## Built-In Rig Awareness

Every rig ships with `codex-rig-awareness` automatically.

- Skill is extracted to `<rig>/bundled-skills/codex-rig-awareness/SKILL.md`.
- Rig `config.toml` gets an enabled `[[skills.config]]` entry.
- Zero manual copy/paste.

Result: every rig agent starts with the same operating model for rig resolution, policy modes, auth linking, and drift checks.

## Rig Instructions Layering

Each rig can add focused guidance without replacing global defaults.

- Global source (first non-empty): `AGENTS.override.md` then `AGENTS.md` from global `CODEX_HOME`.
- Rig fragment: `<rig>/AGENTS.rig.md` (auto-created).
- Generated runtime file: `<rig>/codex-home/AGENTS.override.md`.

Inspect or regenerate at any time:

```bash
codex-rig instructions --rig <name>
codex-rig instructions sync --rig <name>
```

## Commands

```bash
codex-rig create <name>
codex-rig list
codex-rig use [--no-marker] <name>
codex-rig status
codex-rig launch [--rig <name>] [--codex-bin <path>] [-- <codex args...>]

codex-rig share [--rig <name>] <category...>
codex-rig isolate [--rig <name>] <category...>
codex-rig inherit [--rig <name>] <category...>

codex-rig auth status [--rig <name>]
codex-rig auth link [--rig <name>] [--source global|rig:<name>] [--from-rig <name>]
codex-rig auth unlink [--rig <name>] [--discard]
codex-rig instructions [show] [--rig <name>]
codex-rig instructions sync [--rig <name>]
codex-rig rc [show]
codex-rig rc set <rig>
codex-rig rc init [--rig <name>]
codex-rig rc clear

codex-rig doctor
codex-rig diff [--rig <name>] [--all]
codex-rig version
```

## Install and Update

```bash
# macOS / Linux
curl -fsSL https://raw.githubusercontent.com/edimuj/codex-rig/main/install.sh | sh

# Windows (PowerShell)
irm https://raw.githubusercontent.com/edimuj/codex-rig/main/install.ps1 | iex

# Or with Go
go install github.com/edimuj/codex-rig/cmd/codex-rig@latest
```

Update is the same command again. It always installs the latest GitHub release binary.

## Build From Source

```bash
go build -o codex-rig ./cmd/codex-rig
./codex-rig --version
```

## Releasing

Pushing a `v*` tag triggers the release workflow and publishes cross-platform binaries.

```bash
git tag v0.1.0
git push origin v0.1.0
```

## Why Not Just `CODEX_HOME`?

| Need | Manual `CODEX_HOME` only | `codex-rig` |
|---|---|---|
| Switch environments quickly | Ad hoc shell exports | `codex-rig use <name>` + `codex-rig launch` |
| Bind environment to a repo | Manual memory / shell scripts | `.codex-rig` marker with upward lookup |
| Share/isolate by category | Manual symlink/file management | `share` / `isolate` / `inherit` commands |
| Rig-to-rig auth linking | Hand-edit symlinks | `codex-rig auth link --from-rig <name>` |
| Drift detection | Manual inspection | `codex-rig doctor` and `codex-rig diff` |
| Repeatability for teams | Inconsistent conventions | Explicit policy in rig config |

## Status

Active and usable. API and command surface may still evolve.

## License

MIT. See [LICENSE](./LICENSE).
