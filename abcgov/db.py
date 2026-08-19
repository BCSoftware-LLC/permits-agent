"""abc-agent: data layer for the California ABC public data exports.

Core idea: the California Department of Alcoholic Beverage Control publishes a
full daily snapshot of every license and pending application as a CSV at:
    https://www.abc.ca.gov/wp-content/uploads/DailyExport-CSV.zip

We download it once per day (cache-aware), load it into DuckDB, and expose
fast local queries — no scraping, no rate limits, no captchas. This powers both
the CLI and the MCP server.
"""

from __future__ import annotations

import io
import os
import re
import time
import urllib.request
import zipfile
from dataclasses import dataclass
from datetime import datetime, date, timedelta
from pathlib import Path
from typing import Iterable, Optional

import duckdb

EXPORT_URL = "https://www.abc.ca.gov/wp-content/uploads/DailyExport-CSV.zip"
CACHE_DIR = Path(os.environ.get("ABC_AGENT_CACHE", Path.home() / ".cache" / "abc-agent"))
DB_PATH = CACHE_DIR / "abc.duckdb"
CSV_PATH = CACHE_DIR / "ABC-DailyDataExport.csv"
CACHE_TTL_SECONDS = int(os.environ.get("ABC_AGENT_TTL", 12 * 3600))  # 12h default

# The export's first line is a human-readable "Updated ..." banner, the second
# line is the real header.
COLS = [
    "license_type", "file_number", "lic_or_app", "status", "orig_issue_date",
    "expire_date", "fee_codes", "dup_counts", "master_ind", "term_months",
    "geo_code", "district", "primary_name", "prem_addr1", "prem_addr2",
    "prem_city", "prem_state", "prem_zip", "dba_name", "mail_addr1",
    "mail_addr2", "mail_city", "mail_state", "mail_zip", "prem_county",
    "census_tract",
]

STATUSES = {"ACTIVE", "PEND", "SUREND", "REVPEN", "SUSPEN", "R64B", "PDEV"}


@dataclass
class SnapshotInfo:
    csv_date: Optional[date]
    loaded_at: Optional[datetime]
    row_count: int
    fresh: bool
    cache_path: Path


def _http_get(url: str, timeout: int = 180) -> bytes:
    req = urllib.request.Request(url, headers={"User-Agent": "abc-agent/0.1 (public-data-client)"})
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        return resp.read()


def _parse_csv_date(path: Path) -> Optional[date]:
    try:
        first = path.read_text(encoding="utf-8-sig", errors="replace").splitlines()[0]
    except Exception:
        return None
    m = re.search(r"(\w+)\s+(\d{1,2})(?:st|nd|rd|th)?\s+of\s+(\w+)\s+(\d{4})", first)
    if not m:
        return None
    try:
        return datetime.strptime(f"{m.group(2)} {m.group(3)} {m.group(4)}", "%d %B %Y").date()
    except ValueError:
        return None


def _cached_csv_is_fresh() -> bool:
    if not CSV_PATH.exists():
        return False
    age = time.time() - CSV_PATH.stat().st_mtime
    return age < CACHE_TTL_SECONDS


def fetch_daily_export(force: bool = False, quiet: bool = True) -> Path:
    """Download (or reuse cached) daily export CSV. Returns path to the CSV."""
    CACHE_DIR.mkdir(parents=True, exist_ok=True)
    if not force and _cached_csv_is_fresh():
        if not quiet:
            print(f"[abc-agent] using cached export ({CSV_PATH.stat().st_size:,} bytes)")
        return CSV_PATH
    if not quiet:
        print("[abc-agent] downloading daily export …")
    data = _http_get(EXPORT_URL)
    with zipfile.ZipFile(io.BytesIO(data)) as zf:
        name = [n for n in zf.namelist() if n.lower().endswith(".csv")][0]
        with zf.open(name) as fh, open(CSV_PATH, "wb") as out:
            out.write(fh.read())
    if not quiet:
        print(f"[abc-agent] saved {CSV_PATH} ({CSV_PATH.stat().st_size:,} bytes)")
    return CSV_PATH


