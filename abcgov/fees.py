"""ABC fee information.

The base application/annual fee schedules are published as prose + documents on
the License Fees page (they change annually and carry county surcharges), so the
full fee *calculator* belongs to the document-assembly phase. What we extract
here is the machine-readable surcharge table and the raw fee page text, which is
enough to estimate filing costs and to drive alerts ("fees adjusted Jan 1").

Surcharges extracted live from:
    https://www.abc.ca.gov/licensing/license-fees/
"""

from __future__ import annotations

import os
import re
import urllib.request
from html import unescape
from pathlib import Path

FEES_URL = "https://www.abc.ca.gov/licensing/license-fees/"
CACHE_DIR = Path(os.environ.get("ABC_AGENT_CACHE", Path.home() / ".cache" / "abc-agent"))
CACHE_FILE = CACHE_DIR / "fees.json"


def fetch_fees(use_cache: bool = True) -> dict:
    """Return {surcharges: [...], page_text_excerpt: str}."""
    if use_cache and CACHE_FILE.exists():
        try:
            return json_load(CACHE_FILE)
        except Exception:
            pass

    req = urllib.request.Request(FEES_URL, headers={"User-Agent": "abc-agent/0.1"})
    html = urllib.request.urlopen(req, timeout=30).read().decode("utf-8", "replace")

    # Surcharge table: rows of (Surcharge Name | Purpose | Amount | Applies to)
    surcharges = []
    for row in re.findall(r"<tr[^>]*>(.*?)</tr>", html, re.S):
        cells = [unescape(re.sub(r"<[^>]+>", "", c)).strip()
                 for c in re.findall(r"<t[dh][^>]*>(.*?)</t[dh]>", row, re.S)]
        if len(cells) >= 3 and cells[0] and cells[0] != "Surcharge Name":
            surcharges.append({
                "name": cells[0],
                "purpose": cells[1] if len(cells) > 1 else "",
                "amount": cells[2] if len(cells) > 2 else "",
                "applies_to": cells[3] if len(cells) > 3 else "",
            })

    body = re.sub(r"<script.*?</script>|<style.*?</style>", "", html, flags=re.S)
    text = re.sub(r"\s+", " ", unescape(re.sub(r"<[^>]+>", " ", body))).strip()

    result = {"surcharges": surcharges, "page_text_excerpt": text[:3000]}
    CACHE_DIR.mkdir(parents=True, exist_ok=True)
    CACHE_FILE.write_text(json_dumps(result, indent=2))
    return result


def json_load(p: Path):
    import json
    with open(p) as f:
        return json.load(f)


def json_dumps(o, indent=None):
    import json
    return json.dumps(o, indent=indent)
