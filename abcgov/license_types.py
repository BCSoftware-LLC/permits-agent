"""ABC license type reference — parsed from https://www.abc.ca.gov/licensing/license-types/

The page renders each license type as an expandable toggle with the format
    "NN - License Name" + description (citing BPC/CCR sections).
We scrape it once, cache the result as JSON, and fall back to a bundled map
for the most common retail license types if the page is unreachable.
"""

from __future__ import annotations

import json
import os
import re
import urllib.request
from html import unescape
from pathlib import Path

TYPES_URL = "https://www.abc.ca.gov/licensing/license-types/"
CACHE_DIR = Path(os.environ.get("ABC_AGENT_CACHE", Path.home() / ".cache" / "abc-agent"))
CACHE_FILE = CACHE_DIR / "license_types.json"

# Fallback for the most common retail codes (used only if the live page fails).
FALLBACK = {
    "01": "Beer Manufacturer", "02": "Winegrower", "03": "Winegrower (wine blender)",
    "05": "Winegrower (grape grower)", "06": "Brandy Manufacturer",
    "07": "Rectifier", "08": "Distilled Spirits Manufacturer",
    "09": "Distilled Spirits Manufacturer (craft distiller)",
    "17": "Beer and Wine Importer", "18": "Distilled Spirits Importer",
    "20": "Off-Sale Beer and Wine", "21": "Off-Sale General",
    "23": "Small Beer Manufacturer", "40": "On-Sale Beer (eating place)",
    "41": "On-Sale Beer and Wine (eating place)", "42": "On-Sale Beer and Wine (public premises)",
    "43": "On-Sale Beer (public premises)", "47": "On-Sale General (eating place)",
    "48": "On-Sale General (public premises)", "49": "On-Sale General (special event)",
    "58": "On-Sale General (catering)", "61": "Off-Sale General (wine)",
    "62": "On-Sale General (small beer manufacturer)", "64": "On-Sale General (wine)",
    "68": "Off-Sale General (beer and wine, grocery store)",
    "77": "On-Sale General (durable special events)",
    "86": "On-Sale General (seasonal)",
    "87": "On-Sale General (airline, passenger)", "88": "On-Sale General (railroad)",
    "89": "On-Sale General (steamship)", "90": "On-Sale General (ocean-going vessel)",
    "91": "On-Sale General (airport, restaurant)", "92": "On-Sale General (airport, bar)",
}


def fetch_license_types(use_cache: bool = True) -> dict[str, str]:
    """Return {code: description} for every ABC license type."""
    if use_cache and CACHE_FILE.exists():
        try:
            return json.loads(CACHE_FILE.read_text())
        except Exception:
            pass

    req = urllib.request.Request(TYPES_URL, headers={"User-Agent": "abc-agent/0.1"})
    html = urllib.request.urlopen(req, timeout=30).read().decode("utf-8", "replace")

    result: dict[str, str] = {}
    # Each type is a div.et_pb_toggle. NOTE: the div's id= is a sequential
    # toggle counter, NOT the license type code — the code is only in the h3
    # title ("20 - Off-Sale Beer & Wine"). Split into per-toggle chunks, then
    # parse code+name from the title and description from the content div.
    starts = [m.start() for m in re.finditer(
        r'<div id="\d+"[^>]*class="[^"]*et_pb_toggle[^"]*"', html)]
    for idx, pos in enumerate(starts):
        end = starts[idx + 1] if idx + 1 < len(starts) else len(html)
        chunk = html[pos:end]
        title = re.search(r'<h3[^>]*>(.*?)</h3>', chunk, re.S)
        if not title:
            continue
        title_text = unescape(re.sub(r"<[^>]+>", "", title.group(1))).strip()
        m = re.match(r"^(\d+)\s*[-–]\s*(.+)$", title_text)
        if not m:
            continue
        code, name = m.group(1), m.group(2).strip()
        content = re.search(r'<div class="et_pb_toggle_content[^"]*">(.*)$', chunk, re.S)
        desc = ""
        if content:
            desc = unescape(re.sub(r"<[^>]+>", " ", content.group(1)))
            desc = re.sub(r"\s+", " ", desc).strip()
        result[code] = f"{name} — {desc}" if desc else name

    # Fall back to the simpler regex if the toggle pattern missed everything.
    if not result:
        for code, name, desc in re.findall(
            r'<div id="(\d+)"[^>]*>.*?<h3[^>]*>(.*?)</h3>.*?<div class="et_pb_toggle_content[^"]*">(.*?)</div>',
            html, re.S,
        ):
            name = unescape(re.sub(r"<[^>]+>", "", name)).strip()
            name = re.sub(r"^\d+\s*-\s*", "", name).strip()
            result[code] = name

    if not result:
        result = dict(FALLBACK)

    CACHE_DIR.mkdir(parents=True, exist_ok=True)
    CACHE_FILE.write_text(json.dumps(result, indent=2))
    return result


def describe(code: str, types: dict[str, str] | None = None) -> str:
    types = types or fetch_license_types()
    return types.get(code, f"License type {code} (not in reference)")


def short(code: str, types: dict[str, str] | None = None, max_len: int = 60) -> str:
    """Short human label for a license type code (name only, truncated)."""
    full = describe(code, types)
    if " — " in full:
        full = full.split(" — ", 1)[0]
    return full[:max_len]


def table() -> list[dict]:
    return [{"code": k, "description": v} for k, v in sorted(fetch_license_types().items())]
