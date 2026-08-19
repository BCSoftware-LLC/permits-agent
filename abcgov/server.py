"""abc-agent MCP server — exposes California ABC public data as agent tools.

Run over stdio:
    abc-agent serve
or directly:
    python -m abcgov.server

Tools let any agent (Hermes, Claude Code, etc.) search licenses, monitor
pending applications, watch expirations, pull forms and news — all from the
local daily mirror, no scraping, no rate limits.
"""

from __future__ import annotations

import os
from datetime import date, timedelta

from fastmcp import FastMCP

from . import db, fees as fees_mod, forms as forms_mod, license_types, news as news_mod

mcp = FastMCP("abc-agent", instructions=(
    "California Department of Alcoholic Beverage Control (ABC) data gateway. "
    "Queries run against a daily-synced local mirror of ABC's official public "
    "license export (~129k records). Use search_licenses for names/DBAs, "
    "get_license for a file number, pending_applications to find new filings, "
    "expiring_licenses for renewal radar, and license_stats for the big picture."
))

_client: db.ABCClient | None = None


def client() -> db.ABCClient:
    global _client
    if _client is None:
        _client = db.ensure_data()
    return _client


@mcp.tool()
def search_licenses(query: str, status: str | None = None, county: str | None = None,
                    city: str | None = None, license_type: str | None = None,
                    lic_or_app: str | None = None, expire_year: str | None = None,
                    limit: int = 50) -> list[dict]:
    """Search ABC licenses/applications by licensee name, DBA, or file number.
    Filters: status (ACTIVE/PEND/SUREND/...), county, city, license type code,
    lic_or_app (LIC=issued / APP=application), expire_year (e.g. '2026' for
    records whose expiration falls in that calendar year)."""
    return _clean(client().search(query, status=status, county=county, city=city,
                                  license_type=license_type, lic_or_app=lic_or_app,
                                  expire_year=expire_year, limit=min(limit, 200)))


@mcp.tool()
def get_license(file_number: str) -> list[dict]:
    """Fetch all records for an ABC file number (e.g. 00677768); a file number
    may have one line per license type."""
    return _clean(client().get(file_number))


@mcp.tool()
def pending_applications(zip: str | None = None, city: str | None = None, county: str | None = None,
                         limit: int = 100) -> list[dict]:
    """New ABC applications still in progress (status PEND). Filter by zip/city/county
    to see fresh filings in a territory — useful for lead gen and competitor watch."""
    c = client()
    if zip:
        return _clean(c.pending(zip, "zip", limit=limit))
    if city:
        return _clean(c.pending(city, "city", limit=limit))
    if county:
        return _clean(c.pending(county, "county", limit=limit))
    return _clean(c.pending(limit=limit))


@mcp.tool()
def expiring_licenses(days: int = 90, zip: str | None = None, city: str | None = None,
                      county: str | None = None, limit: int = 100) -> list[dict]:
    """Active licenses whose expiration is within `days` (renewal radar)."""
    c = client()
    if zip:
        return _clean(c.expiring(days, zip, "zip", limit=limit))
    if city:
        return _clean(c.expiring(days, city, "city", limit=limit))
    if county:
        return _clean(c.expiring(days, county, "county", limit=limit))
    return _clean(c.expiring(days, limit=limit))


@mcp.tool()
def licenses_at_address(fragment: str, match_mail: bool = False, status: str | None = None,
                        license_type: str | None = None, lic_or_app: str | None = None,
                        expire_year: str | None = None, limit: int = 100) -> list[dict]:
    """Identify every license/application tied to an address. `fragment` can be a
    street name, street + number, city, or zip. With match_mail=True also matches
    mailing addresses (entity-level identification — every license whose owner
    files from the same mail address). Filters: status, license_type, lic_or_app,
    expire_year."""
    return _clean(client().by_address(fragment, match_mail=match_mail, status=status,
                                      license_type=license_type, lic_or_app=lic_or_app,
                                      expire_year=expire_year, limit=limit))


@mcp.tool()
def overdue_licenses(min_days_past: int = 0, zip: str | None = None, city: str | None = None,
                     county: str | None = None, district: str | None = None,
                     limit: int = 100) -> list[dict]:
    """Still-ACTIVE licenses past their expiration date — renewal-failure /
    auto-revocation candidates (ABC auto-revokes for unpaid annual fees).
    Optional area filter by zip/city/county/district."""
    c = client()
    if zip:
        return _clean(c.overdue(min_days_past, zip, "zip", limit=limit))
    if city:
        return _clean(c.overdue(min_days_past, city, "city", limit=limit))
    if county:
        return _clean(c.overdue(min_days_past, county, "county", limit=limit))
    if district:
        return _clean(c.overdue(min_days_past, district, "district", limit=limit))
    return _clean(c.overdue(min_days_past, limit=limit))


