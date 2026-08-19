# Product Brief — what BC Software creates (working title: **ComplianceOS**)

Status: internal strategy draft · 2026-08-19 · Owner: BC Software LLC

## The thesis

The next platform-scale services company is an **AI-native services firm** in a
vertical that is **Document-Driven · Rules-Bound · Verifiable**. Permits and
licenses are the canonical instance: every permit is a document, every issuance
follows codified rules, and every status is a verifiable public record.
**Permits N More** is the vertical front; **BC Software** builds the engine.

## The split (Sherrie vs BC Software)

| | Permits N More (Sherrie) | BC Software (us) |
|---|---|---|
| Surface | `permitsnmore.com` (.com parked since 2014 → make it the front; .org = brand site) | The product + IP |
| Role | Customer-facing filing concierge for CA ABC (+ future verticals) | Headless compliance engine, adapters, API, data |
| Relationship | First tenant & distribution | Platform owner |
| Monetization | Per-filing fees, retainers | Usage + residual: subscriptions, per-filing, API |

BC Software should **not** build another consumer brand. It creates the thing
every permit business will need: a **headless compliance automation platform**.

## What ComplianceOS is

An agent-native platform that, for any agency (driver = adapter), can:

1. **Ingest** — pull the agency's public data into a local mirror
   (abc-agent is the CA ABC driver: 128k-record daily mirror, done & verified).
2. **Monitor headlessly** — portfolio/territory watch with daily briefs,
   status-change diffs, renewal/expiration/auto-revocation radar, lead radar
   on new filings. *(Daily digest built and running via cron — the product in
   miniature.)*
3. **Assemble documents** — intake → auto-filled application packets from the
   agency's form library + fee schedules (Layer 2; 87 ABC forms already indexed).
4. **File with gates** — guided e-filing under the customer's own account,
   human approval at submit/payment (Layer 3; legal review required).
5. **Verify via API** — "is this business's permit valid right now?" — a
   compliance-verification API (B2B: lenders, insurers, franchisors,
   marketplaces, landlords). **This is the verifiable trait monetized and the
   least obvious revenue line.**

## Revenue model (residual + usage)

| Line | Model | Example |
|---|---|---|
| Monitoring subscriptions | **Residual** — $/license/month, zero marginal cost | $5–15/license/mo for ABC portfolio watch |
| Document assembly / filing | **Usage** — per packet | $150–500 per filed application |
| Verification API | **Usage** — per call | $0.10–1 per verification |
| White-label to other permit firms | **Residual** — platform fee | % of their revenue or seat fee |

The residual engine = monitoring + history + API. One headless run a day
serves every subscriber — that is the margin machine.

## Why the moat is real (and what it is)

The adapter is not the moat (data is public). The moat is:
- **Status-change history** — nobody else accumulates ACTIVE→SUSPEN→REV
  transitions, approval times by district/type, PEND durations. We will, from
  day one of daily mirroring. This is proprietary, sellable, defensible data.
- **Document automation** — 87 forms + fee logic + per-type requirements
  encoded once, reused by every tenant.
- **Verification trust** — an API that answers a compliance question with
  provable freshness beats any scraping service.

## Roadmap (probe → pilot → platform)

1. ✅ CA ABC driver (abc-agent) — verified, 14 MCP tools, CLI.
2. ✅ Headless daily digest (cron) — residual engine demonstrated.
3. ⏳ Status-change diffing — the history moat (next build).
4. ⏳ Document-assembly spike — intake → ABC-211 packet + fee estimate.
5. ⏳ Agency #2 driver — prove platform > adapter (contractors, food, tobacco,
   county business licenses — any document-driven/rules-bound/verifiable agency).
6. ⏳ Verification API prototype + 1 pilot customer (lender/insurer/franchisor).
7. ⏳ Pilot pricing with 2–3 real licensees via Sherrie's network.

## Probing principles

- Public data only; never fabricate; freshness always surfaced.
- Ship the smallest headless loop that generates a real signal, then iterate.
- Each agency adapter must pay for itself with one of the five revenue lines
  before the next adapter is built.
- Sherrie's business validates the vertical; BC Software's product outlives
  any single tenant.
