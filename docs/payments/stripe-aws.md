# Stripe Payment Rail on AWS — permits-agent / ComplianceOS

Status: architecture draft · 2026-08-19 · Owner: BC Software LLC (internal)
Scope: human-customer payments (cards) via Stripe. Agent/x402 payments are a
separate workstream and out of scope here.
Related docs: `../product-brief.md` (revenue model), `../architecture.md`
(platform layers), `../data-sources.md` (mirror), `../AGENTS.md` (guardrails).

---

## 1. Goals & scope

ComplianceOS monetizes three human-payable lines (product brief §Revenue):

| Line | Model | Stripe vehicle | Pricing hint |
|---|---|---|---|
| License monitoring | Recurring subscription, per license/month | Checkout (mode=`subscription`) + Billing | $5–15/license/mo |
| Document assembly / filing | One-time fee per packet | Checkout (mode=`payment`) / Payment Links | $150–500 per filing |
| Verification API | Metered, per call | Billing Meters (usage-based price) | $0.10–1 per verification |

Non-goals: x402 agent payments (separate workstream), white-label platform
fees (future), card storage (never store PANs — Stripe owns PCI).

Principles:

- **Stripe-hosted UI first.** Checkout / Payment Links / Customer Portal keep
  PCI scope out of our Lambdas and off our surface. We never touch card data.
- **AWS-native, serverless, minimal.** API Gateway + Lambda + DynamoDB + SQS +
  EventBridge + Secrets Manager. No servers, no VPC required for v1.
- **Stripe is the system of record for money; our ledger is the system of
  record for entitlements and internal orders.** Both must reconcile.
- **Fail closed on entitlements.** If payment state is unknown, deny the tool.
- **Follow repo guardrails**: no credentials in the repo (Secrets Manager
  only), human approval gates before filing, legal review before charging for
  filing services.

---

## 2. AWS topology

```
                        ┌────────────────────────────────────────────────┐
                        │              STRIPE (test/live)                │
                        │  Products/Prices · Customers · Subs · Meters   │
                        └───────┬───────────────┬───────────────┬────────┘
                                │               │               │
                Checkout /      │  webhooks      │  Events       │  usage
                Payment Links   │  (HTTPS)       │  (EventBridge │  (meter
                (hosted UI)     │                │   destination)│   events API)
                                ▼               ▼               ▼
   User / CLI ──▶ API Gateway ──┴──► webhook    ┌──────────┐  ┌──────────┐
   (browser /   (HTTP API,      Lambda          │EventBridge│  │ verifier │
   dashboard)   public)         (verify sig)    │  bus      │  │ Lambda   │
                                │               └────┬─────┘  └────┬─────┘
                                ▼                    │             │
                            SQS queue  ◄─────────────┘             │
                          (events, DLQ)                            │
                                │                                  │
                                ▼                                  ▼
                    ┌────────────────────────┐         ┌─────────────────────┐
                    │   Processor Lambdas    │         │  meter_events batch  │
                    │  (entitlements, order, │◄────────┤  (async, idempotent) │
                    │   ledger, notify)      │         └─────────────────────┘
                    └───────────┬────────────┘
                                │
                    ┌───────────▼───────────────────────────────────────┐
                    │          DynamoDB (single table-ish design)       │
                    │ customers · subscriptions · entitlements · orders │
                    │ events (idempotency+ledger) · meter buffer        │
                    └───────────┬───────────────────────────────────────┘
                                │
                    ┌───────────▼─────────────┐
                    │ Entitlements service    │──► MCP server (abcgov/server.py)
                    │ (fail-closed check)     │──► CLI (abcgov/cli.py)
                    └─────────────────────────┘──► daily digest cron
```

### Components

