---
name: codex-rig-awareness
description: Rig operating discipline for Codex. Use when rig resolution, policy modes, auth links, skills/plugins, MCP config, or instruction layering could affect outcomes.
metadata:
  short-description: Rig-aware workflow and guardrails for Codex sessions
---

# codex-rig-awareness

Default operating discipline for Codex agents running inside a rig.

Use this skill whenever rig resolution, policy mode, auth source, skill loading, plugin state, or MCP setup could change behavior.

## Scope It Covers

- Effective rig resolution order: explicit `--rig`, project marker (`.codex-rig`), then current rig fallback.
- Category policy modes:
  - `auth`: `shared` or `isolated`
  - `skills` and `plugins`: `shared`, `isolated`, or `inherited`
  - `mcp` and `history/logs`: `shared` or `isolated`
- Auth source options: `global` or `rig:<name>`.
- Instruction layering:
  - global source (`AGENTS.override.md` or `AGENTS.md`)
  - rig fragment (`AGENTS.rig.md`)
  - generated runtime override (`AGENTS.override.md` in rig `codex-home`)

## When To Trigger

Apply this skill before making changes if the request touches:

- skills, agent behavior, instruction precedence, or missing guidance
- plugin install/update/removal behavior
- MCP availability or configuration drift
- auth switching (`link`/`unlink`) or account/session confusion
- “wrong config loaded” style failures

## Required Workflow

1. Resolve target rig first.
   - Use `codex-rig status` and/or `codex-rig rc`.
   - If ambiguous, run command with explicit `--rig`.
2. Change state only through `codex-rig` commands.
   - Use `share`, `isolate`, `inherit`, `auth`, `rc`, `instructions sync`.
   - Do not hand-edit symlinks or policy-owned files.
3. Verify post-change state.
   - Run `codex-rig doctor`.
   - Run `codex-rig diff --all`.
4. If still wrong, inspect instruction and skill chain.
   - `codex-rig instructions --rig <name>`
   - confirm bundled skill path is configured in rig `config.toml`.

## Guardrails

- Never assume another rig's auth/plugins/skills state is equivalent.
- For inherited dirs, preserve local overrides and only remove stale inherited links.
- Treat `AGENTS.override.md` in rig `codex-home` as generated output; edit `AGENTS.rig.md` instead.
