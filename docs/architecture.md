# Architecture — permits-agent

## Product thesis

Business owners need agents that can **file documents and gather information on
legacy government websites** — better than a human can. Most agencies have no
public API, but nearly all publish *something* machine-readable: bulk exports,
form libraries, RSS. The winning move is a **local mirror + compound queries**
(Steinberger's discrawl/gogcli playbook, also the core idea behind
printingpress.dev) — not scraping live pages.

## Three layers

| Layer | What | Status |
|---|---|---|
| **1. Data & compliance engine** | Daily-synced local mirror; status tracking, pending-application monitoring, expiration/renewal radar, news/advisories, forms index, fee info. MCP tools + CLI. | ✅ **Shipped (ABC reference adapter), verified live** |
| **2. Document assembly** | Intake → auto-filled application packet (fillable PDFs at predictable URLs + cover sheets + fee estimate + per-type/district submission checklist). Owner signs; agent files. The $500–$2,000 consultant replacement. | ⏳ Next |
| **3. Guided e-filing** | Browser automation against the authenticated portal (e.g. `abcbiz.abc.ca.gov`, CloudFront, no public API) under the customer's own account, human approval gates at submit/payment. | 🔒 Gated — needs legal review + product decisions |

Layer 1 is also the **lead-gen radar**: every PEND application in a zip code
is visible the day it appears.

## Adapter pattern

```
agency/
  client.py    # fetch + cache the agency's public data (CSV export, PDFs, RSS)
  db.py        # load into DuckDB, expose typed queries
  cli.py       # compound commands
  server.py    # MCP tools (same surface per agency)
```

One MCP surface, N adapters → one agent platform for permit compliance across
agencies. `abcgov/` is the reference implementation; agency #2 should be
chosen to prove the pattern generalizes (e.g. CA contractors board, DOJ
firearms, county business licenses, TTB).

## Data pipeline (ABC adapter)

1. Daily export zip: `https://www.abc.ca.gov/wp-content/uploads/DailyExport-CSV.zip`
   (~7.4 MB → ~27 MB CSV, ~129k rows, refreshed daily, no auth).
2. Cache-aware downloader (12h TTL default, `ABC_AGENT_TTL` env).
3. Load into DuckDB file (`~/.cache/abc-agent/abc.duckdb`), indexed on file
   number / name / status. Rebuilt when the source CSV is newer.
4. Query API → CLI + MCP tools.

## MCP surface (11 tools)

search_licenses · get_license · pending_applications · expiring_licenses ·
licenses_in_area · license_stats · license_type_description · search_forms ·
latest_news · fee_surcharges · refresh_data — plus resources `abc://license-types`
and `abc://stats`.

## Guardrails

- Public data only, official channels, one fetch per TTL. Never hammer.
- No credentials in repo. Portal automation (Layer 3) only under the
  customer's account with explicit authorization + approval gates.
- Sworn statements: applicant signs, agent prepares/files.
- Regulatory review before charging for filing services.

## Roadmap

1. Document assembly spike (intake → ABC-211 packet + fee estimate for one
   license type, e.g. new on-sale restaurant 47).
2. Agency #2 adapter (prove generalization).
3. Daily digest cron: new PEND filings + expiring licenses per territory.
4. Priority-registration lottery monitoring (the scarce new-license resource).
