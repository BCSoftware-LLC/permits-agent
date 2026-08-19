"""ABC forms library — indexed from https://www.abc.ca.gov/licensing/license-forms/

The page contains a table (#all-forms-table) of every ABC form:
    Form (PDF link) | Revised Date | Description
All PDFs live at predictable URLs under https://www.abc.ca.gov/wp-content/uploads/forms/
"""

from __future__ import annotations

import json
import os
import re
import urllib.request
from html import unescape
from pathlib import Path

FORMS_URL = "https://www.abc.ca.gov/licensing/license-forms/"
FORMS_DIR = "https://www.abc.ca.gov/wp-content/uploads/forms/"
CACHE_DIR = Path(os.environ.get("ABC_AGENT_CACHE", Path.home() / ".cache" / "abc-agent"))
CACHE_FILE = CACHE_DIR / "forms_index.json"


def fetch_forms(use_cache: bool = True) -> list[dict]:
    """Return [{number, title, revised, url}] for every form on the page."""
    if use_cache and CACHE_FILE.exists():
        try:
            return json.loads(CACHE_FILE.read_text())
        except Exception:
            pass

    req = urllib.request.Request(FORMS_URL, headers={"User-Agent": "abc-agent/0.1"})
    html = urllib.request.urlopen(req, timeout=30).read().decode("utf-8", "replace")

    forms: list[dict] = []
    rows = re.findall(r"<tr>\s*<th><a href=\"([^\"]+)\">([^<]+)</a></th>\s*<td>(.*?)</td>\s*<td>(.*?)</td>\s*</tr>", html, re.S)
    for url, number, revised, title in rows:
        forms.append({
            "number": unescape(number).strip(),
            "title": unescape(re.sub(r"<[^>]+>", "", title)).strip(),
            "revised": unescape(re.sub(r"<[^>]+>", "", revised)).strip(),
            "url": url,
        })

    CACHE_DIR.mkdir(parents=True, exist_ok=True)
    CACHE_FILE.write_text(json.dumps(forms, indent=2))
    return forms


def search_forms(query: str, forms: list[dict] | None = None) -> list[dict]:
    forms = forms if forms is not None else fetch_forms()
    q = query.lower()
    return [f for f in forms if q in f["number"].lower() or q in f["title"].lower()]


def pdf_path(number: str, forms: list[dict] | None = None) -> str | None:
    forms = forms if forms is not None else fetch_forms()
    for f in forms:
        if f["number"].lower() == number.lower():
            return f["url"]
    return None
