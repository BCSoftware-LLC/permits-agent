"""ABC license type → application requirements mapping (document assembly layer).

Every requirement in this module is traced to an official abc.ca.gov page
(URLs in SOURCES, captured 2026-08-19). Form numbers are verified against the
local forms index (abcgov/forms.py); a form the official pages require but the
local index does not contain is flagged ``in_index: False`` at resolve time so
callers never invent a PDF URL (AGENTS.md: never fabricate data).

Mappings:
- 'new'      — original retail license application (the chargeable product)
- 'transfer' — person-to-person transfer of an existing retail license
- 'renewal'  — annual renewal (fee payment; ABC publishes no application form)

The official "New License Application" page organizes required forms by
ownership structure (sole owner / partnership / corporation / LLC / limited
partnership / trust), not by license type code. The retail form set is the
same for every retail type; entity-specific additions (ABC-140, ABC-243,
ABC-256, ABC-256-LLC) are therefore carried in ``conditional_forms`` with a
note, and per-type differences (quotas/priority lottery, permit nature, etc.)
are carried in ``notes``.
"""

from __future__ import annotations

from . import forms as forms_mod

ACTIONS = ("new", "transfer", "renewal")

# Official abc.ca.gov sources (captured 2026-08-19).
SOURCES = {
    "new_license_application": "https://www.abc.ca.gov/licensing/license-forms/new-license-application/",
    "person_to_person_transfer": "https://www.abc.ca.gov/licensing/transfer-or-change-a-license/person-to-person-transfer/",
    "license_application_requirements": "https://www.abc.ca.gov/licensing/apply-for-a-new-license/license-application-requirements/",
    "license_fees": "https://www.abc.ca.gov/licensing/license-fees/",
    "license_types": "https://www.abc.ca.gov/licensing/license-types/",
    "caterers_permit": "https://www.abc.ca.gov/licensing/license-forms/caterers-permit/",
    "event_authorization": "https://www.abc.ca.gov/licensing/license-forms/event-authorization/",
    "online_renewal_portal": "https://www.abc.ca.gov/abc-launches-new-online-licensing-services-portal/",
}

# --------------------------------------------------------------------------
# Shared form sets — from the "Retail License" column of the official pages.
# --------------------------------------------------------------------------

# New application, retail (S1: new-license-application page).
_NEW_FORMS = [
    "ABC-208-A",  # Individual Personal Affidavit
    "ABC-208-B",  # Individual Financial Affidavit
    "ABC-211-SIG",  # Application Signature Sheet ("Sign On")
    "ABC-217",  # Application Questionnaire
    "ABC-247",  # Statement re: Residences (Rule 61.4)
    "ABC-251",  # Statement re: Consideration Points
    "ABC-253",  # Supplemental Diagram
    "ABC-255",  # Zoning Affidavit
    "ABC-257",  # Licensed Premises Diagram
    "ABC-258",  # Planned Operations Retail
]

_NEW_CONDITIONAL_FORMS = [
    "ABC-140",  # Tied-House Certification — corps, limited partnerships, LLCs (NEW Mar-26; NOT in local index)
    "ABC-243",  # Corporate Questionnaire — corporations
    "ABC-256",  # Limited Partnership Questionnaire
    "ABC-256-LLC",  # Limited Liability Company Questionnaire
]

_NEW_DOCUMENTS = [
    "State-issued identification, driver's license or passport for person(s) appearing at ABC to sign the application.",
    "Verification (proof) of the source of funds — e.g. bank statements, savings passbooks, loan papers, real estate papers, financial statement, gift/loan letters.",
    "Copy of Conditional Use Permit, or receipt showing the CUP application has been submitted (city/county planning department).",
]

_NEW_NOTES = [
    "The main application, Form ABC-211, is referenced by the official page but is NOT in the local forms index "
    "(ABC-211-SIG is its signature page for applicants who cannot appear in person). Obtain Form ABC-211 at the "
    "ABC district office or via the online license application.",
    "Entity-specific forms also required depending on ownership structure: ABC-140 (corporations/LPs/LLCs; NEW on "
    "abc.ca.gov Mar-26 and NOT in the local forms index), ABC-243 (corporations), ABC-256 (limited partnerships), "
    "ABC-256-LLC (limited liability companies).",
    "Applications are filed in person at the local ABC district office. BPC §23985 requires a 30-day posting period "
    "for the application notice; BPC §23987 requires ABC to mail a copy of each application to certain local officials.",
    "All alcohol servers and managers must be RBS certified (Responsible Beverage Service Training Program).",
]

