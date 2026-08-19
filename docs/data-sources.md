# Data sources — verified inventory (CA ABC)

All facts below were verified against the live site on 2026-08-19.

## Primary: daily license export (the data engine)

| | |
|---|---|
| URL | `https://www.abc.ca.gov/wp-content/uploads/DailyExport-CSV.zip` |
| Format | ZIP → single CSV (~27 MB), **~128,870 rows**, refreshed daily |
| Auth | none; no rate limit observed; robots.txt only disallows /wp-admin/ |
| Coverage | every license AND pending application in California |

Columns: License Type · File Number · Lic or App · Type Status · Type Orig Iss
Date · Expir Date · Fee Codes · Dup Counts · Master Ind · Term in # of Months ·
Geo Code · District · Primary Name · Prem Addr 1/2 · Prem City · Prem State ·
Prem Zip · DBA Name · Mail Addr · Prem County · Prem Census Tract #

### Quirks (all handled in `abcgov/db.py`)
- Line 1 is a human banner (`"Updated Tuesday 18th of August 2026 …"`); the
  real header is line 2 (`skip=1, header=true`).
- **Dates are `DD-MON-YYYY`** (e.g. `31-DEC-2026`) — must strptime; naive
  string compare is wrong.
- **File numbers are NOT unique** — one file number = multiple rows (one per
  license type). `get` returns all rows.
- Statuses: ACTIVE (118,981) · PEND (6,620) · SUREND (2,376) · REVPEN (410) ·
  SUSPEN (357) · R64B (126). `Lic or App`: LIC 108,840 / APP 20,030.
- Some premises addresses are empty for pending apps; zip may carry `-NNNN`.

## Reference pages (scraped once, cached as JSON)

| Source | URL | What we extract |
|---|---|---|
| License types | `/licensing/license-types/` | 86 codes + names + descriptions. **Page's div ids are toggle counters, not codes** — code lives in the h3 heading. |
| Forms | `/licensing/license-forms/` | 87 forms: number, title, revised date, PDF URL under `/wp-content/uploads/forms/ABC-XXX.pdf` |
| Fees | `/licensing/license-fees/` | statutory surcharge table (Appeals Board 3%, CHP $10, Business Practices) + page text. Base annual/application fee schedules are prose/documents, not tables — belongs to Layer 2 calculator. |
| News | `/feed/` | WordPress RSS (news releases) |
| Advisories | `/industry-advisories/` | **The `…/feed/` URL serves a comments feed (0 items)** — we parse the category page's article list instead. |

## Interactive lookup / filing surfaces

| Surface | Reality |
|---|---|
| `lookup.abc.ca.gov` | **Dead** — no longer resolves; lookup merged into the main site's report pages + our mirror |
| Report pages (`licenses-by-zip`, `license-number`, …) | WordPress pages; the daily export covers the same data with more flexibility |
| `abcbiz.abc.ca.gov` | Authenticated portal (CloudFront 403 to anonymous), no public API. RBS portal + License Administrator. **The filing channel — Layer 3, credential-gated.** |
| data.ca.gov (Socrata) | No ABC license dataset found (catalog query empty) |

## Operational rules

- One export download per TTL (default 12h) — the mirror exists so we never
  hammer the site.
- Cache files live in `~/.cache/abc-agent/` (env `ABC_AGENT_CACHE`), never in
  the repo.
- If the export is missing/stale, tools surface the snapshot date — never
  fabricate or serve stale data as fresh.