def load_to_duckdb(csv_path: Optional[Path] = None, force: bool = False) -> duckdb.DuckDBPyConnection:
    """Load the CSV into a DuckDB file (re-created when the export is newer)."""
    csv_path = csv_path or CSV_PATH
    if not csv_path.exists():
        raise FileNotFoundError(f"Export not found at {csv_path}; run fetch_daily_export() first")

    need_load = force or not DB_PATH.exists()
    if not need_load:
        # Rebuild if the source CSV is newer than the DB.
        try:
            need_load = csv_path.stat().st_mtime > DB_PATH.stat().st_mtime
        except OSError:
            need_load = True

    con = duckdb.connect(str(DB_PATH))
    if need_load:
        con.execute("DROP TABLE IF EXISTS licenses")
        # Skip the banner line, read header from line 2.
        con.execute(
            f"""
            CREATE TABLE licenses AS
            SELECT * FROM read_csv_auto('{csv_path}', header=true, skip=1, null_padding=true)
            """
        )
        con.execute("""
            ALTER TABLE licenses RENAME COLUMN "License Type" TO license_type;
            ALTER TABLE licenses RENAME COLUMN "File Number" TO file_number;
            ALTER TABLE licenses RENAME COLUMN "Lic or App" TO lic_or_app;
            ALTER TABLE licenses RENAME COLUMN "Type Status" TO status;
            ALTER TABLE licenses RENAME COLUMN "Primary Name" TO primary_name;
            ALTER TABLE licenses RENAME COLUMN "DBA Name" TO dba_name;
            ALTER TABLE licenses RENAME COLUMN "Prem Addr 1" TO prem_addr1;
            ALTER TABLE licenses RENAME COLUMN "Prem Addr 2" TO prem_addr2;
            ALTER TABLE licenses RENAME COLUMN "Prem City" TO prem_city;
            ALTER TABLE licenses RENAME COLUMN "Prem State" TO prem_state;
            ALTER TABLE licenses RENAME COLUMN "Prem Zip" TO prem_zip;
            ALTER TABLE licenses RENAME COLUMN "Prem County" TO prem_county;
            ALTER TABLE licenses RENAME COLUMN "Expir Date" TO expire_date;
            ALTER TABLE licenses RENAME COLUMN "District" TO district;
            ALTER TABLE licenses RENAME COLUMN "Type Orig Iss Date" TO orig_issue_date;
            ALTER TABLE licenses RENAME COLUMN "Mail Addr 1" TO mail_addr1;
            ALTER TABLE licenses RENAME COLUMN "Mail Addr 2" TO mail_addr2;
            ALTER TABLE licenses RENAME COLUMN "Mail City" TO mail_city;
            ALTER TABLE licenses RENAME COLUMN "Mail State" TO mail_state;
            ALTER TABLE licenses RENAME COLUMN "Mail Zip" TO mail_zip;
        """)
        con.execute("CREATE INDEX IF NOT EXISTS idx_file ON licenses(file_number)")
        con.execute("CREATE INDEX IF NOT EXISTS idx_name ON licenses(primary_name)")
        con.execute("CREATE INDEX IF NOT EXISTS idx_status ON licenses(status)")
    return con


