"""abc-agent CLI — compound commands over the California ABC public data.

Usage:
    abc-agent refresh                      # download + rebuild the local mirror
    abc-agent search "dream hollywood"
    abc-agent get 00677768
    abc-agent pending --zip 90028
    abc-agent expiring --days 60 --city "LOS ANGELES"
    abc-agent by --zip 90028 --status ACTIVE --type 47
    abc-agent stats
    abc-agent types [--code 47]
    abc-agent forms [--search "application"]
    abc-agent news [--feed advisories]
    abc-agent serve                      # run the MCP server over stdio

All commands accept --json for machine-readable output.
"""

from __future__ import annotations

import json
import sys
from datetime import date

import typer

from . import db, forms as forms_mod, license_types, news as news_mod

app = typer.Typer(add_completion=False, help=__doc__.splitlines()[0], no_args_is_help=True)
json_opt = typer.Option(False, "--json", help="Emit raw JSON instead of a table.")


def _out(data, as_json: bool, table_fn=None):
    if as_json:
        print(json.dumps(data, indent=2, default=str))
    else:
        (table_fn or (lambda: print(json.dumps(data, indent=2, default=str)) ))()


def _client():
    return db.ensure_data()


def _clean(rows: list[dict]) -> list[dict]:
    out = []
    for r in rows:
        out.append({
            "type": r.get("license_type"),
            "type_desc": license_types.short(str(r.get("license_type", "")).strip()),
            "file_number": r.get("file_number"),
            "status": r.get("status"),
            "lic_or_app": r.get("lic_or_app"),
            "name": r.get("primary_name"),
            "dba": r.get("dba_name"),
            "address": ", ".join(x for x in [r.get("prem_addr1"), r.get("prem_city"),
                                             r.get("prem_state"), r.get("prem_zip")] if x and str(x).strip() not in ("", " ")),
            "county": r.get("prem_county"),
            "district": r.get("district"),
            "expire": r.get("expire_date"),
            "orig_issue": r.get("orig_issue_date"),
        })
    return out


def _table(rows: list[dict], cols: list[str] | None = None) -> None:
    if not rows:
        print("(no results)")
        return
    cols = cols or list(rows[0].keys())
    widths = {c: max(len(c), *(len(str(r.get(c, ""))) for r in rows)) for c in cols}
    print("  ".join(c.ljust(widths[c]) for c in cols))
    print("  ".join("-" * widths[c] for c in cols))
    for r in rows:
        print("  ".join(str(r.get(c, "")).ljust(widths[c]) for c in cols))


@app.command()
def refresh():
    """Download the latest daily export and rebuild the local mirror."""
    client = _client()
    info = client.snapshot_info()
    print(f"Mirror ready: {info.row_count:,} records (export date: {info.csv_date})")


@app.command()
def search(query: str, status: str = None, county: str = None, city: str = None,
           type: str = typer.Option(None, "--type", help="License type code, e.g. 47"),
           limit: int = 20, json: bool = json_opt):
    """Full-text search over licensee name / DBA / file number."""
    rows = _clean(_client().search(query, status=status, county=county, city=city,
                                   license_type=type, limit=limit))
    _out(rows, json, lambda: _table(rows, ["type", "type_desc", "file_number", "status", "name", "dba", "address", "expire"]))


@app.command()
def get(file_number: str, json: bool = json_opt):
    """Look up all records for an ABC file number."""
    recs = _clean(_client().get(file_number))
    if not recs:
        print(f"No record for file number {file_number}")
        raise typer.Exit(1)
    _out(recs, json, lambda: _table(recs))


@app.command()
def by(zip: str = typer.Option(None, "--zip"), city: str = None, county: str = None,
       district: str = None, status: str = None, type: str = typer.Option(None, "--type"),
       limit: int = 100, json: bool = json_opt):
    """List licenses/apps in an area (--zip / --city / --county / --district)."""
    client = _client()
    if zip:
        rows = _clean(client.by_area(zip, "zip", status=status, license_type=type, limit=limit))
    elif city:
        rows = _clean(client.by_area(city, "city", status=status, license_type=type, limit=limit))
    elif county:
        rows = _clean(client.by_area(county, "county", status=status, license_type=type, limit=limit))
    elif district:
        rows = _clean(client.by_area(district, "district", status=status, license_type=type, limit=limit))
    else:
        raise typer.BadParameter("provide one of --zip / --city / --county / --district")
    _out(rows, json, lambda: _table(rows, ["type", "type_desc", "file_number", "status", "name", "address", "expire"]))


