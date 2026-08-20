"""Self-contained smoke test for the hosted HTTP endpoint (abcgov/http_server.py).

Starts the server on a random port with test API keys, then verifies:
    /healthz 200 · no-key 401 · free-key on pro tool 403 · free-key on free
    tool passes middleware. The full streamable-http handshake is covered by
    the canonical TS SDK probe (see docs/hosted-mcp.md §Testing) — the Python
    mcp SDK 1.29 HTTP client has a known streamable-http bug (works over
    stdio), so this test deliberately stays at the HTTP/auth layer.

Run:  .venv/bin/python tests/http_smoke.py
"""
import json
import os
import socket
import subprocess
import sys
import time
import urllib.request
import urllib.error

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
PY = os.path.join(REPO, ".venv", "bin", "python")
KEYS = "free-token:free,pro-token:pro"


def free_port() -> int:
    with socket.socket() as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


def http_json(method: str, url: str, body: dict | None = None,
              token: str | None = None) -> tuple[int, str]:
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(url, data=data, method=method)
    req.add_header("Content-Type", "application/json")
    req.add_header("Accept", "application/json, text/event-stream")
    if token:
        req.add_header("Authorization", f"Bearer {token}")
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            return resp.status, resp.read().decode()
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode()


def main() -> None:
    port = free_port()
    env = dict(os.environ, ABC_API_KEYS=KEYS, ABC_MCP_TRANSPORT="streamable-http",
               PORT=str(port))
    proc = subprocess.Popen([PY, "-m", "abcgov.http_server"], cwd=REPO, env=env,
                            stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    try:
        base = f"http://127.0.0.1:{port}"
        for _ in range(30):
            try:
                code, _ = http_json("GET", f"{base}/healthz")
                if code == 200:
                    break
            except Exception:
                pass
            time.sleep(0.3)
        else:
            raise RuntimeError("server did not come up")

        checks = [
            ("healthz", http_json("GET", f"{base}/healthz")[0], 200),
            ("no-key /mcp -> 401", http_json(
                "POST", f"{base}/mcp",
                {"jsonrpc": "2.0", "id": 1, "method": "tools/call",
                 "params": {"name": "pending_applications", "arguments": {}}})[0], 401),
            ("free-key pro tool -> 403", http_json(
                "POST", f"{base}/mcp",
                {"jsonrpc": "2.0", "id": 1, "method": "tools/call",
                 "params": {"name": "pending_applications", "arguments": {}}},
                token="free-token")[0], 403),
            ("free-key free tool passes middleware (not 401/403)",
             http_json(
                 "POST", f"{base}/mcp",
                 {"jsonrpc": "2.0", "id": 1, "method": "tools/call",
                  "params": {"name": "search_licenses", "arguments": {"query": "x"}}},
                 token="free-token")[0], None),
        ]
        failed = 0
        for label, got, want in checks:
            if want is None:
                ok = got not in (401, 403)
                print(f"  {'PASS' if ok else 'FAIL'}  {label}: {got}")
            else:
                ok = got == want
                print(f"  {'PASS' if ok else 'FAIL'}  {label}: {got} (want {want})")
            failed += 0 if ok else 1
        if failed:
            print(f"http_smoke FAILED ({failed} check(s))")
            sys.exit(1)
        print("http_smoke PASS")
    finally:
        proc.terminate()
        proc.wait(timeout=10)


if __name__ == "__main__":
    main()
