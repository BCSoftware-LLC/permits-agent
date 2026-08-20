"""Hosted MCP endpoint for abc-agent (the paid surface).

Exposes the same FastMCP tool set over HTTP (streamable-http) with API-key
auth and plan-gated tool access — the enforceable paywall. The CLI stays the
free/dev face; this server is what agents subscribe to.

Run locally:
    ABC_API_KEYS="test-free-key:free,test-pro-key:pro" \
        python -m abcgov.http_server          # -> http://localhost:8000

Deployment:
    - Any ASGI host (Fly.io, AWS Lambda via mangum, ECS). The mirror is the
      DuckDB file at ABC_MIRROR_PATH (default: ~/.cache/abc-agent/abc.duckdb);
      refresh it nightly with `abc-agent refresh` (the history moat lives in
      the same file and survives rebuilds).
    - Cloudflare edge (x402/Monetization Gateway) can sit in front per
      docs/payments/cloudflare-bridge.md — the origin stays unchanged.
    - Stripe webhooks (docs/payments/stripe-aws.md) write entitlements; v1
      auth source of truth is the API-keys file below.

API keys: comma-separated TOKEN:PLAN in $ABC_API_KEYS, or a JSON file
{"token": "plan"} in $ABC_API_KEYS_FILE. Plans:
    free    — public lookups (search/get/types/forms/news/stats/statuses)
    pro     — everything incl. monitoring radar, area/address queries,
              document requirements (license_requirements), refresh

Health: GET /healthz (no auth) -> {"status": "ok", "mirror_rows": N}
MCP:    any path below /mcp (Bearer token required)
"""

from __future__ import annotations

import json
import os
from pathlib import Path
from typing import Optional

from fastmcp import FastMCP

from . import db
from .server import mcp  # the canonical tool set (15 tools + resources)

FREE_TOOLS = {
    "search_licenses", "get_license", "license_type_description",
    "search_forms", "latest_news", "fee_surcharges", "license_stats",
    "status_overview",
}
PRO_TOOLS = FREE_TOOLS | {
    "pending_applications", "expiring_licenses", "overdue_licenses",
    "licenses_in_area", "licenses_at_address", "license_requirements",
    "refresh_data",
}

PLAN_TOOLS = {"free": FREE_TOOLS, "pro": PRO_TOOLS}
DEFAULT_PLAN = "pro"


def load_keys() -> dict[str, str]:
    """Return {token: plan} from env (ABC_API_KEYS or ABC_API_KEYS_FILE)."""
    keys: dict[str, str] = {}
    if os.environ.get("ABC_API_KEYS_FILE"):
        data = json.loads(Path(os.environ["ABC_API_KEYS_FILE"]).read_text())
        keys.update({str(k): str(v) for k, v in data.items()})
    if os.environ.get("ABC_API_KEYS"):
        for pair in os.environ["ABC_API_KEYS"].split(","):
            if ":" in pair:
                tok, plan = pair.split(":", 1)
                keys[tok.strip()] = plan.strip()
    return keys


class BearerAuthMiddleware:
    """ASGI middleware: require a valid Bearer API key for /mcp; gate
    tools/call by plan. Fail closed: unknown key -> 401, tool outside the
    key's plan -> 403 with an upgrade pointer."""

    def __init__(self, app, keys: dict[str, str]):
        self.app = app
        self.keys = keys

    async def __call__(self, scope, receive, send):
        if scope["type"] != "http":
            return await self.app(scope, receive, send)

        path = scope.get("path", "")

        if path in ("/healthz", "/health"):
            try:
                rows = db.ABCClient().snapshot_info().row_count
            except Exception:  # noqa: BLE001
                rows = None
            body = json.dumps({"status": "ok", "mirror_rows": rows}).encode()
            await _raw(send, 200, body)
            return

        # Buffer the request body once so auth can inspect it and the app
        # still receives it. After the buffered body is replayed once,
        # subsequent receives pass through to the real channel (the app
        # awaits the http.disconnect to clean up its session) — replaying
        # the body forever is what wedges FastMCP's session cleanup.
        body = b""
        while True:
            message = await receive()
            if message["type"] != "http.request":
                break
            body += message.get("body", b"")
            if not message.get("more_body", False):
                break

        token = None
        for name, value in scope.get("headers", []):
            if name.lower() == b"authorization":
                if value.lower().startswith(b"bearer "):
                    token = value[7:].decode()
                break

        plan = self.keys.get(token or "")
        if plan is None:
            await _raw(send, 401, json.dumps({
                "error": "invalid_api_key",
                "message": "Provide a valid API key via 'Authorization: Bearer <key>'",
            }).encode())
            return

        # Plan-gate tool invocations.
        if b'"tools/call"' in body:
            try:
                req = json.loads(body)
                tool = (req.get("params") or {}).get("name")
            except (json.JSONDecodeError, AttributeError):
                tool = None
            if tool and tool not in PLAN_TOOLS.get(plan, set()):
                await _raw(send, 403, json.dumps({
                    "error": "ERR_ENTITLEMENT_REQUIRED",
                    "message": (f"tool '{tool}' requires the pro plan — "
                                f"your key '{plan}' covers: "
                                f"{', '.join(sorted(PLAN_TOOLS.get(plan, set())))}"),
                }).encode())
                return

        replayed = False

        async def receive_replay():
            nonlocal replayed
            if not replayed:
                replayed = True
                return {"type": "http.request", "body": body, "more_body": False}
            return await receive()

        await self.app(scope, receive_replay, send)


async def _raw(send, status: int, body: bytes) -> None:
    await send({"type": "http.response.start",
                "status": status,
                "headers": [(b"content-type", b"application/json"),
                            (b"content-length", str(len(body)).encode())]})
    await send({"type": "http.response.body", "body": body})


def build_app():
    keys = load_keys()
    transport = os.environ.get("ABC_MCP_TRANSPORT", "streamable-http")
    if transport not in ("http", "streamable-http", "sse"):
        raise ValueError(f"ABC_MCP_TRANSPORT must be http|streamable-http|sse, got {transport!r}")
    wrapped = BearerAuthMiddleware(mcp.http_app(
        path="/mcp",
        transport=transport,  # type: ignore[arg-type]
        allowed_hosts=None,
    ), keys)
    return wrapped


def main() -> None:
    import uvicorn  # noqa: PLC0415
    port = int(os.environ.get("PORT", "8000"))
    print(f"abc-agent hosted MCP on :{port} (path /mcp, health /healthz)")
    print(f"plans: free={len(FREE_TOOLS)} tools, pro={len(PRO_TOOLS)} tools")
    uvicorn.run(build_app(), host="0.0.0.0", port=port)


if __name__ == "__main__":
    main()