@app.command()
def pending(zip: str = typer.Option(None, "--zip"), city: str = None, county: str = None,
            limit: int = 100, json: bool = json_opt):
    """Pending applications (new filings in progress) — optional area filter."""
    client = _client()
    if zip:
        rows = _clean(client.pending(zip, "zip", limit=limit))
    elif city:
        rows = _clean(client.pending(city, "city", limit=limit))
    elif county:
        rows = _clean(client.pending(county, "county", limit=limit))
    else:
        rows = _clean(client.pending(limit=limit))
    _out(rows, json, lambda: _table(rows, ["type", "type_desc", "file_number", "name", "address"]))


@app.command()
def expiring(days: int = 90, zip: str = typer.Option(None, "--zip"), city: str = None,
             county: str = None, limit: int = 100, json: bool = json_opt):
    """Active licenses expiring within N days (renewal radar)."""
    client = _client()
    if zip:
        rows = _clean(client.expiring(days, zip, "zip", limit=limit))
    elif city:
        rows = _clean(client.expiring(days, city, "city", limit=limit))
    elif county:
        rows = _clean(client.expiring(days, county, "county", limit=limit))
    else:
        rows = _clean(client.expiring(days, limit=limit))
    _out(rows, json, lambda: _table(rows, ["type", "type_desc", "file_number", "name", "address", "expire"]))


@app.command()
def address(fragment: str, mail: bool = typer.Option(False, "--mail", help="Also match mailing addresses (entity-level)"),
            status: str = None, type: str = typer.Option(None, "--type"),
            limit: int = 100, json: bool = json_opt):
    """Identify every license/application tied to an address (street, number, city, or zip)."""
    rows = _clean(_client().by_address(fragment, match_mail=mail, status=status,
                                       license_type=type, limit=limit))
    _out(rows, json, lambda: _table(rows, ["type", "type_desc", "file_number", "status", "name", "dba", "address", "expire"]))


@app.command()
def statuses(json: bool = json_opt):
    """Full ABC status vocabulary (official glossary) + observed counts + overdue (auto-revocation candidates)."""
    from . import statuses as st
    data = st.status_overview(_client().con)
    _out(data, json)


@app.command()
def overdue(min_days_past: int = typer.Option(0, "--min-days", help="Only licenses past expiry by at least N days"),
            zip: str = typer.Option(None, "--zip"), city: str = None, county: str = None,
            district: str = None, limit: int = 100, json: bool = json_opt):
    """Still-ACTIVE licenses past their expiration date — renewal-failure / auto-revocation candidates."""
    client = _client()
    if zip:
        rows = _clean(client.overdue(min_days_past, zip, "zip", limit=limit))
    elif city:
        rows = _clean(client.overdue(min_days_past, city, "city", limit=limit))
    elif county:
        rows = _clean(client.overdue(min_days_past, county, "county", limit=limit))
    elif district:
        rows = _clean(client.overdue(min_days_past, district, "district", limit=limit))
    else:
        rows = _clean(client.overdue(min_days_past, limit=limit))
    _out(rows, json, lambda: _table(rows, ["type", "type_desc", "file_number", "name", "address", "expire"]))


@app.command()
def stats(json: bool = json_opt):
    """Snapshot summary of the mirror."""
    data = _client().stats()
    _out(data, json)


@app.command()
def types(code: str = None, json: bool = json_opt):
    """License type reference (codes + descriptions from ABC)."""
    data = license_types.table()
    if code:
        data = [t for t in data if t["code"] == code]
    if not data:
        print(f"Unknown type code: {code}")
        raise typer.Exit(1)
    _out(data, json, lambda: _table(data, ["code", "description"]))


@app.command()
def forms(search: str = None, json: bool = json_opt):
    """ABC forms library index (87 forms with PDF URLs)."""
    data = forms_mod.search_forms(search) if search else forms_mod.fetch_forms()
    _out(data, json, lambda: _table(data, ["number", "title", "revised", "url"]))


@app.command()
def news(feed: str = typer.Option("news", "--feed", help="news | advisories"), limit: int = 8, json: bool = json_opt):
    """Latest ABC news releases or industry advisories."""
    data = news_mod.fetch_feed(feed, limit)
    _out(data, json, lambda: _table([{k: (v[:60] if isinstance(v, str) else v) for k, v in d.items()} for d in data],
                                    ["published", "title", "link"]))


@app.command()
def serve():
    """Run the MCP server over stdio (for Hermes/Claude/agents)."""
    from .server import mcp
    mcp.run()


if __name__ == "__main__":
    app()
