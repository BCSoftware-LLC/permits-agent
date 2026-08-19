# permits-agent — BC Software internal

**INTERNAL PROJECT — not for public distribution.** Agent-native gateway for
government permitting/licensing data: an MCP server + CLI that gives agents
(and humans) better-than-human access to agency records — license status,
pending applications, expirations, forms, fees, news — over a daily-synced
local mirror.

Powers the **Permits N More** business. **California ABC is the reference
adapter** (verified working); the pattern generalizes to other agencies.

```
agency export ──▶ local DuckDB mirror ──▶ CLI + MCP tools
               (daily snapshot,           (fast compound queries,
                no rate limits)            no scraping)
```

## Status: internal verification phase

- ✔ ABC data engine verified live: 128,870-record daily mirror, all queries
  exercised end-to-end, MCP server (11 tools) tested over stdio
- ✔ Registered in Hermes as `mcp_abc_agent_*` tools
- ⏳ Next: document assembly (Layer 2), agency #2 adapter, productization

## Quick start

```bash
cd ~/bcsoftware/permits-agent
source .venv/bin/activate        # or: uv venv --python 3.11 .venv && uv pip install -e .
pip install -e .

abc-agent refresh                # download + build mirror (~7 MB, once/day)
abc-agent search "stater bros"
abc-agent pending --zip 90028    # new filings in a territory (lead radar)
abc-agent expiring --days 60 --county "LOS ANGELES"
abc-agent get 00677768
abc-agent forms --search "transfer"
abc-agent news --feed advisories
abc-agent serve                  # MCP server over stdio
```

MCP registration (any agent host):

```yaml
mcp_servers:
  abc_agent:
    command: "/Users/bpmac3/bcsoftware/permits-agent/.venv/bin/abc-agent"
    args: ["serve"]
```

## Repo layout

```
abcgov/            CA ABC adapter (reference implementation)
  db.py            daily-export downloader, DuckDB loader, query API
  license_types.py 86 license type codes + descriptions
  forms.py         87-form library index
  fees.py          surcharge table + fee page text
  news.py          news RSS + advisory page parsing
  cli.py           typer CLI
  server.py        FastMCP server (11 tools, 2 resources)
docs/              architecture + data-source notes (see docs/)
tests/             end-to-end MCP stdio smoke test
```

See `docs/architecture.md` for the full plan (data engine → document assembly
→ guided e-filing) and `docs/data-sources.md` for the verified source
inventory and quirks.

## Guardrails

- No public filing API exists; e-filing requires the licensee's own
  `abcbiz.abc.ca.gov` account under explicit authorization, never autonomous on
  attestations/payments.
- ABC application documents contain sworn statements — the **applicant signs**,
  the agent prepares and files.
- Legal review required before charging for filing services themselves (vs.
  software/information services).

© 2026 BC Software LLC. Proprietary and confidential.