# Transfer (person-to-person), retail (S2: person-to-person-transfer page).
_TRANSFER_FORMS = [
    "ABC-208-A",  # Individual Personal Affidavit
    "ABC-208-B",  # Individual Financial Affidavit
    "ABC-211-SIG",  # Application Signature Sheet ("Sign On")
    "ABC-217",  # Application Questionnaire
    "ABC-227",  # Notice of Intended Transfer (BPC §§24073/24074) — record + certify
    "ABC-253",  # Supplemental Diagram
    "ABC-257",  # Licensed Premises Diagram
    "ABC-258",  # Planned Operations Retail
    "ABC-211-A",  # License Transfer Request ("Sign-Off")
]

_TRANSFER_CONDITIONAL_FORMS = [
    "ABC-140",  # Tied-House Certification — corps, limited partnerships, LLCs (NOT in local index)
    "ABC-243",  # Corporate Questionnaire — corporations
    "ABC-256",  # Limited Partnership Questionnaire
    "ABC-256-LLC",  # Limited Liability Company Questionnaire
    "ABC-282",  # Declaration Re Temporary Permit — only if a temporary retail permit is requested
]

_TRANSFER_DOCUMENTS = [
    "State-issued identification, driver's license or passport for person(s) appearing at ABC to sign the application.",
    "Verification (proof) of the source of funds — e.g. bank statements, savings passbooks, loan papers, real estate papers, financial statement, gift/loan letters.",
]

_TRANSFER_NOTES = [
    "Notice of Intended Transfer: Form ABC-227 (BPC §§24073/24074) must be recorded with the County Recorder and "
    "certified, then presented at filing (ABC-227-A is the §24071.1/24071.2 variant). Obtain the blank form from ABC "
    "or the escrow company.",
    "Escrow: if there is any purchase price or consideration, the full amount must be placed in escrow before filing "
    "(BPC §§24071.1, 24073, 24074, 24074.1, 24074.3, 24075); the applicant must furnish the §24074.3 statement under "
    "penalty of perjury within 30 days of application.",
    "Form ABC-282 (Temporary Permit) only if a temporary retail permit is requested — temporary permit fees apply.",
    "Entity-specific forms also required depending on ownership structure: ABC-140 (corporations/LPs/LLCs; NOT in the "
    "local forms index), ABC-243 (corporations), ABC-256 (limited partnerships), ABC-256-LLC (limited liability companies).",
]

# Renewal — ABC publishes no renewal application form; renewal is an annual fee
# payment (S3 "License Renewal Period" + S4 Annual Fees Overview).
_RENEWAL_FORMS: list[str] = []

_RENEWAL_DOCUMENTS = [
    "Renewal notice from ABC (mailed prior to the license expiration date, showing the annual fee, deadline and payment methods).",
    "Annual fee payment — online via ABC's Licensing Online Services portal (or by mail).",
    "Penalty fee if full payment is not received/postmarked by the payment deadline.",
]

_RENEWAL_NOTES = [
    "All licenses renew on a 12-month basis; renewals must be paid on or before the last day of the month printed on "
    "the license. No license will be transferred until the renewal fee is paid.",
    "No renewal application form exists in the local forms index — renewal is an annual fee payment, not a form filing.",
]

# --------------------------------------------------------------------------
# Per-type notes (S5 license-types page + S4 license-fees page).
# --------------------------------------------------------------------------

_PRIORITY_NOTE = (
    "'General' retail license types are quota-limited per county; new applications are filed only once per year "
    "through ABC's public 'Priority' lottery (form ABC-521, Priority License Application — NOT in the local forms "
    "index; see https://www.abc.ca.gov/licensing/priority-registration-drawings/)."
)

