# Native Go v0.1 release contract

> Historical scope. The September 2026 owner instruction supersedes private/local-only boundaries for the new public product; see [commercial readiness](commercial-readiness.md) and [hosting](hosting.md). The compatibility and correctness requirements below still apply to the native CLI.

Approved scope: finish the existing public-data CLI/MCP feature set in Go; no Python runtime. Blake confirmed tests at CLI invocation, MCP protocol, and public ingestion/history API boundaries. Keep private; no government writes, payments, applicant data, external communications, or hosted deployment. Existing Python is behavioral reference only, not code to harden. Base: 075cd473330fc51e95a507e628bb6dea89f82179.

## Runtime contract

- Binaries: `abc-agent` and `abc-agent-mcp` (stdio). `abc-agent serve` and `mcp` run stdio. Keep CLI names/flags from Python where meaningful; add `fees`, `doctor`, `version`.
- Queries never implicitly refresh the license mirror. `refresh` explicitly uses the official ABC export; optional `--force` bypasses TTL. Missing cache is an actionable error, not a fabricated empty result. Cache env `ABC_AGENT_CACHE`, legacy `ABC_PP_CACHE` fallback. Native SQLite, Python DuckDB cache remains untouched.
- `--json` always emits parseable JSON only on stdout, errors to stderr, nonzero error exit. Default human presentation may be formatted JSON (document explicitly); no ignored format claims. Empty lists `[]`. Exit 2 invalid usage/input; exit 1 operational failure. No error text leaking API keys. No interactive prompt required.
- Record-list responses preserve array compatibility. `--limit 0` means all matching records; positive limit caps result count and is explicitly documented. Add `--offset` for pagination; stable deterministic ordering includes unique row tie-break. `get` returns every record for exact eight-digit file number; never silently normalize malformed identifiers or drop multi-type/multi-record identities.
- Validate filters at shared data/API boundaries: status vocabulary (unknown official future codes can be surfaced via stats), two-digit type, LIC/APP, four-digit year, nonnegative limit/offset/days. Reject conflicting area selectors rather than silently choosing one. Literal user search fragments must not act as SQL wildcards.
- Official dates parsed, daily boundaries in America/Los_Angeles. Overdue = ACTIVE LIC expired before today, derived candidate not official revocation. Expiring includes today through N days. Never call overdue a legal determination.
- Stats and status overview expose source URL, export date, loaded time and stale flag. Queries remain usable offline against existing mirror; stale warning on CLI stderr. Reference fetches expose source/fetched/stale provenance and explicit offline cached mode.
- Refresh import validates banner/header/row shapes/nonempty dataset; failed import preserves last good database. Bounded download and expansion, timeouts, official-host redirect policy. Avoid shared temporary paths. History lives independently from replaceable mirror and survives refresh; snapshot per export date, idempotent, no apparent new daily snapshot from stale data; diff preserves LIC/APP and multi-row identity, explicit added/removed/changed counts.
- Source-backed forms/types/fees/news; requirements port existing curated mappings as dated guidance with citations, not legal completeness. Unsupported type/action is explicit unmapped. No synthesized authority. Digest counts are true totals, not sample lengths; deterministic pending order cannot claim true filing recency absent dates.

## Shared Go interfaces (agent ownership boundaries)

Data owner `internal/abc`: retain existing Store, QueryOptions, AreaFilter, Record. Add QueryOptions.Offset int; AreaFilter.Offset int. Public methods:
- `ByArea(ctx context.Context, area AreaFilter, options QueryOptions) ([]Record,error)`
- `Expiring(ctx context.Context, area AreaFilter, days int, now time.Time) ([]Record,error)`
- `Statuses(ctx context.Context, now time.Time) (any,error)`
- `HistorySnapshot(ctx context.Context) (any,error)` and `HistoryDiff(ctx context.Context) (any,error)`
- Existing Search/Address/Get/Pending/Overdue/Stats/OpenDefault/EnsureData continue.
- `ValidateOptions(QueryOptions) error`, `ValidateArea(AreaFilter) error`; introduce exported `ValidationError` and `IsValidationError(error) bool` for adapter exit/error classification.

Reference owner `internal/reference` new package (may use stdlib + existing x/net/html if needed). Public functions:
- `Types(ctx context.Context, code string, offline bool) (any,error)`
- `Forms(ctx context.Context, search string, offline bool) (any,error)`
- `Fees(ctx context.Context, offline bool) (any,error)`
- `News(ctx context.Context, feed string, limit int, offline bool) (any,error)`
- `Requirements(ctx context.Context, code, action string, offline bool) (any,error)`
Return JSON-safe typed results or maps, include provenance. Shared validation errors can use abc.ValidationError once available; avoid circular dependencies.

Adapter owner `internal/cli`, `internal/mcpserver`, `cmd`: wire above interfaces, no direct SQL. Digest can assemble store results and reference.News (query limit=0 for accurate totals); explicit watched-not-found. MCP keep existing names and add missing Python tools, two existing resources, plus digest/history if useful for full parity. Validate JSON types and integer bounds without silent coercion; no auto-refresh on reads. CLI tests invoke actual Go binaries with isolated cache fixtures; MCP tests exercise initialization/list/call/resources over protocol. Hosted HTTP API-key prototype is not part of approved native stdio release and is explicitly retired, not called production-ready.

## Gates

Go fmt, vet, tests/race, vulnerability scan, clean native build/install smoke; all commands and MCP tools exercised; corrupt/empty/stale/offline/filter/date/history cases; official export dogfood without applicant data or external writes. Independent standards/security and spec reviews pinned to exact candidate. Cross-build macOS/Linux binaries, CI checks exact SHA, repository clean and synchronized after cutover. Retain source provenance, old Python history, rollback path, and limitation documentation. Production-ready claim is only the native public-data CLI/MCP, not the broader regulatory SaaS/case product.
