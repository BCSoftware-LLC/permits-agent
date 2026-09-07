# Permits Agent

Private BC Software **native Go CLI and stdio MCP server** for California ABC public data. No Python runtime, database service, credentials, or account required. California ABC is connector one for the broader permits platform; this release is the public-data gateway, not filing or legal-compliance automation.

## Build and install

Requires the Go version in `go.mod` or newer. From a private checkout:

```sh
go mod download
make build
./bin/abc-agent version
./bin/abc-agent --help
```

`bin/abc-agent` and `bin/abc-agent-mcp` are self-contained native binaries. Copy them to a directory on your PATH. `make release` builds macOS/Linux arm64/amd64 variants and `dist/SHA256SUMS`; GitHub Actions retains the same private binary artifacts for each passing source SHA. Do not use the old Python `.venv/bin/abc-agent`.

```sh
mkdir -p "$HOME/.local/bin"
install -m 755 bin/abc-agent bin/abc-agent-mcp "$HOME/.local/bin/"
```

## First use

```sh
# Optional: explicit cache location. Otherwise use the native OS cache directory.
export ABC_AGENT_CACHE="$HOME/.cache/permits-agent"
abc-agent refresh
abc-agent stats --json
abc-agent address "6417 SELMA AVE" --limit 0 --json
abc-agent address "6417 SELMA AVE" --status ACTIVE --type 47 --lic-or-app LIC --json
abc-agent get 00677768 --json
abc-agent search "MARKET" --county "LOS ANGELES" --limit 25 --offset 25 --json
abc-agent area --city "LOS ANGELES" --status PEND --limit 20 --json
abc-agent pending --zip 90028 --json
abc-agent expiring --days 30 --county "LOS ANGELES" --json
abc-agent overdue --min-days 30 --json
abc-agent statuses --json
```

**Read commands never refresh the license mirror implicitly.** A missing mirror fails with instructions to run `refresh`. Inspect export date, load time and staleness; a successful download does not prove ABC published new data. `refresh --force` bypasses the local refresh TTL. Old Python DuckDB and Printing Press spike caches remain untouched. `ABC_AGENT_CACHE` takes precedence over legacy `ABC_PP_CACHE`.

## Reference, digest and history

```sh
abc-agent types 47 --json
abc-agent forms --search "ABC-211" --json
abc-agent fees --json
abc-agent requirements 47 --action new --json
abc-agent news --feed all --limit 10 --json
abc-agent news --feed all --offline --json
abc-agent digest --city "LOS ANGELES" --watch 00677768 --json
abc-agent history-snapshot --json
abc-agent history-diff --json
abc-agent doctor --json
```

Reference responses carry provenance and explicit stale/offline metadata. `--offline` means no network request. Current official page/feed coverage is not a complete historical archive. Requirements are **dated curated guidance** for supported type/action pairs, not legally complete filing packages; unsupported mappings fail explicitly. Fees are public reference material, not a price quote or a calculator. Digest news failures must be visible, never silently presented as no news.

History snapshots represent exports actually loaded, one per export date. Refreshing the same date is idempotent. A meaningful diff requires distinct dated snapshots; a one-snapshot result is not evidence that statewide records did not change. History is local, independent of the replaceable license mirror, and survives refresh.

## Output and input contract

- Default data output is formatted JSON; `--json` explicitly selects the same machine-readable format. Diagnostics go to stderr. Help/version are text unless their documented JSON flag is used.
- Record results are arrays, including `[]` for no matches. `--limit 0` returns all; positive limit and `--offset` select a deterministic page. A page size is not a total matching count.
- File numbers must be **exactly eight digits**, including leading zeros. `get` returns every matching license/application/type row; a file number is not a unique record.
- Type codes are exactly two digits. Use `statuses` for recognized official codes. Unknown future source codes are preserved in statistics rather than translated into invented meanings.
- Search/address fragments are literal case-insensitive text, not SQL patterns. Area flags select one premises area using prefix matching; conflicting selectors fail. This is not address standardization or parcel identity verification.
- Expiring includes today through the specified number of days, using California calendar dates. Overdue means an ACTIVE LIC with a past expiration, a follow-up candidate **not official revocation**.
- Exit `0`: success, including no matches. Exit `2`: invalid command/argument/filter. Exit `1`: operational/network/cache failure. Use `doctor` to diagnose local readiness.

## MCP integration

Run `abc-agent-mcp`, `abc-agent serve`, or `abc-agent mcp` as a subprocess. All use stdio; stdout is reserved for MCP JSON-RPC. Example host configuration (substitute the actual installed absolute path):

```json
{
  "mcpServers": {
    "abc_agent": {
      "command": "/absolute/path/to/abc-agent-mcp",
      "args": [],
      "env": {"ABC_AGENT_CACHE": "/absolute/path/to/local-cache"}
    }
  }
}
```

The host discovers schemas with `tools/list` and resources with `resources/list`; mirror reads and reference calls use the same Go domain code as the CLI. `refresh_data` explicitly downloads public data and changes the local cache, never government records. Local history writes are declared as such. No hosted HTTP listener, API-key service, tenant isolation or production deployment is provided. The old hosted Python prototype is retired, not silently ported.

## Verification and operations

```sh
make check build audit
make release
```

Tests use synthetic fixtures and local HTTP servers, not live ABC dependencies. CI runs race/vet/tests on macOS and Linux, verifies module checksums, scans called code for known vulnerabilities, and cross-builds native artifacts. Live official-export and reference dogfood, command/protocol evidence and independent reviews are recorded in [verification](docs/verification.md). See [architecture](docs/architecture.md), [data sources](docs/data-sources.md), and [release contract](docs/go-v1-contract.md).

Back up the local cache/history before operating on it. Failed refresh must preserve the last good mirror. A busy concurrent refresh should be retried after the first finishes; do not manually remove an active lock. Nothing schedules refresh, files an application, sends a message, or collects money automatically.

## Migration and license

The handwritten Python baseline is preserved at Git commit `075cd473330fc51e95a507e628bb6dea89f82179`; it was not Printing Press output. Its known filter defect is not carried into Go. Source lineage and third-party licenses are in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md). BC Software code remains proprietary/internal under [LICENSE](LICENSE). Keep the repository private; public release, payments, protected applicant data, filing and hosted services require separate authorization.
