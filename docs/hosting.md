# Hosting and operations

Status: deployable pilot candidate, **not a completed production filing service**. Public product: Permits Agent. First customer: Permits N More. The public repository is intentional; customer data and configuration are private.

## Architecture decision

Use Cloudflare DNS/TLS/WAF and optionally the included Worker proxy for the public website, API and MCP. Run one native Go origin on a host with an **encrypted durable volume**. It serves the embedded website and owns the case database, access control, request ledger and optional assistant. ABC data is a separate refreshable cache. This reuses the verified Go data engine without inventing a second implementation in Workers.

Do not put the only SQLite case or usage database on Cloudflare Containers' local disk: Cloudflare documents that disk as ephemeral across sleep. A future all-Cloudflare design must move durable state to D1/Durable Objects and documents to R2 with explicit adapters and tested isolation. R2 file mounting is not a substitute for a tested SQLite durability design.

Cloudflare AI Gateway can sit in front of the optional model and attribute usage by an opaque tenant hash. Gateway analytics measures upstream model traffic; the application request ledger enforces tenant access allowances. Neither is an invoicing system. The current Cloudflare Monetization Gateway announcement invites waitlist registration; do not depend on access before verifying the account's availability.

Sources checked September 20, 2026 UTC:

- https://developers.cloudflare.com/containers/faq/
- https://developers.cloudflare.com/ai-gateway/observability/user-insights/
- https://developers.cloudflare.com/ai-gateway/usage/providers/openai/
- https://blog.cloudflare.com/monetization-gateway/
- https://docs.stripe.com/billing/subscriptions/usage-based/recording-usage

## Local synthetic-data preview

```sh
make build
mkdir -p /PRIVATE_PATH/permits-agent
./bin/permits-agent-server keygen SYNTHETIC_TENANT > /PRIVATE_PATH/permits-agent/key-once.json
```

The output contains a one-time raw `access_key` and a `credential` object containing only its hash. Store the raw key securely, share it only with its intended customer, and write a JSON array of credential objects to a private `keys.json` file. Do not put either file in this repository, shell history, tickets or public logs. `keygen` does not register the key anywhere. The server loads the file at startup. Restart after adding, rotating or revoking keys.

```sh
./bin/permits-agent-server \
  --listen 127.0.0.1:8080 \
  --origin http://localhost:8080 \
  --database /PRIVATE_PATH/permits-agent/cases.sqlite \
  --keys /PRIVATE_PATH/permits-agent/keys.json
```

A credential has `tenant`, `token_hash`, `scopes`, `monthly_request_limit`, and `monthly_model_call_limit` (default zero). Valid scopes are `research`, `cases:read`, `cases:write`, and `agent:read`. Use opaque tenant IDs. Keys for a tenant must share the same request and model-call limits. Permissions do not imply one another: read access must be granted explicitly. Customer IDs are always taken from the authenticated key; callers cannot choose another tenant.

Visit the local origin. The public fictional example requires no key. Connect a pilot key to create and reopen cases, append notes, download preparation briefs, and export review reminders. Browser keys live in memory and are cleared on reload/disconnect. Do not enter sensitive applicant information in this pilot.

## Data refresh

Hosted clients cannot refresh ABC or modify shared history. An operator refreshes the cache separately using the existing CLI:

```sh
ABC_AGENT_CACHE=/PRIVATE_PATH/abc ./bin/abc-agent refresh --force
ABC_AGENT_CACHE=/PRIVATE_PATH/abc ./bin/abc-agent forms
ABC_AGENT_CACHE=/PRIVATE_PATH/abc ./bin/abc-agent types
ABC_AGENT_CACHE=/PRIVATE_PATH/abc ./bin/abc-agent fees
ABC_AGENT_CACHE=/PRIVATE_PATH/abc ./bin/abc-agent news
```

Use the same `ABC_AGENT_CACHE` for the HTTP server. Confirm all command help before automation. Maintain an operator-owned refresh schedule and freshness alerts on the hosting platform; this change does not create a live scheduled job. Hosted ABC reference calls force offline cache reads, and missing/stale evidence remains explicit. `source_catalog` and `source_read` separately retrieve curated public state, federal and local HTML entry points, with 24-hour cache, final URL, content fingerprint, bounded excerpts and explicit stale fallback. They do not accept arbitrary caller URLs or prove site-specific permit applicability. TTB returned HTTP 403 in live validation; direct agency review remains required for that source. Never replace the existing live Python registration until a separate cutover and rollback test passes.

