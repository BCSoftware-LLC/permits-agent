# Architecture

## ADR-001: native Go, local public-data mirror

**Decision:** ship one native Go public-data CLI and stdio MCP server. Go supersedes Python as the delivery target. The private repository remains `BCSoftware-LLC/permits-agent`; the handwritten Python baseline survives in Git at `075cd473330fc51e95a507e628bb6dea89f82179`. The initial Go scaffold came from cli-printing-press 4.31.1; see [notices](../THIRD_PARTY_NOTICES.md). No generated code is presumed verified.

### Small module boundaries

- `internal/abc`: official-export download/import, SQLite license mirror, validation, literal filters, expiration rules, source/status metadata, local snapshots/diffs.
- `internal/reference`: source-backed types/forms/fees/news and explicitly curated requirements. Source provenance, freshness, atomic local cache and offline behavior live here, not in adapters.
- `internal/cli`: Cobra commands; JSON on stdout, diagnostics on stderr; no SQL or hidden refresh on reads.
- `internal/mcpserver`: MCP tool/resource schemas and handlers over stdio; shared domain APIs, not alternate business rules.
- `cmd/abc-agent` and `cmd/abc-agent-mcp`: process entrypoints.

SQLite uses the pure-Go modernc driver, allowing CGO-disabled macOS/Linux binaries. No Python, DuckDB, service, database credentials, or container is required. Cache root is `ABC_AGENT_CACHE`, legacy `ABC_PP_CACHE`, then the native OS cache directory plus `abc-agent`. Old Python caches are never migrated or deleted implicitly.

### State and correctness

Read commands open an existing mirror and never refresh implicitly. Explicit `refresh` downloads/imports official public data with bounded input and rejects invalid sources while preserving the previous mirror. Expiration comparisons use California calendar dates; overdue is a derived follow-up candidate, not official revocation. The original address SQL grouping defect must not recur. All record-list queries retain multi-record identities and support documented limits/offsets.

History is local evidence from exports actually loaded, not a statewide event archive. One snapshot per export date; refreshing the same source date is idempotent. `diff` needs distinct export dates for a meaningful comparison. No inferred filing dates or real-time status promises.

### Trust boundaries

The CLI accepts untrusted arguments; MCP accepts untrusted typed JSON arguments. Validation belongs at public data interfaces as well as transport parsing. Official public pages are untrusted network input: bound bytes/time, reject parser drift, restrict redirects, preserve valid cached data. Do not label stale cache fallback a live response.

The stdio server inherits the invoking OS user's local privileges and cache. There is no public listener, hosted API-key authentication, tenant isolation, or filing authority in this release. Old hosted Python/payment proposals are historical Git documents, not working or approved features. Government writes, applicant records, payments, and outbound communication remain separate approval-gated work.

### Verification

See [Go release contract](go-v1-contract.md). `make check build audit` covers formatting, vet, race tests, module integrity, native binaries and called-code vulnerability scanning. `make release` cross-builds macOS/Linux arm64/amd64 binaries with checksums. GitHub CI runs native checks on macOS/Linux and retains private artifacts. Live official-source dogfood is deliberately separate from deterministic network-independent tests. See `docs/verification.md` when release evidence has been recorded.