@mcp.tool()
def status_overview() -> dict:
    """Full ABC status vocabulary (official LQS glossary: ACTIVE, PEND, REV, REVP,
    NREN, WDRL, …) plus the statuses currently observed in the export and the
    count of overdue (auto-revocation candidate) licenses."""
    from . import statuses as st
    return st.status_overview(client().con)


@mcp.tool()
def licenses_in_area(zip: str | None = None, city: str | None = None, county: str | None = None,
                     district: str | None = None, status: str | None = None,
                     license_type: str | None = None, lic_or_app: str | None = None,
                     expire_year: str | None = None,
                     limit: int = 200) -> list[dict]:
    """List licenses/applications in a zip, city, county, or ABC district.
    Filters: status, license_type, lic_or_app, expire_year."""
    c = client()
    if zip:
        return _clean(c.by_area(zip, "zip", status=status, license_type=license_type,
                                lic_or_app=lic_or_app, expire_year=expire_year, limit=limit))
    if city:
        return _clean(c.by_area(city, "city", status=status, license_type=license_type,
                                lic_or_app=lic_or_app, expire_year=expire_year, limit=limit))
    if county:
        return _clean(c.by_area(county, "county", status=status, license_type=license_type,
                                lic_or_app=lic_or_app, expire_year=expire_year, limit=limit))
    if district:
        return _clean(c.by_area(district, "district", status=status, license_type=license_type,
                                lic_or_app=lic_or_app, expire_year=expire_year, limit=limit))
    return []


@mcp.tool()
def license_stats() -> dict:
    """Aggregate summary: active licenses, pending applications, status/type distribution."""
    return client().stats()


@mcp.tool()
def license_type_description(code: str) -> str:
    """Describe an ABC license type code (e.g. 47 = On-Sale General, eating place)."""
    return license_types.describe(code)


@mcp.tool()
def search_forms(query: str) -> list[dict]:
    """Find ABC forms (e.g. 'application', 'transfer', 'ABC-211') with PDF URLs."""
    return forms_mod.search_forms(query)


@mcp.tool()
def latest_news(feed: str = "news", limit: int = 8) -> list[dict]:
    """Latest ABC news releases ('news') or industry advisories ('advisories')."""
    return news_mod.fetch_feed(feed, limit)


@mcp.tool()
def fee_surcharges() -> list[dict]:
    """Current statutory surcharges applied on top of base license fees (Appeals Board 3%, CHP, etc.)."""
    return fees_mod.fetch_fees().get("surcharges", [])


@mcp.tool()
def refresh_data() -> str:
    """Force a refresh of the daily export mirror. Returns record count loaded."""
    c = db.ensure_data(force=True)
    return f"Mirror refreshed: {c.snapshot_info().row_count:,} records " \
           f"(export {c.snapshot_info().csv_date})"


@mcp.resource("abc://license-types")
def _types_resource() -> str:
    return "\n".join(f"{t['code']}: {t['description']}" for t in license_types.table()[:60])


@mcp.resource("abc://stats")
def _stats_resource() -> str:
    import json as _json
    return _json.dumps(client().stats(), indent=2)


def _clean(rows: list[dict]) -> list[dict]:
    out = []
    for r in rows:
        out.append({
            "license_type": r.get("license_type"),
            "type_description": license_types.describe(str(r.get("license_type", "")).strip()),
            "file_number": r.get("file_number"),
            "status": r.get("status"),
            "lic_or_app": r.get("lic_or_app"),
            "name": r.get("primary_name"),
            "dba": r.get("dba_name"),
            "premises": ", ".join(x for x in [r.get("prem_addr1"), r.get("prem_city"),
                                              r.get("prem_state"), r.get("prem_zip")] if x and str(x).strip() not in ("", " ")),
            "county": r.get("prem_county"),
            "district": r.get("district"),
            "expires": r.get("expire_date"),
            "originally_issued": r.get("orig_issue_date"),
        })
    return out


def main():
    mcp.run()


if __name__ == "__main__":
    main()