| Component | Service | Notes |
|---|---|---|
| Public webhook endpoint | API Gateway HTTP API → Lambda | POST `/stripe/webhook`. No auth header needed — **signature verification in Lambda** is the auth. Request timeout: Stripe waits ~seconds for 2xx; our handler must ack fast (see §5). |
| Webhook verifier | Lambda (`stripe-python` `Webhook.construct_event`) | Rejects bad signatures with 400; validates timestamp window (replay protection). |
| Event intake | SQS standard queue + DLQ | Verifier pushes the verified event to SQS and returns 200 immediately. Processor drains SQS. Idempotency happens at the processor (DynamoDB `events` PK). |
| Alternative delivery | **Stripe EventBridge destination** (native, no public endpoint) | Stripe can deliver events straight to an EventBridge bus in our account. Viable later once the bus pattern is proven; v1 keeps the classic endpoint so `stripe listen` local dev works identically. |
| Entitlement store | DynamoDB `entitlements` | Single row per account: `active`, `license_quantity`, `features[]`, `updated_at`. Written by subscription processors; read by every tool gate. |
| Ledger / idempotency | DynamoDB `events` | Append-only: `event_id` PK, `type`, `object_id`, `amount_cents`, `status`, `received_at`. Same table dedupes webhooks **and** powers reconciliation. |
| Orders (filing) | DynamoDB `orders` | `filing_id`, `checkout_session_id`, `customer_id`, `amount_cents`, `status` (`pending_payment → paid → refunded`), payload ref. |
| Meter buffer | DynamoDB `meter_events` | Outbox pattern: API records usage locally first, then a batcher flushes to Stripe (`status: pending/sent/failed`). Prevents losing usage if Stripe is briefly unavailable. |
| Secrets | Secrets Manager | `stripe/live/secret_key`, `stripe/live/webhook_secret`, `stripe/test/…` — see §6. |
| Non-secrets | SSM Parameter Store | `stripe/live/price_monitoring_per_license`, `price_filing_211`, `meter_id_verifications`, etc. |
| Domain events | EventBridge (internal bus) | `entitlement.granted/revoked`, `order.paid`, `meter.error` → fans out to notifiers (SNS → email/Slack), digest cron, and the MCP gate's cache invalidation. |
| Reconciliation | EventBridge Schedule → Lambda | Daily: pull Stripe Balance transactions + invoices, compare to ledger, alert on mismatch (§5). |

### Why DynamoDB (not Postgres) for v1

- The billing state is a handful of small, point-lookup access patterns
  (`get entitlement by account_id`, `put event by event_id`), not joins.
- Serverless fit: no VPC, no RDS cost floor; pairs with Lambda + SQS.
- Escape hatch: if reporting/analytics outgrows it (e.g. revenue by agency),
  export to the existing DuckDB mirror for analysis — the mirror stays
  analytics-only, never transactional.
- If the team later standardizes on Postgres for the product DB, the
  entitlement/ledger tables are a mechanical migration (columns are simple).
  Decision: **DynamoDB now, revisit when a product DB exists.**

---

## 3. Subscriptions — per-license monitoring

### Product/price model

- **Product** `monitoring` (one per agency adapter, e.g. `monitoring_abc`).
- **Price** `monitoring_abc_license`: recurring `month`, `unit_amount` =
  per-license price (e.g. $9.00), `billing_scheme = per_unit`, quantity =
  number of watched licenses. **Quantity-based, not one-subscription-per-
  license** — one subscription with `quantity = N` licenses is dramatically
  simpler to manage and to prorate than N subscriptions.
- Optional tiering later: `tiered` pricing (e.g. first 10 licenses $12, next
  90 $8) via a tiered price with the same meter/quantity semantics.

### Checkout flow (self-serve)

1. Backend `POST /checkout` (internal API) builds a Checkout Session:
   - `mode = "subscription"`, line item `{price: monitoring_abc_license,
     quantity: <N>}`.
   - `success_url` / `cancel_url`.
   - `client_reference_id = account_id` (our join key back to Stripe).
   - `metadata = {account_id, agency: "abc"}`.
   - `automatic_tax[enabled] = true` (§7) once tax is decided.
   - `subscription_data[metadata]` carries `account_id`.