_PER_TYPE_NOTES = {
    "20": [],  # Off-Sale Beer & Wine
    "21": [_PRIORITY_NOTE],
    "40": [],  # On-Sale Beer
    "41": [],  # On-Sale Beer & Wine - Eating Place
    "42": [],  # On-Sale Beer & Wine - Public Premises
    "47": [_PRIORITY_NOTE],
    "48": [_PRIORITY_NOTE],
    "58": [
        "Type 58 is a Caterer's Permit attached to an on-sale license, not a standalone license. Eligible holders: "
        "on-sale beer & wine (41, 42), on-sale general (47, 48, 57), club (50, 51, 52), on-sale general wine/food/art "
        "museum (78), general caterer's (83), designated special use (71, 72, 73, 87, 88, 99). Seasonal licensees and "
        "boat/vessel licensees are not eligible.",
        "Each catered event requires a separate Catering Authorization (Form ABC-218); a Supplemental Diagram "
        "(ABC-253) may be required for the event location, and events may need property owner / law enforcement approval.",
    ],
    "61": [],  # On-Sale Beer - Public Premises
    "77": [
        "Type 77 is an Event Permit attached to an on-sale license, not a standalone license. Eligible holders: on-sale "
        "beer & wine (41, 42), on-sale general (47, 48, 49, 57, 90), on-sale general wine/food/art museum (78), "
        "designated special use (88, 99).",
        "Each event requires a separate Event Authorization (Form ABC-215); authorizations are limited to no more than "
        "four event days per calendar year.",
    ],
    "86": [
        "The Instructional Tasting License can only be held in conjunction with a qualified off-sale license; no "
        "dedicated application form was found in the official forms index — apply at the ABC district office and "
        "verify requirements with ABC.",
    ],
    "87": [
        "Type 87 (Special On-Sale General, specified census tracts in San Francisco) is limited in number and may only "
        "be issued to premises meeting BPC §23826.13; the licensee must operate and maintain the premises as a bona "
        "fide eating place.",
    ],
}

# --------------------------------------------------------------------------
# The mapping itself: (license_type, action) -> entry.
# --------------------------------------------------------------------------


def _entry(license_type: str, action: str, type_name: str, forms: list[str],
           documents: list[str], notes: list[str], source: str,
           conditional_forms: list[str] | None = None) -> dict:
    return {
        "license_type": license_type,
        "type_name": type_name,
        "action": action,
        "forms": list(forms),
        "conditional_forms": list(conditional_forms or []),
        "documents": list(documents),
        "notes": list(notes),
        "source": source,
    }


def _build() -> dict[tuple[str, str], dict]:
    """Build REQUIRED_FORMS: {(type_code, action): entry} for every mapped combo."""
    result: dict[tuple[str, str], dict] = {}

    retail_new = [  # standard retail license types with a 'new' application
        ("20", "Off-Sale Beer & Wine"),
        ("21", "Off-Sale General"),
        ("40", "On-Sale Beer"),
        ("41", "On-Sale Beer & Wine - Eating Place"),
        ("42", "On-Sale Beer & Wine - Public Premises"),
        ("47", "On-Sale General - Eating Place"),
        ("48", "On-Sale General - Public Premises"),
        ("61", "On-Sale Beer - Public Premises"),
        ("87", "Special On-Sale General License for Specified Census Tracts in the City/County of San Francisco"),
    ]
    for code, name in retail_new:
        result[(code, "new")] = _entry(
            code, "new", name, _NEW_FORMS, _NEW_DOCUMENTS,
            list(_NEW_NOTES) + list(_PER_TYPE_NOTES[code]),
            SOURCES["new_license_application"], conditional_forms=_NEW_CONDITIONAL_FORMS,
        )
        result[(code, "transfer")] = _entry(
            code, "transfer", name, _TRANSFER_FORMS, _TRANSFER_DOCUMENTS,
            list(_TRANSFER_NOTES) + list(_PER_TYPE_NOTES[code]),
            SOURCES["person_to_person_transfer"], conditional_forms=_TRANSFER_CONDITIONAL_FORMS,
        )
        result[(code, "renewal")] = _entry(
            code, "renewal", name, _RENEWAL_FORMS, _RENEWAL_DOCUMENTS,
            list(_RENEWAL_NOTES) + list(_PER_TYPE_NOTES[code]),
            SOURCES["license_fees"],
        )

    # Type 58 — Caterer's Permit (S6): apply with ABC-239; per-event ABC-218.
    result[("58", "new")] = _entry(
        "58", "new", "Caterer's Permit", ["ABC-239"],
        [],
        list(_PER_TYPE_NOTES["58"]) + [
            "Apply for a Type 58 caterer's permit by submitting a completed Additional License/Permit Application "
            "(Form ABC-239) to the nearest ABC office. The permit carries its own annual fee.",
        ],
        SOURCES["caterers_permit"],
    )
    result[("58", "renewal")] = _entry(
        "58", "renewal", "Caterer's Permit", _RENEWAL_FORMS, _RENEWAL_DOCUMENTS,
        list(_RENEWAL_NOTES) + list(_PER_TYPE_NOTES["58"]) + [
            "The caterer's permit carries its own annual fee, paid with the license renewal cycle.",
        ],
        SOURCES["caterers_permit"],
    )
    # Type 58 'transfer' intentionally unmapped: the permit is attached to an on-sale license,
    # not transferred on its own (no official transfer page for it) — see lookup() messaging.

    # Type 77 — Event Permit (S7): apply with ABC-239; per-event ABC-215.
    result[("77", "new")] = _entry(
        "77", "new", "Event Permit", ["ABC-239"],
        [],
        list(_PER_TYPE_NOTES["77"]) + [
            "Apply for a Type 77 event permit by submitting a completed Additional License/Permit Application "
            "(Form ABC-239) to the nearest ABC office. The permit carries its own annual fee.",
        ],
        SOURCES["event_authorization"],
    )
    result[("77", "renewal")] = _entry(
        "77", "renewal", "Event Permit", _RENEWAL_FORMS, _RENEWAL_DOCUMENTS,
        list(_RENEWAL_NOTES) + list(_PER_TYPE_NOTES["77"]) + [
            "The event permit carries its own annual fee, paid with the license renewal cycle.",
        ],
        SOURCES["event_authorization"],
    )
    # Type 77 'transfer' intentionally unmapped (permit attached to an on-sale license).

    # Type 86 — Instructional Tasting License (S5): held with a qualified off-sale license.
    result[("86", "new")] = _entry(
        "86", "new", "Instructional Tasting License", [],
        [],
        list(_PER_TYPE_NOTES["86"]),
        SOURCES["license_types"],
    )
    result[("86", "renewal")] = _entry(
        "86", "renewal", "Instructional Tasting License", _RENEWAL_FORMS, _RENEWAL_DOCUMENTS,
        list(_RENEWAL_NOTES) + list(_PER_TYPE_NOTES["86"]),
        SOURCES["license_fees"],
    )
    # Type 86 'transfer' intentionally unmapped (held in conjunction with an off-sale license).

    return result


