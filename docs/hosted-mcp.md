# Hosted MCP Endpoint — abc-agent over HTTP (the paid surface)

Status: implemented (v1 scaffold) · 2026-08-19 · Owner: BC Software LLC
Companion docs: `payments/stripe-aws.md` (human money — subscriptions,
one-time filing fees, metered verification), `payments/cloudflare-bridge.md`
(edge gatekeeper + x402 machine money), `architecture.md` (layers).

---

## 1. What this is

`abcgov/http_server.py` exposes the exact same FastMCP tool set (15 tools,
2 resources — see `server.py`) over HTTP, gated by **API keys + plan
allowlists**. The CLI stays the free/dev face; this server is what paying
agents connect to. Auth is bearer-token, fail-closed:

| HTTP status | Meaning |
|---|---|
| 401 | missing/invalid API key (`invalid_api_key`) |
| 403 | key valid but tool outside its plan (`ERR_ENTITLEMENT_REQUIRED`, with an upgrade pointer) |
| 200+ | through the gate — FastMCP handles the MCP protocol from there |

Plans (v1): `free` = public lookups (search/get/types/forms/news/stats/
statuses) · `pro` = everything, incl. monitoring radar (pending/expiring/
overdue), area & address queries, `license_requirements` (document packets),
`refresh_data`.

## 2. Topology (v1 scaffold → production)

```
MCP clients (Claude Code, Hermes, any agent)
        │  streamable-http + Bearer <key>
        ▼
┌───────────────────────────────┐
│ Cloudflare edge (LATER)       │  x402-proxy / Monetization Gateway —
│                               │  per-call machine payments at the edge,
│                               │  origin unchanged (cloudflare-bridge.md)
└──────────────┬────────────────┘
               ▼
┌───────────────────────────────┐
│ Host: ASGI server             │  uvicorn (local / Fly.io / ECS)
│  abcgov/http_server.py        │  BearerAuthMiddleware → FastMCP http_app
│  BearerAuthMiddleware         │  (streamable-http, session-based)
└──────────────┬────────────────┘
               ▼
┌───────────────────────────────┐
│ abcgov/server.py (FastMCP)    │  same engine as stdio `abc-agent serve`
│ DuckDB mirror (~/.cache/…)    │  history table (the moat) in same file
└───────────────────────────────┘
```

- **Mirror sync**: nightly `abc-agent refresh` + `history-snapshot` (cron
  exists on the dev box; in production, a scheduled task rebuilds the DuckDB
  file and the server hot-reloads or restarts). The history moat accumulates
  in the same file.
- **Stripe (stripe-aws.md)**: webhooks write entitlements (DynamoDB per that
  doc). v1 uses the API-keys file as the auth source of truth; the
  entitlement service plugs in behind `load_keys()` when deployed.
- **Cloudflare (cloudflare-bridge.md)**: Phase 3 — edge gating for per-call
  x402 payments, Web Bot Auth for org attribution. No origin change.

## 3. Running & configuring

```bash
# local
ABC_API_KEYS="sk_live_demo_free:free,sk_live_demo_pro:pro" \
  python -m abcgov.http_server            # :8000, /mcp + /healthz
```

- `ABC_API_KEYS` — comma-separated `TOKEN:PLAN` pairs, or
  `ABC_API_KEYS_FILE` — JSON `{"token": "plan"}`.
- `ABC_MCP_TRANSPORT` — `streamable-http` (default) | `http` | `sse`.
  Default is streamable-http (the modern spec; works with the TS SDK).
- `PORT` — listen port.
- `ABC_MIRROR_PATH` — DuckDB mirror location (default `~/.cache/abc-agent/`).

Deployment targets:
- **Fly.io / ECS (Fargate)**: run the ASGI app directly; attach the mirror as
  a volume or pull from object storage at boot. Simplest v1.
- **AWS Lambda**: wrap `build_app()` with `mangum`; sessions are in-memory,
  so keep warm concurrency or accept cold-start per session. The
  `payments/stripe-aws.md` stack already assumes Lambda — revisit when the
  entitlement service lands.

## 4. Testing

- `tests/http_smoke.py` — self-contained: boots the server with test keys,
  asserts healthz/401/403/middleware-pass. Run: `.venv/bin/python
  tests/http_smoke.py`.
- **Canonical protocol E2E** (the one that matters): the official TypeScript
  SDK client against a running server. Verified 2026-08-19 — 15 tools listed,
  `pending_applications` + `license_requirements` return data, free key is
  blocked from pro tools. Probe: `node /tmp/mcp-probe/probe.mjs` (npm i
  `@modelcontextprotocol/sdk`).
- `tests/smoke_mcp.py` — stdio engine regression (unchanged).

## 5. Known issues (honest list)

1. **Python `mcp` SDK 1.29 HTTP client is broken for streamable-http** (works
   fine over stdio). The server passes the canonical TS SDK client end-to-end,
   so the server is protocol-correct; the Python client helper chokes on its
   own session teardown (`anyio.ClosedResourceError` after initialize) across
   all three transports. Mitigation: use the TS SDK / official inspector /
   Claude Code for HTTP connections; the Python SDK remains fine for stdio.
   Re-test when mcp SDK ≥1.30 ships.
2. **FastMCP ASGI middleware footgun**: if you wrap `mcp.http_app()` in auth
   middleware that buffers the request body, the replay must pass the real
   `receive` through after one buffered chunk — replaying the body forever
   wedges FastMCP's session cleanup (the event loop stops serving other
   requests). `BearerAuthMiddleware` implements the correct replay-once
   pattern; keep it if you rewrite.
3. **Transport semantics**: `ABC_MCP_TRANSPORT=http` (stateless JSON) is
   stricter about `Accept: application/json, text/event-stream` and closes
   sessions after initialize — prefer the default streamable-http.
4. **Auth source of truth** is the env/file keys until the Stripe
   entitlement service lands; no key rotation, no per-account DB. Fine for
   pilots; replace with DynamoDB entitlements + webhook-written keys before
   GA (stripe-aws.md §2).

## 6. Roll-out sequence (ties into payments docs)

| Step | What | Gate |
|---|---|---|
| 1 | Deploy ASGI app on Fly.io/ECS with mirror volume; nightly refresh cron | Pilot customer |
| 2 | Stripe Phase 0–1 (test mode): Checkout + webhooks → entitlements store; `load_keys()` reads entitlements | Test-mode subscription round-trips |
| 3 | Cloudflare edge: x402-proxy for per-call verification pricing | Phase 1 spike per cloudflare-bridge.md |
| 4 | Public docs: MCP registration snippet with `url` + header auth | GA |
