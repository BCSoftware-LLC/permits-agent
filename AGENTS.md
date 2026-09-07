# Permits Agent engineering contract

Private BC Software native Go CLI/MCP. Read `docs/go-v1-contract.md`, `docs/architecture.md`, and `docs/data-sources.md`. No Python runtime; historical Python remains in Git history.

## Boundaries

- Official public ABC data only. No credentials, applicant data, government mutations, filings, payments, or external communications. Do not turn a stdio tool into a hosted production endpoint without separate approval.
- Never fabricate records, legal requirements, counts, dates, or freshness. Curated requirements are dated guidance, not a complete filing package or legal determination.
- File numbers are strings and not unique records; preserve all license/application/type rows. Parse ABC dates instead of lexicographic comparison.
- Keep literal search text literal and compose filters with explicit grouping. Validate trust-boundary inputs; reject ambiguous selectors and invalid pagination.
- Queries must work against the existing mirror without implicit network refresh. Surface source date/staleness; explicit refresh must preserve last good data on error. Local history survives refresh.
- Protect secrets and cached records from Git, logs, and diagnostics. Tests use synthetic fixtures; live dogfood uses only public official sources and records aggregate evidence.

## Work and verification

Use tests first at approved seams: actual CLI invocation, MCP protocol, and public ingestion/history/query APIs. Prefer standard library and existing dependencies. Keep source ownership coherent; no speculative adapters.

Run `make check build audit`; run `make release` for macOS/Linux native artifacts. Dogfood each command and protocol surface with isolated cache state; verify missing cache, stale/offline reads, malformed sources, filters, multi-record identities, dates, and history. New CLI/MCP capabilities need tests and help/documentation. Do not call the release complete from unit tests alone.

Before release: independent standards/security and specification review; exact Git tree/commit recorded; CI passes on that exact SHA; private repo and local/main/remote parity verified. Preserve inherited work, source provenance, and Python Git history. Do not delete original spike or old local caches without explicit authorization.
