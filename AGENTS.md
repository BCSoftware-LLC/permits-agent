# AGENTS.md — permits-agent repo conventions

This file is the operating contract for AI agents (Hermes, Claude Code, Codex,
etc.) working in this repository. Read it before making changes.

## What this repo is

Internal BC Software platform: agent-native access to government permitting /
licensing data. The California ABC adapter (`abcgov/`) is the reference
implementation and the only shipped adapter. Everything generalizes via the
adapter pattern (see docs/architecture.md).

## Non-negotiable rules

1. **Public data only, on the official channel.** Fetch data from the sources
   documented in docs/data-sources.md (daily CSV export, forms page, RSS).
   Do NOT scrape the site aggressively, bypass CloudFront, or hammer endpoints.
   The local mirror exists precisely so we never hammer anything.
2. **Never fabricate data.** Every query result must trace to the local mirror
   which traces to the daily export. If the export is stale or missing, say so.
3. **No credentials in the repo.** Licensee portal credentials, API keys,
   tokens never enter this repo, config, or commit history.
4. **File numbers are not unique** — one file number can map to multiple rows
   (one per license type). Never collapse them silently.
5. **Dates are `DD-MON-YYYY`** in the export; parse with strptime before
   comparisons, never lexicographic string compare.
6. **Licensing/legal sensitivity**: ABC documents contain sworn statements and
   the applicant must sign. Any feature touching actual filing carries human
   approval gates. Flag regulatory questions, don't guess.

## Verification expectations

- After any change to `abcgov/`, run: `abc-agent refresh`, one search, one
  area query, `abc-agent stats`, and `python tests/smoke_mcp.py` (exercises the
  MCP server over stdio).
- Commits must not include `__pycache__`, `.venv`, cache JSON, or the DuckDB
  mirror (all gitignored).

## Getting started

```bash
cd ~/bcsoftware/permits-agent
source .venv/bin/activate
pip install -e .
abc-agent refresh
```

See README.md for the command surface.