2. Customer pays on Stripe-hosted Checkout. We never see the card.
3. Webhook `checkout.session.completed` (with `mode=subscription`) →
   processor writes `customers` + `subscriptions` rows, grants
   `entitlements[account_id] = {active: true, license_quantity: N}`, emits
   `entitlement.granted`.

### Quantity changes (add/remove licenses) & proration

- Self-serve: **Customer Portal** (`billing_portal.configurations`, enable
  "subscriptions / update payment method / cancel"). Quantity editing in
  Portal only appears if the Subscription has `adjustable_quantity` enabled —
  set `adjustable_quantity.enabled = true` on the Checkout line item.
- Programmatic: `POST /subscriptions/{id}` with
  `items[{id: sub_item_id, quantity: M}]` (or `subscription_item.updated`).
  Stripe prorates automatically; keep default
  `proration_behavior = create_prorations` so mid-cycle adds/removes land as
  proration line items on the next invoice. Match behavior in both directions
  (adds are owed, removes are credited).
- **Our rule**: entitlement changes are applied from webhooks only
  (`customer.subscription.updated`, `subscription_item.updated`), never from
  our own API call response — webhooks are the single ordering channel.
- Concurrency: if two quantity updates race, the ledger's per-subscription
  `updated` ordering is by Stripe event `created` timestamp; reprocess the
  stale one if needed (rare; acceptable for v1).

### Entitlements gating the MCP/CLI tools

Gate lives in a tiny `entitlements` client used by `abcgov/server.py` (MCP
tools) and `abcgov/cli.py`:

- Check: `entitlements.get(account_id)` → `active == true` AND requested
  feature ∈ `features`. If missing/stale → **fail closed**: tool returns
  `ERR_ENTITLEMENT_REQUIRED` and points the user to Checkout.
- All reads are **free** (monitoring value = watch, not per-query); the
  subscription gates *access to the tool set* (search/monitor/pending/
  expiring), not individual queries. Usage metering is only for the
  verification API (§4).
- License quantity gates **portfolio size**, not queries: the CLI/MCP
  surfaces a hard cap of `license_quantity` watched licenses (enforced when
  persisting a watchlist; a warning when exceeded, write rejected).
- The daily digest cron checks entitlements before sending; revoked accounts
  get a "resume monitoring" email, not silence.
- Cache entitlement lookups for ≤ 60s (DynamoDB TTL-friendly); a webhook
  invalidates via EventBridge → `entitlement.changed` → cache refresh. Fail
  closed on cache miss + DB error.

### Lifecycle events

| Event | Action |
|---|---|
| `invoice.paid` | Mark subscription `paid`; extend `current_period_end`; ensure entitlement active. |
| `invoice.payment_failed` | Enter dunning (Stripe default retries). **Grace window**: entitlement stays active for 3 days past `current_period_end`, then revoked via scheduled check. Send recovery email via Customer Portal link. |
| `customer.subscription.updated` | Re-apply quantity/status → entitlement (handles prorated changes, Portal edits, plan swaps). |
| `customer.subscription.deleted` | Revoke entitlement immediately (cancel-at-period-end sets `cancel_at_period_end`; actual deletion revokes). |
| `customer.subscription.paused` | Revoke (or suspend) entitlement for pause duration. |

Downgrade UX: never take a paying customer dark mid-billing-cycle on one
failed card — dunning retries + grace window first.

---

## 4. One-time flows — filing / document-assembly fees

### Checkout (mode=`payment`) — primary

1. Internal API `POST /filing/checkout {filing_id, type}`:
   - Looks up the one-time **Price** per filing type (e.g.
     `filing_211_new_onsale`), builds Checkout Session `mode="payment"`,
     `metadata = {filing_id, type, agency}`,
     `client_reference_id = account_id`.
   - Filing requires an active monitoring subscription? **No** — one-time
     filing is open to non-subscribers (it's the land-and-expand entry
     point). Decide per-product; default open.