## Optional owner assistant

Disabled unless `PERMITS_AGENT_MODEL` is set. Configure:

- `OPENAI_API_KEY`: provider key in the host's secret store.
- `PERMITS_AGENT_MODEL`: explicitly selected supported Responses model. No model or price is assumed by the code.
- `PERMITS_AGENT_RESPONSES_URL`: default `https://api.openai.com/v1/responses`; optional `https://gateway.ai.cloudflare.com/v1/ACCOUNT/GATEWAY/openai/responses`.
- `CF_AIG_TOKEN`: optional Cloudflare authenticated gateway token.

Also grant the tenant key `agent:read`, `cases:read`, and `research`, and set a positive `monthly_model_call_limit`. Default generated keys have no assistant permission or budget. Set provider-level spend limits as an additional control. A model call is reserved durably before the network request; failed/uncertain attempts retain their reservation. At most three model calls and four read-only research calls occur per question. There is no automatic retry after uncertain provider failure.

The model receives the selected case intake, latest three notes, checklist and question. It can use only seven read-only ABC and official-source research tools; it has no case mutation, email, calendar-write, payment or filing tools. It cannot retrieve other cases. Requests set `store:false`; this does not by itself establish provider zero data retention. When routed through Cloudflare the app requests no content logging or caching and attaches an opaque tenant hash. Confirm the gateway/account logging and retention configuration separately. Provider tokens are accounted by response and never used as an invoice without reconciliation.

Official API sources: https://developers.openai.com/api/docs/guides/function-calling and https://developers.openai.com/api/docs/guides/conversation-state . Live provider behavior and answer quality still need an approved evaluation before enabling the assistant for customers; tests use a simulated provider and synthetic cases.

## Container and Cloudflare edge

The Dockerfile builds both the hosted server and maintenance CLI. It runs as UID/GID 65532. Prepare a writable encrypted `/data` volume and a read-only `/run/secrets/permits-keys.json`, accessible only to that identity. Supply the public HTTPS `--origin`; the default localhost origin is intentionally inappropriate for a public deployment. Pin base image digests during the actual deployment and scan the resulting image. Docker is not installed in the current development environment, so the container build remains an explicit deployment gate.

The native origin is a **single instance**. Do not run multiple independent replicas over separate databases or place SQLite WAL on unvalidated shared/network filesystems: quotas and case ownership must share the same durable state. A horizontally scaled service needs a transactional shared database design.

`deploy/cloudflare/worker.mjs` forwards requests to a configured HTTPS `ORIGIN_URL` with no caching or redirect following. Set the app origin to the customer-facing domain. Configure a custom Worker route/domain and leave `workers_dev:false`. Restrict the origin with a firewall, Cloudflare Tunnel or Access service authentication. Optional Worker secrets `ORIGIN_ACCESS_CLIENT_ID` and `ORIGIN_ACCESS_CLIENT_SECRET` authenticate to an Access-protected origin; caller-supplied values are stripped. The operator must create/configure those resources; none were deployed by this change.

Do not enable WAF browser challenges on `/mcp` or `/api/*` that agent clients cannot satisfy. Apply appropriate edge request-rate controls. Keep all production secrets in the platform's secret storage, not the checked-in Wrangler example.

## Operations gates before real customer data

- Verify DNS, TLS, origin isolation, request/body/time limits and credential revocation on the deployed host.
- Verify persistent volume ownership, disk encryption, capacity alarms, encrypted off-host backups and restore into an isolated environment. Back up a stopped SQLite database or use a supported online backup; copying only the live `.sqlite` file can omit WAL data.
- Establish and test tenant export/deletion/retention, including backups. There is no automated deletion endpoint in this candidate.
- Publish the actual operator contact, privacy terms, retention schedule, service terms, supported workflows and prices. Current website is a pilot notice, not completed customer terms.
- Run browser/assistive-technology/mobile verification. The development browser tool was blocked by an unavailable administrator policy check; visual QA is not certified.
- Run live MCP client tests and model evaluation using approved synthetic cases. Bearer-capable clients are supported; OAuth-only connector clients require a separate OAuth implementation.
- Record health, source freshness, API error rate, quota rejection rate, model reservations, token consumption and backup/restore evidence. `GET /healthz` is liveness only, not proof that source caches or all dependencies are ready.
- Keep a rollback binary and database backup. Run schema changes against a restored copy before release; current additive tables have no down-migration.