class ABCClient:
    """High-level query API over the local DuckDB snapshot."""

    def __init__(self, con: Optional[duckdb.DuckDBPyConnection] = None, db_path: Path = DB_PATH):
        self.con = con or duckdb.connect(str(db_path))

    # ---- freshness ------------------------------------------------------
    def snapshot_info(self) -> SnapshotInfo:
        info = self.con.execute("SELECT count(*) FROM licenses").fetchone()[0]
        return SnapshotInfo(
            csv_date=_parse_csv_date(CSV_PATH),
            loaded_at=datetime.fromtimestamp(DB_PATH.stat().st_mtime) if DB_PATH.exists() else None,
            row_count=info,
            fresh=_cached_csv_is_fresh(),
            cache_path=CSV_PATH,
        )

    # ---- queries ---------------------------------------------------------
    @staticmethod
    def _filters(q: str, args: list, *, status: Optional[str] = None,
                 license_type: Optional[str] = None, lic_or_app: Optional[str] = None,
                 expire_year: Optional[str] = None) -> tuple[str, list]:
        """Append the shared filter vocabulary to a query: status, license type,
        lic_or_app, and expiration calendar year (export format DD-MON-YYYY, so
        expire_year='2026' matches '%-2026'). Blank dates never match a year.
        One vocabulary → CLI and MCP stay consistent on every query surface."""
        if status:
            q += " AND status = ?"
            args.append(status.upper())
        if license_type:
            q += " AND license_type = ?"
            args.append(license_type)
        if lic_or_app:
            q += " AND lic_or_app = ?"
            args.append(lic_or_app.upper())
        if expire_year:
            y = str(expire_year).strip()
            if not (y.isdigit() and len(y) == 4):
                raise ValueError(f"expire_year must be a 4-digit year, got {expire_year!r}")
            q += " AND expire_date LIKE ?"
            args.append(f"%-{y}")
        return q, args

    def search(self, query: str, status: Optional[str] = None, county: Optional[str] = None,
               city: Optional[str] = None, license_type: Optional[str] = None,
               lic_or_app: Optional[str] = None, expire_year: Optional[str] = None,
               limit: int = 50) -> list[dict]:
        q = "WHERE (primary_name ILIKE ? OR dba_name ILIKE ? OR file_number ILIKE ?)"
        args: list = [f"%{query}%", f"%{query}%", f"%{query}%"]
        if county:
            q += " AND prem_county ILIKE ?"
            args.append(f"%{county}%")
        if city:
            q += " AND prem_city ILIKE ?"
            args.append(f"%{city}%")
        q, args = self._filters(q, args, status=status, license_type=license_type,
                                lic_or_app=lic_or_app, expire_year=expire_year)
        q += " ORDER BY primary_name LIMIT ?"
        args.append(limit)
        rows = self.con.execute(f"SELECT * FROM licenses {q}", args).fetchall()
        cols = [d[0] for d in self.con.description]
        return [dict(zip(cols, r)) for r in rows]

    def get(self, file_number: str) -> list[dict]:
        """All records for a file number (one per license type line in the export)."""
        rows = self.con.execute(
            "SELECT * FROM licenses WHERE file_number = ?", [file_number]
        ).fetchall()
        cols = [d[0] for d in self.con.description]
        return [dict(zip(cols, r)) for r in rows]

    def by_area(self, area: str, kind: str = "zip", status: Optional[str] = None,
                license_type: Optional[str] = None, lic_or_app: Optional[str] = None,
                expire_year: Optional[str] = None, limit: int = 200) -> list[dict]:
        col = {"zip": "prem_zip", "city": "prem_city", "county": "prem_county",
               "district": "district"}.get(kind)
        if not col:
            raise ValueError("kind must be zip|city|county|district")
        q = f"WHERE {col} ILIKE ?"
        args: list = [f"{area}%"]
        q, args = self._filters(q, args, status=status, license_type=license_type,
                                lic_or_app=lic_or_app, expire_year=expire_year)
        q += " ORDER BY primary_name LIMIT ?"
        args.append(limit)
        rows = self.con.execute(f"SELECT * FROM licenses {q}", args).fetchall()
        cols = [d[0] for d in self.con.description]
        return [dict(zip(cols, r)) for r in rows]

    def by_address(self, fragment: str, match_mail: bool = False, status: Optional[str] = None,
                   license_type: Optional[str] = None, lic_or_app: Optional[str] = None,
                   expire_year: Optional[str] = None, limit: int = 200) -> list[dict]:
        """Find every license/application whose PREMISES address matches a fragment
        (street, street + number, city, or zip). With match_mail=True, also matches
        the mailing address — useful for entity-level identification."""
        like = f"%{fragment.strip().upper()}%"
        cols = ["prem_addr1", "prem_addr2", "prem_city", "prem_zip", "prem_state"]
        if match_mail:
            cols += ["mail_addr1", "mail_addr2", "mail_city", "mail_zip", "mail_state"]
        q = "WHERE " + " OR ".join(f"{c} ILIKE ?" for c in cols)
        args: list = [like] * len(cols)
        q, args = self._filters(q, args, status=status, license_type=license_type,
                                lic_or_app=lic_or_app, expire_year=expire_year)
        q += " ORDER BY prem_addr1, primary_name LIMIT ?"
        args.append(limit)
        rows = self.con.execute(f"SELECT * FROM licenses {q}", args).fetchall()
        cols = [d[0] for d in self.con.description]
        return [dict(zip(cols, r)) for r in rows]

    def overdue(self, min_days_past: int = 0, area: Optional[str] = None, kind: str = "zip",
                limit: int = 200) -> list[dict]:
        """Records still marked ACTIVE whose expiration date is in the past —
        renewal-failure / auto-revocation candidates."""
        cutoff = (date.today() - timedelta(days=min_days_past)).isoformat()
        q = ("WHERE status = 'ACTIVE' AND expire_date IS NOT NULL AND trim(expire_date) != '' "
             "AND strptime(trim(expire_date), '%d-%b-%Y')::DATE < CAST(? AS DATE)")
        args: list = [cutoff]
        if area:
            col = {"zip": "prem_zip", "city": "prem_city", "county": "prem_county",
                   "district": "district"}[kind]
            q += f" AND {col} ILIKE ?"
            args.append(f"{area}%")
        q += " ORDER BY expire_date LIMIT ?"
        args.append(limit)
        rows = self.con.execute(f"SELECT * FROM licenses {q}", args).fetchall()
        cols = [d[0] for d in self.con.description]
        return [dict(zip(cols, r)) for r in rows]

    def pending(self, area: Optional[str] = None, kind: str = "zip", limit: int = 200) -> list[dict]:
        return self.by_area(area, kind, status="PEND", limit=limit) if area else \
            self.search("", status="PEND", limit=limit)

    def expiring(self, days: int = 90, area: Optional[str] = None, kind: str = "zip",
                 limit: int = 200) -> list[dict]:
        today = date.today().isoformat()
        horizon = (date.today() + timedelta(days=days)).isoformat()
        # Expire dates are DD-MON-YYYY (e.g. 31-DEC-2026); normalize with strptime.
        q = ("WHERE status = 'ACTIVE' AND expire_date IS NOT NULL AND trim(expire_date) != '' "
             "AND strptime(trim(expire_date), '%d-%b-%Y')::DATE >= CAST(? AS DATE) "
             "AND strptime(trim(expire_date), '%d-%b-%Y')::DATE <= CAST(? AS DATE)")
        args: list = [today, horizon]
        if area:
            col = {"zip": "prem_zip", "city": "prem_city", "county": "prem_county"}[kind]
            q += f" AND {col} ILIKE ?"
            args.append(f"{area}%")
        q += " ORDER BY expire_date LIMIT ?"
        args.append(limit)
        rows = self.con.execute(f"SELECT * FROM licenses {q}", args).fetchall()
        cols = [d[0] for d in self.con.description]
        return [dict(zip(cols, r)) for r in rows]

    def stats(self) -> dict:
        by_status = dict(self.con.execute(
            "SELECT status, count(*) FROM licenses GROUP BY status ORDER BY 2 DESC"
        ).fetchall())
        by_type = dict(self.con.execute(
            "SELECT license_type, count(*) FROM licenses GROUP BY license_type ORDER BY 2 DESC"
        ).fetchall())
        apps = self.con.execute("SELECT count(*) FROM licenses WHERE lic_or_app = 'APP'").fetchone()[0]
        return {
            "total_records": self.snapshot_info().row_count,
            "active_licenses": by_status.get("ACTIVE", 0),
            "pending_applications": by_status.get("PEND", 0),
            "surrendered": by_status.get("SUREND", 0),
            "by_status": by_status,
            "by_license_type": by_type,
            "applications_vs_licenses": {"APP": apps, "LIC": self.snapshot_info().row_count - apps},
        }

    def districts(self) -> list[dict]:
        rows = self.con.execute(
            "SELECT district, count(*) n, count(DISTINCT prem_county) counties "
            "FROM licenses GROUP BY district ORDER BY n DESC"
        ).fetchall()
        return [dict(zip(("district", "records", "counties"), r)) for r in rows]


def ensure_data(force: bool = False, quiet: bool = True) -> ABCClient:
    """One-call bootstrap: download if stale, load into DuckDB, return client."""
    csv_path = fetch_daily_export(force=force, quiet=quiet)
    con = load_to_duckdb(csv_path, force=force)
    return ABCClient(con)
