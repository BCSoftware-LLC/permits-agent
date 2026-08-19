"""ABC news + industry advisory feeds (WordPress RSS)."""

from __future__ import annotations

import os
import urllib.request
import xml.etree.ElementTree as ET
from datetime import datetime
from pathlib import Path

FEEDS = {
    "news": "https://www.abc.ca.gov/feed/",
    "advisories": "https://www.abc.ca.gov/industry-advisories/feed/",
}
ADVISORIES_PAGE = "https://www.abc.ca.gov/industry-advisories/"
CACHE_DIR = Path(os.environ.get("ABC_AGENT_CACHE", Path.home() / ".cache" / "abc-agent"))


def _get(url: str) -> str:
    req = urllib.request.Request(url, headers={"User-Agent": "abc-agent/0.1"})
    return urllib.request.urlopen(req, timeout=30).read().decode("utf-8", "replace")


def _parse_rss(xml_text: str, feed_name: str) -> list[dict]:
    root = ET.fromstring(xml_text)
    items = []
    for item in root.iter("item"):
        def _t(tag: str) -> str:
            el = item.find(tag)
            return (el.text or "").strip() if el is not None else ""

        pub = _t("pubDate")
        try:
            dt = datetime.strptime(pub, "%a, %d %b %Y %H:%M:%S %z")
            published = dt.astimezone().isoformat()
        except ValueError:
            published = pub
        items.append({
            "feed": feed_name,
            "title": _t("title"),
            "link": _t("link"),
            "published": published,
            "summary": _t("description")[:400],
        })
    return items


def _parse_advisories_page(limit: int = 10) -> list[dict]:
    """The industry-advisories 'feed' is actually a comments feed, so parse the
    category page's article list instead (title, link, published date)."""
    import re
    from html import unescape as _un

    html_text = _get(ADVISORIES_PAGE)
    out = []
    for article in re.findall(r'<article[^>]*category-industry-advisory.*?</article>', html_text, re.S)[:limit]:
        m_link = re.search(r'<a href="([^"]+)"[^>]*>([^<]+)</a>', article)
        m_date = re.search(r'class="published">([^<]+)<', article)
        if not m_link:
            continue
        out.append({
            "feed": "advisories",
            "title": _un(m_link.group(2)).strip(),
            "link": m_link.group(1),
            "published": m_date.group(1).strip() if m_date else "",
            "summary": "",
        })
    return out


def fetch_feed(feed: str = "news", limit: int = 10) -> list[dict]:
    if feed not in FEEDS:
        raise ValueError(f"feed must be one of {list(FEEDS)}")
    items = _parse_rss(_get(FEEDS[feed]), feed)[:limit]
    # The advisories RSS URL serves a comments feed (no items) — fall back to page parsing.
    if feed == "advisories" and not items:
        items = _parse_advisories_page(limit)
    return items


def fetch_all(limit: int = 5) -> list[dict]:
    out = []
    for name in FEEDS:
        try:
            out.extend(fetch_feed(name, limit))
        except Exception as e:  # keep one failing feed from killing the rest
            out.append({"feed": name, "title": f"(feed unavailable: {e})", "link": "", "published": "", "summary": ""})
    out.sort(key=lambda x: x.get("published", ""), reverse=True)
    return out[: limit * len(FEEDS)]
