"""ABC status & district reference.

Status meanings are ABC's own LQS glossary (https://www.abc.ca.gov/licensing/license-lookup/glossary/),
captured 2026-08-19. The daily export only ever shows a subset of these at any
moment (currently ACTIVE/PEND/SUREND/REVPEN/SUSPEN/R64B) — licenses move through
the rest (REV, REVP, NREN, WDRL, …) as their lifecycle unfolds.

Note on "auto revocation": ABC's vocabulary for it is REVP — "Revocation
Pending Due To Non-Payment of Recent Renewal" (annual-fee non-payment, B&P
§ 24049 auto-revocation path). We also derive an OVERDUE status: records still
marked ACTIVE whose expiration date has already passed — i.e. renewal-failure /
auto-revocation candidates.
"""

from __future__ import annotations

from datetime import date

STATUS_MEANINGS: dict[str, str] = {
    "ACTIVE": "Valid and exercisable (current license).",
    "PEND": "Pending — application under review; not yet issued.",
    "DENY": "Denied — application rejected.",
    "INACT": "Inactive — license not currently exercised.",
    "ISSUPD": "Indefinite Suspension.",
    "NREN": "Non-Renewal — renewal refused.",
    "R64B": "Issue And Hold — issued but held (not yet in use).",
    "R65": "Surrendered, not in use.",
    "REV": "Revocation — license revoked.",
    "REVP": "Revocation Pending Due To Non-Payment of Recent Renewal (auto-revocation path).",
    "REVPEN": "Revocation Pending — disciplinary revocation action initiated.",
    "RNST": "In the process of being reinstated due to late payment of renewal.",
    "SUSPEN": "Suspended — privileges temporarily withdrawn.",
    "SUSPEND": "Suspended (glossary spelling).",
    "SUREND": "Surrendered — voluntarily given up.",
    "S/REV": "Revoked for SLMS (Social Services hold).",
    "SLMS": "Social Services Hold — noncompliance with child-support/benefits rules.",
    "VOID": "Voided — never took effect / cancelled.",
    "WDRL": "Withdrawn — application withdrawn by applicant.",
}

# Derived statuses computed by this tool (not in ABC's vocabulary).
DERIVED_STATUSES: dict[str, str] = {
    "OVERDUE": "Still ACTIVE in the export but past its expiration date — renewal "
               "failure / auto-revocation candidate (ABC auto-revokes for unpaid "
               "annual fees; glossary term REVP).",
}

DISTRICT_NAMES: dict[str, str] = {
    "01": "Inglewood", "02": "Monrovia", "03": "Lakewood/Long Beach",
    "04": "Los Angeles/Metro", "05": "Van Nuys", "06": "Bakersfield",
    "07": "Riverside", "08": "Rancho Mirage", "09": "San Marcos",
    "10": "San Diego", "11": "Santa Ana", "12": "Ventura",
    "13": "San Luis Obispo", "21": "Fresno", "22": "Oakland",
    "23": "Sacramento", "24": "San Francisco", "25": "San Jose",
    "26": "Salinas", "27": "Santa Rosa", "28": "Eureka", "29": "Stockton",
    "30": "Yuba City", "31": "Redding", "50": "Headquarters",
    "51": "Northern Division", "52": "Southern Division",
    "75": "Headquarters Licensing",
}

ACTION_CODES: dict[str, str] = {
    "DBL": "Double Transfer", "DPR": "Dropping Partner", "DOR": "Duplicate Original",
    "EXC": "Exchange", "FID": "Fiduciary", "ICO": "Intercounty", "ORI": "Original",
    "PER": "Person to Person", "PREM": "Premise to Premise", "STK": "Stock Transfer",
    "EXC/PER": "Person to Person Exchange", "EXC/PERP": "Person to Person, Premise to Premise Exchange",
    "EXC/PREM": "Premise to Premise Exchange",
}


def status_overview(con) -> dict:
    """Observed status distribution from the mirror + full vocabulary context."""
    import duckdb

    rows = con.execute(
        "SELECT status, count(*) n FROM licenses GROUP BY status ORDER BY n DESC"
    ).fetchall()
    observed = {s: n for s, n in rows}

    today = date.today().isoformat()
    overdue = con.execute(
        "SELECT count(*) FROM licenses WHERE status = 'ACTIVE' AND expire_date IS NOT NULL "
        "AND trim(expire_date) != '' AND strptime(trim(expire_date), '%d-%b-%Y')::DATE < CAST(? AS DATE)",
        [today],
    ).fetchone()[0]

    return {
        "observed_in_export": observed,
        "overdue_active_licenses": overdue,
        "vocabulary": {k: v for k, v in STATUS_MEANINGS.items()},
        "derived": DERIVED_STATUSES,
    }