REQUIRED_FORMS: dict[tuple[str, str], dict] = _build()


def _norm_code(code: str | int) -> str:
    s = str(code).strip()
    if s.isdigit():
        return str(int(s))  # drop leading zeros / whitespace
    return s


def lookup(license_type: str | int, action: str = "new") -> dict | None:
    """Return the requirements entry for (license_type, action), or None if unmapped."""
    code = _norm_code(license_type)
    action = (action or "new").strip().lower()
    if action not in ACTIONS:
        raise ValueError(f"action must be one of {ACTIONS}, got {action!r}")
    return REQUIRED_FORMS.get((code, action))


def resolve(entry: dict, forms_index: list[dict] | None = None) -> dict:
    """Resolve each form number in `entry` against the forms index.

    Returns a copy of the entry with:
      - 'forms' and 'conditional_forms' replaced by lists of
        {number, title, url, in_index} — url is only present when the form
        exists in the local index (never invented).
      - 'type_name' refreshed from the license types reference when available.
    """
    index = forms_index if forms_index is not None else forms_mod.fetch_forms()
    by_number = {f["number"].strip().lower(): f for f in index}
    out = dict(entry)

    def _resolve(nums: list[str]) -> list[dict]:
        resolved = []
        for num in nums:
            f = by_number.get(num.strip().lower())
            if f:
                resolved.append({"number": f["number"], "title": f["title"], "url": f["url"], "in_index": True})
            else:
                resolved.append({"number": num, "title": None, "url": None, "in_index": False})
        return resolved

    out["forms"] = _resolve(entry["forms"])
    out["conditional_forms"] = _resolve(entry["conditional_forms"])

    # Refresh the display name from the live type reference (fall back to static).
    try:
        from . import license_types as lt
        name = lt.short(entry["license_type"])
        if name and "not in reference" not in name:
            out["type_name"] = name
    except Exception:
        pass
    return out


def mapped_actions(license_type: str | int) -> list[str]:
    """Actions ('new'/'transfer'/'renewal') currently mapped for a license type."""
    code = _norm_code(license_type)
    return [a for (c, a) in REQUIRED_FORMS if c == code]
