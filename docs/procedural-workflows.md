# Procedural workflows: deadlines, delivery and human handoffs

Owner requirement, September 20, 2026: a useful permitting product must manage the actual sequence of agency work, including time windows, in-person delivery, the right department, required attendees and proof of completion. This is a domain implementation contract. The current generic checklist and owner-selected calendar export do not implement it.

## The unit of work is a verified procedural activity

A checklist entry such as “File CUP” is insufficient. Each activity must be versioned and applicable to a specific issuer, premises, business activity and transaction. Store the following structured fields, backed by official evidence and reviewed exceptions:

| Area | Information needed |
|---|---|
| Applicability | Jurisdiction/parcel, agency, permit/transaction, conditions, exemptions, rule version and effective dates |
| Action | Concrete task, responsible customer/representative, prerequisite tasks, earliest start and completion criteria |
| Timing | Trigger event and evidence, interval/window, calendar versus business days, applicable holiday calendar, counting convention, timezone, cutoff and permitted extensions |
| Delivery | Online, email, mail, courier, counter, hearing or inspection; allowed alternatives and required route for this transaction |
| Destination | Official department, counter/room, verified address or portal, office hours, appointment requirement and contact channel |
| People | Required attendees, representation/signing authority, assigned case planner if verified, receiving staff versus decision-making authority |
| Documents | Exact form/revision, original versus copy, quantity, dimensions/format, wet/electronic/notarized signature, exhibits and cross-document consistency |
| Fees | Current source, deposit versus final charge, payee/payment channel, approved amount/currency and payment evidence |
| Proof | Timestamped portal acknowledgment, delivery/counter receipt, filed copy, tracking, inspection result or formal decision; what each proves and does not prove |
| Exceptions | Deficiencies, rejected delivery, office closure, unavailable appointment, modified instructions, expired forms, extension procedure and escalation |
| Provenance | Source URL and section/page, retrieval and review dates, source fingerprint, effective version, reviewer, confidence/status and conflicting evidence |

The public source library holds reusable rules. Private cases hold actual triggers, applicant facts, documents, appointments and receipts. Human tasks are first-class activities with owners and dependencies, not footnotes in an AI answer.

## Deadline calculation must be deterministic and explainable

Separate statutory deadlines, agency-set dates, minimum waiting periods, processing estimates, appointment dates and internal targets. Do not turn a published estimate into a due date or an opening-date promise.

An activity deadline requires a verified rule and an evidenced trigger. Missing or conflicting inputs produce `needs_verification`; do not silently assume today's date, calendar days, midnight, or the next business day. Store the applicable calendar/version and timezone. Use local calendar arithmetic for day-based rules rather than adding multiples of 24 hours across daylight-saving changes. A closed office does not by itself establish that a legal deadline extends.

The owner should see the derivation: the source, trigger, counted interval, excluded days where applicable, due date/cutoff, and any unresolved qualification. Distinguish “received by” from “sent/postmarked by.” A mailing reminder must allow delivery time without presenting an estimated postal transit duration as a legal extension.

If a trigger, rule, hearing date or accepted extension changes, version the calculation, identify dependent tasks, invalidate stale reminders and approvals where necessary, and show what moved and why. Rejection, corrections and resubmission do not universally reset a clock; use the rule for that agency and process.

## Research examples that demonstrate the nuance

These are source-backed design/evaluation examples retrieved September 20, 2026, not a complete procedure pack or a determination for an actual applicant.

**ABC notification timing.** ABC's ABC-207 instructions tie the protest period to posting or mailing, whichever is later. They also specify follow-up notice/declaration materials to return to the district office. A single “application date + 30” shortcut loses the relationship between separate events. The product must establish applicability, retain posting/mailing evidence and distinguish the waiting/protest period from applicant tasks. [ABC-207 instructions](https://www.abc.ca.gov/abc-207-instructions/).

