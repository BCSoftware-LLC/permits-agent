# Public data sources and interpretation

| Surface | Official source | Interpretation |
|---|---|---|
| License/application mirror | https://www.abc.ca.gov/wp-content/uploads/DailyExport-CSV.zip | Daily published snapshot, not a real-time or complete historical register |
| Status definitions | https://www.abc.ca.gov/licensing/license-lookup/glossary/ | Official status is distinct from derived overdue flags |
| License types | https://www.abc.ca.gov/licensing/license-types/ | Parse current public reference; fail on unrecognized shape |
| Forms index | https://www.abc.ca.gov/licensing/license-forms/ | Links and revision labels, not downloaded filing packages |
| Fees | https://www.abc.ca.gov/licensing/license-fees/ | Published reference, not a quote or fee-calculation engine |
| News | https://www.abc.ca.gov/feed/ | Items provided by the current feed, not unlimited archive coverage |
| Industry advisories | https://www.abc.ca.gov/industry-advisories/ | Current advisory listing; older paginated archive is not implied |
| Requirements | Official ABC licensing pages cited per result | Dated, manually curated mappings; forms-index freshness does not imply legal re-verification |

## License records

The CSV includes a BOM, a dated banner, and an official header. File numbers are literal eight-digit strings. License/application/type rows are not necessarily unique businesses or unique file numbers. A site can have multiple associated records. Record counts must never be described as counts of unique businesses.

Address matching is literal case-insensitive substring matching against premises fields (optionally mailing fields). It is not geocoding or proof of parcel identity. Area filters use documented prefix matching; one selector is accepted at a time. Results preserve official spellings. `--limit 0` returns all matches; otherwise list length is only the returned page size.

Status codes observed in an export do not exhaust the official glossary. REV/REVP and other glossary states may be absent from today's export. Overdue means an ACTIVE LIC whose expiration precedes today's California date. It is a **renewal/auto-revocation follow-up candidate**, not proof ABC revoked the license or that the business may lawfully operate. Unknown future source status codes remain visible in statistics; do not silently translate them to invented states.

## Freshness and limits

Use `stats`, `statuses`, and `doctor` to inspect source/export/load metadata. Read commands operate offline against the existing mirror; they do not initiate refresh. Run `refresh --force` when a current source check is required. The published export can remain old even after a successful download.

References cache separately with fetched timestamp, source and stale/offline labels. `--offline` requests no network access. A returned stale fallback must be treated as stale. Parser failures must never become fabricated fallback types or apparently successful empty datasets.

Requirements support only the mapped type/action combinations; unmapped requests fail explicitly. Business-specific entity, location, enforcement and filing circumstances require human review with ABC. The tool neither submits applications nor determines legal compliance.
