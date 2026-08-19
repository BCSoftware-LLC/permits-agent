"""Status-change history — the proprietary moat.

A daily snapshot of every record's (file_number, license_type, lic_or_app,
status, expire_date) is appended to a `history` table that SURVIVES mirror
rebuilds (load_to_duckdb only drops the `licenses` table). Over time this
accumulates the one asset public data alone cannot buy: the transition
history (ACTIVE→SUSPEN→REV, approval durations, PEND lifetimes) per license.

    abc-agent history-snapshot        # append today's snapshot (~129k rows)
    abc-agent history-diff            # status/expiry changes since last snapshot

Diff joins consecutive snapshots on (file_number, license_type) and reports:
status transitions, expire-date changes, records that newly appeared, and
records that disappeared.
"""

from __future__ import annotations

from datetime import date
from pathlib import Path
from typing import Optional

import duckdb

from .db import DB_PATH

HISTORY_TABLE = "history"

_SCHEMA = f"""
CREATE TABLE IF NOT EXISTS {HISTORY_TABLE} (
    snapshot_date DATE,
    file_number   VARCHAR,
    license_type  VARCHAR,
    lic_or_app    VARCHAR,
    status        VARCHAR,
    expire_date   VARCHAR
)
"""


def _ensure_history(con: duckdb.DuckDBPyConnection) -> None:
    con.execute(_SCHEMA)
    con.execute(
        f"CREATE INDEX IF NOT EXISTS idx_hist_date ON {HISTORY_TABLE}(snapshot_date)"
    )


def snapshot(con: Optional[duckdb.DuckDBPyConnection] = None,
             db_path: Path = DB_PATH) -> dict:
    """Append today's status snapshot of every record. Returns row counts."""
    con = con or duckdb.connect(str(db_path))
    _ensure_history(con)
    today = date.today().isoformat()
    # Skip duplicate runs on the same day (idempotent per day).
    existing = con.execute(
        f"SELECT COUNT(*) FROM {HISTORY_TABLE} WHERE snapshot_date = ?", [today]
    ).fetchall()[0][0]
    if existing:
        return {"snapshot_date": today, "rows_added": 0,
                "note": "snapshot already exists for today", "rows_in_table": _total(con)}
    con.execute(
        f"""
        INSERT INTO {HISTORY_TABLE} (snapshot_date, file_number, license_type,
                                     lic_or_app, status, expire_date)
        SELECT ?, file_number, license_type, lic_or_app, status, expire_date
        FROM licenses
        """,
        [today],
    )
    return {"snapshot_date": today, "rows_added": _snapshot_rows(con, today),
            "rows_in_table": _total(con)}


def _total(con: duckdb.DuckDBPyConnection) -> int:
    return con.execute(f"SELECT COUNT(*) FROM {HISTORY_TABLE}").fetchall()[0][0]


def _snapshot_rows(con: duckdb.DuckDBPyConnection, day: str) -> int:
    return con.execute(
        f"SELECT COUNT(*) FROM {HISTORY_TABLE} WHERE snapshot_date = ?", [day]
    ).fetchall()[0][0]


def diff(con: Optional[duckdb.DuckDBPyConnection] = None,
         db_path: Path = DB_PATH) -> dict:
    """Diff the two most recent snapshots. Returns transitions, additions,
    removals, and expire-date changes with counts."""
    con = con or duckdb.connect(str(db_path))
    _ensure_history(con)
    days = [r[0] for r in con.execute(
        f"SELECT DISTINCT snapshot_date FROM {HISTORY_TABLE} ORDER BY 1 DESC LIMIT 2"
    ).fetchall()]
    if len(days) < 2:
        return {"status": "need_more_snapshots",
                "snapshots": days,
                "message": "Run history-snapshot daily; at least two snapshots are needed to diff."}
    d1, d2 = days[1], days[0]  # d1 older, d2 newer

    transitions = con.execute(
        f"""
        SELECT a.file_number, a.license_type, a.lic_or_app,
               a.status AS prev_status, b.status AS new_status,
               a.expire_date AS prev_expire, b.expire_date AS new_expire
        FROM {HISTORY_TABLE} a
        JOIN {HISTORY_TABLE} b
          ON a.file_number = b.file_number AND a.license_type = b.license_type
        WHERE a.snapshot_date = ? AND b.snapshot_date = ?
          AND a.status != b.status
        ORDER BY a.file_number
        LIMIT 500
        """,
        [d1, d2],
    ).fetchall()
    trans_cols = ["file_number", "license_type", "lic_or_app", "prev_status",
                  "new_status", "prev_expire", "new_expire"]
    added = con.execute(
        f"""
        SELECT b.file_number, b.license_type, b.status
        FROM {HISTORY_TABLE} b
        LEFT JOIN {HISTORY_TABLE} a
          ON a.file_number = b.file_number AND a.license_type = b.license_type
         AND a.snapshot_date = ?
        WHERE b.snapshot_date = ? AND a.file_number IS NULL
        ORDER BY b.file_number LIMIT 500
        """,
        [d1, d2],
    ).fetchall()
    removed = con.execute(
        f"""
        SELECT a.file_number, a.license_type, a.status
        FROM {HISTORY_TABLE} a
        LEFT JOIN {HISTORY_TABLE} b
          ON a.file_number = b.file_number AND a.license_type = b.license_type
         AND b.snapshot_date = ?
        WHERE a.snapshot_date = ? AND b.file_number IS NULL
        ORDER BY a.file_number LIMIT 500
        """,
        [d2, d1],
    ).fetchall()

    def to_dicts(rows, cols):
        return [dict(zip(cols, r)) for r in rows]

    return {
        "status": "ok",
        "from": d1,
        "to": d2,
        "transition_count": len(transitions),
        "added_count": len(added),
        "removed_count": len(removed),
        "transitions": to_dicts(transitions, trans_cols),
        "added": to_dicts(added, ["file_number", "license_type", "status"]),
        "removed": to_dicts(removed, ["file_number", "license_type", "status"]),
    }
