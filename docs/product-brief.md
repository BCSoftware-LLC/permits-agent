# Permits Agent: native public-data release

Private BC Software infrastructure for Permits N More. This release is a **native Go CLI and stdio MCP public-data gateway**, not the complete permits-business operating system.

## Included

Official California ABC export ingestion, record/address/area lookup, pending and expiration follow-up, status interpretation, current type/form/fee/news references, curated requirements, local digest and export-date history. Human operators and agents use the same domain code and explicit provenance.

## Not included

Applicant or case storage, agency submissions, form filling, payments/x402, hosted authentication/tenancy, orchestration, agent memory, legal advice, outbound messages, or public marketing/release. Those are separate architecture and approval decisions, not hidden switches in this binary.

## Acceptance

The native public-data surface must satisfy [the Go v0.1 release contract](go-v1-contract.md), deterministic tests, live public-source dogfood, independent review, clean build/audit and exact-SHA CI. The project remains private until separately authorized. Passing these gates applies to the CLI/MCP scope only.