**Estimated processing versus required time.** ABC's application requirements page separately discusses a posting period and typical investigation duration. These are different kinds of time information; neither establishes a guaranteed opening date. Published general in-person guidance also needs reconciliation with supported online application paths for the exact transaction. [ABC application requirements](https://www.abc.ca.gov/licensing/apply-for-a-new-license/license-application-requirements/), [ABC online services](https://www.abc.ca.gov/online-services/).

**Irvine optional meeting versus formal submission.** The City's August 6, 2026 article describes a pre-application meeting as optional, before formal application, with preliminary feedback from multiple disciplines. We must not convert it into a mandatory filing prerequisite or record it as permit approval. [Current City article](https://cityofirvine.gov/news-media/news-article/streamlining-development-irvine-value-pre-application-meetings).

**Legacy instructions versus current delivery route.** An accessible Irvine CUP PDF is explicitly revised March 2004 and includes physical-plan and delivery language. A separate official tenant-improvement guide describes Irvine READY submission and in-person assistance appointments. They cover different procedures and vintages; neither justifies declaring all CUP submissions physical or all tasks electronic. Find the current applicable instructions and resolve conflicts before issuing an actionable delivery package. During this research an older `cityofirvine.org` news URL redirected to the new `cityofirvine.gov` homepage, demonstrating that an HTTP-successful redirect does not prove the requested instruction was retrieved. [Legacy CUP sheet](https://legacy.cityofirvine.org/pdfs/cd/42_02.pdf), [tenant-improvement guide](https://legacy.cityofirvine.org/civica/filebank/blobdload.asp?BlobID=35766).

## What an in-person task should produce

Prepare a visit packet: purpose, prerequisites, verified location/department, confirmed appointment or walk-in rules, who must attend, representation requirements, exact document checklist, approved payment arrangement and what receipt to obtain. The interface distinguishes `appointment_requested`, `appointment_confirmed`, `delivered`, `accepted_for_review` and `approved`.

Assign the visit to an authorized person, offer an internal preparation target and reminders, and track acknowledgment. Calendar booking and sending correspondence remain separately authorized external actions. An exported calendar file is not a booked appointment. A user's “I dropped it off” is an assertion until the appropriate receipt is verified. Receiving staff may not be the decision maker; named staff assignments must be dated and rechecked.

If the owner cannot attend or the office rejects a package, create a recovery task, preserve the rejection reason and any running deadline, and route to a qualified human. Do not fabricate a receipt, bypass portal controls, or assume another department accepted the package.

## Build and acceptance sequence

1. Create the issuer/procedure registry with a review queue, source versions, applicability and explicit unsupported paths. An LLM may propose extracted rules; reviewed structured rules control deadlines and execution.
2. Add case activities, evidenced trigger events, dependency graphs, assignments and receipt semantics. Keep applicant assertions distinct from agency-confirmed events.
3. Implement a tested calculation engine with per-rule counting conventions, calendars, cutoffs, stale/conflict handling, amendments and recalculation history. Do not enable an unreviewed rule by default.
4. Connect durable reminder/escalation jobs, approval records and supported scheduling/correspondence adapters. Test revocation, duplicate events and uncertain outcomes before customer use.
5. Validate complete representative journeys with Permits N More, including physical visits, deficiencies, amended plans and a final recorded result. Keep California-wide intake while publishing actual procedure coverage.

Acceptance fixtures must cover: later-of-two triggers; missing trigger; exact-cutoff delivery; weekends and issuer-specific holidays; leap day and DST; paused/extended clocks with evidence; changed hearing dates; office closure without an automatic extension; stale form revisions; appointment unavailable before a deadline; physical originals versus electronic copies; wrong counter; absent signatory; rejected package; corrected receipt; duplicate reminder delivery; and submission receipt versus substantive approval.

A procedure is “supported” only when its source/version, applicability, timing, delivery, documents and completion evidence are reviewed and the workflow is exercised. A reachable website or a checklist generated by the model is not sufficient coverage.