2. Webhook `checkout.session.completed` (mode=`payment`) → processor:
   - Validates `amount_total` against expected price for the filing type
     (sanity guard; mismatch → alert, don't fulfill).
   - Creates/updates `orders[filing_id]` → `paid`; captures the
     `payment_intent` id; emits `order.paid`.
3. Fulfillment is **decoupled**: `order.paid` → SQS → document-assembly
   worker (Layer 2) prepares the packet; the human-facing filing gate (Layer
   3, applicant signs; human approval before submit/payment) is unchanged
   and **out of the Stripe rail's scope** — Stripe only collects the fee.
4. `payment_intent.succeeded` is redundant with `checkout.session.completed`
   here; subscribe to `charge.refunded` / `dispute.created` to flip
   `orders.status` → `refunded` and halt fulfillment if not yet shipped.

### Payment Links — non-interactive sales

- Phone/email/paper-concierge sales (Sherrie's flow) get a **Payment Link**
  (Stripe-hosted, no code) bound to the one-time price, or a Checkout
  Session created with `mode="payment"` and the URL handed out manually.
- Attach `metadata` via `payment_link` creation (or `client_reference_id`)
  so `checkout.session.completed` still lands in the same ledger path.
- Not for subscriptions in v1 (Portal + Checkout covers self-serve).

### Refunds & disputes

- Refunds via Stripe Dashboard or `POST /charges/{id}/refunds` from an
  internal admin tool — both emit `charge.refunded` → ledger + order update.
- `dispute.created` → Slack/email alert; respond within Stripe's timeline.
  Filing disputes are rare but **the sworn-statement nature of ABC
  documents means we dispute hard with the signed intake record**.

---

## 5. Metered usage — verification API

### Product/price

- **Product** `verification`; **Price** `verification_per_call` with
  `billing_scheme = per_unit`, `unit_amount` = e.g. $0.25,
  `recurring.usage_type = metered`, `recurring.interval = month`.
- Per docs (2026): metered prices are **meter-backed** — create/attach a
  **Billing Meter** (`/v1/billing/meters`) and bind it to the price via
  `meter_id`. Meter events drive billing; the legacy
  `usage_records` path is deprecated for new builds — don't use it.
- **Metronome (Stripe's usage-based-billing product) vs Billing Meters**:
  Stripe now recommends Metronome for *new* usage-billing integrations
  (tiered/dimensional pricing, credits, enterprise contracts). For us:
  single flat per-call price, one meter, low volume → **Billing Meters is
  correct for v1** (no added product cost/complexity; Metronome is a
  separate premium offering). Revisit at scale or if tiered/credit pricing
  appears. *(Owner decision D1.)*

### Meter event pipeline (outbox pattern)

1. Verification API Lambda answers the call and records
   `meter_events[{identifier, timestamp, value: 1, customer/account ref}]`
   in DynamoDB **first** (append to outbox), then returns the answer.
2. A batcher (EventBridge Schedule every 5 min, or SQS-driven) flushes
   buffered events to Stripe
   (`POST /v1/billing/meter_events`, `event_name = "verification_call"`).
   - **Idempotency**: `identifier` must be globally unique per event
     (e.g. `verify_<call_uuid>`). Stripe dedupes on identifier — retries are
     safe.
   - **Timestamp**: use the moment of the API call (`timestamp`) so billing
     lands in the correct billing period even if the flush is late.
   - Per-meter `value`: integer units (1 call = 1). Volume caps: batch
     ≤ ~1000 events per request; chunk in the batcher.
3. Failure handling: flush failure marks rows `failed` → DLQ + retry with
   backoff. **Billing impact is bounded** (a late meter event lands in the
   current period); usage data is never lost because the outbox is
   durable.
4. Monitoring: `billing.meter.error_reported` webhook → alert;
   CloudWatch alarm on outbox age (`pending` rows older than 15 min).
5. Meter *events* are also stored in the internal ledger (`meter_events`
   table) with counts by account/period — this is what reconciliation
   compares against Stripe's invoice line items.

### Free-tier / pilot handling

- Grant pilot credits via **billing credits** or a zero-amount promo
  (coupon 100% off first N units) rather than code branches in the verifier.
- Do not meter internal calls (self-diagnostics) — tag requests
  `internal: true` and skip the outbox.

---

## 6. Webhooks + internal ledger + reconciliation

### Endpoint & verification

- API Gateway HTTP API route `POST /stripe/webhook` → Lambda (Python,
  `stripe-python`). No API key on the route; the **signature check is the
  auth**:
  ```python
  payload = event["body"]
  sig = event["headers"]["stripe-signature"]
  stripe.Webhook.construct_event(payload, sig, os.environ["STRIPE_WEBHOOK_SECRET"])
  ```
  - Returns 400 on invalid signature/timestamp (tolerance window ± 5 min —
    replay protection), 200 after the event is safely on SQS.
  - **Thin events**: Stripe is migrating webhook payloads to thin events
    (event + object reference, no snapshot). Processors should **fetch the
    object by id from the Stripe API** rather than trusting embedded object
    data — fresh state, immune to stale payloads, works for both fat and
    thin events.
- One endpoint handles all subscribed types (`*` filtering happens in code
  via an allowlist, or subscribe only to the exact types we need — do the
  latter in the Dashboard).

### Event intake & idempotency

- Flow: webhook Lambda → verify → `SQS.send(event_id)` → 200. Processor
  Lambda reads SQS, does `PutItem(events, condition: event_id does not
  exist)` — the conditional write is the idempotency guard. Duplicate
  deliveries (Stripe retries, our own reprocessing) no-op.
- Stripe retries with backoff up to 3 days on non-2xx; SQS gives us an
  additional retry layer + DLQ after N failures. DLQ alarm → page.
- Event ordering: Stripe doesn't guarantee global order; for
  subscription.updated we compare the event's `created` timestamp against
  the row's last-applied timestamp and skip stale events. (Rare in
  practice; cheap insurance.)

### Ledger & reconciliation

- `events` table is the **ledger**: every processed event appends a row
  (event_id, type, object_id, amount_cents, status, received_at). Money
  events (invoice.paid, charge.refunded, dispute.*, checkout.*) also write
  `amount_cents` so the ledger is a true money log.
- **Daily reconciliation** (EventBridge Schedule → Lambda):
  1. Pull Stripe Balance transactions (`/v1/balance_transactions`) and
     invoices (`/v1/invoices?status=paid&created[gte]=yesterday`) for the
     window.
  2. Compare against ledger rows: every Stripe money event has a matching
     ledger row with same object_id + amount; every invoice.paid matches an
     orders/entitlement side-effect.
  3. Mismatches (missing row, amount drift, orphan charge) → CloudWatch
     alarm + Slack/email. Never auto-correct — human (or a follow-up doc)
     fixes drift.
- Because entitlements and orders are *derived from* ledger-confirmed
  events, the ledger is the single source of truth for "did we actually
  get paid" — Stripe API is source of truth for the money itself.

### Event types to subscribe (v1)

`checkout.session.completed`, `checkout.session.async_payment_succeeded`,
`checkout.session.async_payment_failed`, `invoice.paid`,
`invoice.payment_failed`, `customer.subscription.created/updated/deleted/
paused`, `subscription_item.updated`, `payment_intent.succeeded/failed`,
`charge.refunded`, `dispute.created`, `billing.meter.error_reported`.

---

## 7. Environments, secrets, testing, IaC

### Key management (Secrets Manager)

| Secret | Purpose |
|---|---|
| `stripe/test/secret_key` | Test-mode API key (starts `sk_test_`) |
| `stripe/test/webhook_secret` | Test endpoint signing secret (`whsec_`) |
| `stripe/live/secret_key` | Live key (starts `sk_live_`) — **provisioned at go-live, access audited** |
| `stripe/live/webhook_secret` | Live signing secret |

- Lambdas get secrets via `GetSecretValue` with a narrowly scoped IAM role
  (per-env role; dev role can't read live secrets). Rotate live keys on
  schedule; rotate webhook secrets per Stripe's guidance (roll + update
  env + delete old).
- **Never in the repo** (AGENTS.md rule 3): no `.env` with keys committed,
  no keys in Parameter Store (non-secrets only — price IDs, meter IDs,
  product IDs, feature flags).

### Environment topology

- Three AWS environments: `dev` (Stripe test mode), `staging` (test mode,
  full pipeline incl. webhook), `prod` (live mode). Same CDK stack, one
  `STRIPE_ENV` param switching `stripe/test/*` ↔ `stripe/live/*`.
- **Webhook endpoints per env**: `https://<dev-api>/stripe/webhook` with
  its own signing secret registered in the Stripe Dashboard (test vs live
  endpoints are configured per mode automatically).

### Testing

- **Stripe CLI** is the dev loop: `stripe listen --forward-to
  localhost:8000/stripe/webhook` for local, `stripe trigger
  invoice.paid` / `customer.subscription.updated` for canned events.
- Test mode: test cards (`4242 4242 4242 4242` success, `4000 0000 0000
  0002` decline), `stripe test_clocks` to fast-forward subscriptions past
  renewal/failure windows — essential for testing proration, dunning, and
  the grace-period revoke.
- Scripted scenario suite (pytest + `stripe-mock` or CLI): subscribe →
  add license → proration → failed payment → dunning → revoke → re-pay →
  re-grant; one-time checkout → paid → refund → dispute; meter flush →
  invoice contains usage.
- Reconcile test: run the reconciliation lambda against a test ledger and
  confirm it finds injected drift.

### IaC

- **AWS CDK (Python)** — matches the repo's Python stack; one stack
  `PaymentRailStack` (API Gateway, Lambdas, SQS+DLQ, DynamoDB tables,
  EventBridge rules/schedule, Secrets/SSM refs, alarms). Plain
  CloudFormation if the team prefers zero-dep; CDK recommended.
- Stripe-side config (products/prices/meters/webhook endpoints) versioned
  as an idempotent bootstrap script (`scripts/stripe_bootstrap.py`) run via
  Stripe API against test/live — prices and meters get stable IDs stored in
  Parameter Store.

---

## 8. Tax — flag, don't resolve (legal territory)

- **Stripe Tax** can be enabled per Checkout Session /
  Subscription (`automatic_tax[enabled] = true` with `customer` address).
  It calculates + collects US state sales tax (all states, 100+ countries),
  with product tax codes (SaaS code `txcd_10103001` for monitoring/API;
  filing services need a services code — verify).
- Known contours (2026): SaaS is **not taxable in California** (our home and
  primary market) but is taxable in ~25 states (NY 100%, TX 80%, WA treats
  SaaS as tangible software, etc.). Filing/document services can be taxed
  differently from SaaS in the same state — **cannot be assumed**.
- **Owner decisions required (not engineering)**: whether BC Software is
  the merchant of record; which states have nexus (CA HQ + wherever
  customers sit); whether to register/collect or use Stripe Tax's
  calculation + manual remittance; whether *filing-service fees* count as
  taxable services in the states where filings occur (legal review — the
  repo already gates charging for filing services behind legal review).
- Engineering posture: build `automatic_tax` plumbing **now** (it's one
  flag + tax-exempt handling for resellers), decide *collection policy* at
  go-live with legal. Stripe Tax has no monthly fee below ~$100k annual
  volume (then % of volume) — check current pricing at implementation.

---

## 9. Phased plan

| Phase | Scope | Exit criteria |
|---|---|---|
| **0. Foundations** | Stripe account + test mode; CDK scaffold; Secrets/SSM layout; webhook endpoint with signature verification + SQS + ledger table; stripe CLI dev loop | `stripe trigger invoice.paid` round-trips to ledger; alarms wired |
| **1. Monitoring subscriptions** | Products/prices; Checkout (subscription, adjustable qty); Customer Portal; entitlement store; MCP/CLI gate + watchlist cap; digest cron gate; dunning/grace/revoke | Live (test-mode) subscribe → MCP tools gated correctly; quantity change prorates; failed payment revokes after grace |
| **2. One-time filing fees** | Checkout mode=payment per filing type; Payment Links; `orders` table; fulfillment queue to document-assembly; refunds/disputes; dashboard for refunds | Paid filing → order.paid → packet job queued; refund flips order + alerts |
| **3. Metered API** | Billing Meter + meter-backed price; outbox + batcher; ledger counts; `billing.meter.error_reported` alert; usage monitor/alerts | Verification API call → meter event → invoice line item in test mode |
| **4. Production hardening** | Reconciliation daily job + drift alerts; thin-event migration; tax policy (legal sign-off); rate limiting/authz on checkout APIs; x402 handoff doc | Reconciliation clean for 30 days; legal sign-off on tax + filing fees; go-live with real customers (pilot) |

Each phase ships with test-mode end-to-end evidence (the scripted scenario
suite from §7) before live keys are touched.

---

## 10. Top owner decisions (summary)

1. **D1 — Metering: Billing Meters vs Metronome.** Stripe now steers new
   usage-billing integrations to Metronome (premium, richer pricing). For a
   flat per-call verification price at our volume, **Billing Meters
   (meter-backed prices) for v1**, revisit Metronome only if tiered/
   credits/enterprise pricing appears. (Engineering recommends Billing
   Meters.)
2. **D2 — Merchant of record + tax posture.** BC Software vs Permits N More
   as the Stripe merchant; CA is non-taxable for SaaS but filing services
   are a separate question; ~25 states tax SaaS. Needs **legal sign-off
   before charging for filing services** (repo guardrail). Build
   `automatic_tax` plumbing regardless.
3. **D3 — Entitlement enforcement level.** Do we (a) gate tool access
   server-side (recommended — fail closed), and (b) hard-cap watchlists at
   `license_quantity`, or let customers exceed and bill overage? Affects
   Phase 1 scope; recommended: gate + hard cap, no overage in v1.
4. **D4 — Pricing mechanics**: $/license/mo with quantity on one
   subscription + proration (recommended) vs per-license subscriptions;
   plus the monitoring price point itself ($5–15 band) and pilot discounts.
5. **D5 — Self-serve vs assisted sales split**: Customer Portal + Checkout
   for self-serve, Payment Links for Sherrie's concierge sales — confirm
   the concierge flow gets identical ledger/reconciliation treatment
   (it does in this design).

---

## Appendix A — DynamoDB table shapes (v1)

```
customers      PK customer_id   (Stripe customer id)  attrs: account_id, email, name, created
subscriptions  PK sub_id        attrs: customer_id, account_id, plan, quantity,
                                status, current_period_end(ttl), metadata
entitlements   PK account_id    attrs: active, license_quantity, features[SS], updated_at
events         PK event_id      attrs: type, object_id, amount_cents, status,
                                received_at, applied_at      (ledger + idempotency)
orders         PK filing_id     attrs: customer_id, checkout_session_id, price_cents,
                                status, agency, type, created_at
meter_events   PK identifier    attrs: timestamp, value, account_id, status,
                                flushed_at (ttl)             (outbox; billing-side copy is source)
```

## Appendix B — Key risks & mitigations

| Risk | Mitigation |
|---|---|
| Lost webhook (Stripe 3-day retry exhausted) | SQS DLQ + alarm; daily reconciliation catches any gap (money vs ledger) |
| Duplicate webhook processing | Conditional PutItem on `event_id`; event `created` ordering check |
| Meter events lost on Stripe outage | Durable outbox in DynamoDB first; batcher retries; late events bill to current period |
| Entitlement drift (paid but gated) | Entitlement written only from verified events; daily reconcile emits `entitlement.changed` on drift |
| Card data exposure | Never stored/processed by us — Checkout/Portal/Links only (PCI out of scope) |
| Test-mode leakage to prod | Env-separated secrets + roles; `STRIPE_ENV` guard in bootstrap; live key access audited |
| Chargebacks on filing fees | Signed intake record kept per filing; dispute alerting; respond in-window |
