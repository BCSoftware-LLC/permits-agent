# x402 + Coinbase Payment Rail on AWS — permits-agent / ComplianceOS

**Status:** Architecture proposal (design doc, no code yet)
**Author:** BC Software engineering (Hermes subagent)
**Last verified:** 2026-08-19 — protocol facts pinned to the x402 V2 spec (x402-foundation/x402, `specs/x402-specification-v2.md` + `specs/transports-v2/http.md`, latest commit 2026-08-17), Coinbase CDP x402 docs (docs.cdp.coinbase.com/x402), and the x402 facilitator landscape roundup (Wavect, verified 2026-07-12).
**Scope:** x402 (stablecoin, agent-first) rail on AWS, with Coinbase as the managed facilitator, plus a Stripe fiat rail for humans. Does **not** cover the human-facing checkout UI (Coinbase Commerce) beyond a pointer.

---

## 0. TL;DR

Monetize the permits-agent verification/search API as an x402 "paid resource": every paid HTTP call is negotiated over HTTP 402, paid in USDC on **Base mainnet** (`eip155:8453`) via **Coinbase CDP Facilitator** (managed verify + KYT + settlement + gas), and recorded in a **DynamoDB payment ledger** that doubles as the replay/idempotency guard. Topology: **API Gateway (HTTP API) → Lambda (Python) → DynamoDB**, with **EventBridge → SQS → reconciler Lambda** for async settlement reconciliation, **Secrets Manager + KMS** for the CDP merchant key, and **CloudWatch** for ledger-state alarms. Stripe is added as a second, parallel rail (human buyers / fiat settlement / refunds) — the two rails share the same pricing table and ledger.

Smallest shippable slice (Phase 0): one paid endpoint (`verify-license`) on **Base Sepolia** via CDP testnet facilitator, Python SDK, curl-verifiable 402 → sign → 200 loop. ~1–2 weeks.

---

## 1. How x402 works end-to-end (protocol V2, current)

x402 is an open, HTTP-native payment protocol governed by the **x402 Foundation** (Linux Foundation; formed 2026-04-02, contributed by Coinbase, co-launched with Cloudflare). HTTP 402 itself is still "reserved for future use" in RFC 9110 — x402 supplies the application-layer schemas, headers, and flow on top of it. It is a negotiation protocol, not a processor: a **facilitator** (e.g., Coinbase CDP) does the blockchain work.

Three roles: **Client** (human or agent), **Resource Server** (us), **Facilitator** (Coinbase CDP). Three HTTP headers carry everything:

| Header | Direction | Payload |
|---|---|---|
| `PAYMENT-REQUIRED` | Server → Client | base64(`PaymentRequired` JSON) |
| `PAYMENT-SIGNATURE` | Client → Server | base64(`PaymentPayload` JSON) |
| `PAYMENT-RESPONSE` | Server → Client | base64(`SettlementResponse` JSON) |

### 1.1 The wire flow (authorization flow — the default and our recommendation)

```
Client                Resource Server (AWS)          Facilitator (Coinbase CDP)
   │ 1. GET /v1/verify-license?fn=123 (no payment)        │
   ├─────────────────────────────────────────►            │
   │ 2. HTTP 402 Payment Required                         │
   │    PAYMENT-REQUIRED: base64(PaymentRequired)         │
   │    ◄─────────────────────────────────────────────────│
   │ 3. Client picks an `accepts[]` entry (USDC/Base),    │
   │    signs EIP-712 authorization (EIP-3009 Transfer    │
   │    with Authorization), retries:                     │
   │    PAYMENT-SIGNATURE: base64(PaymentPayload)         │
   │ ├─────────────────────────────────────────►          │
   │ │ 4. POST /verify {paymentPayload, paymentRequirements}
   │ ├────────────────────────────────────────────────────►
   │ │ 5. {isValid:true, payer:0x…}  (read-only; free)    │
   │ │ ◄──────────────────────────────────────────────────┤
   │ │ 6. Execute the work (query DuckDB mirror)          │
   │ │ 7. POST /settle {paymentPayload, paymentRequirements}
   │ ├────────────────────────────────────────────────────►
   │ │ 8. {success:true, transaction:0x…, network:eip155:8453}
   │ │ ◄──────────────────────────────────────────────────┤
   │ 9. HTTP 200 + resource body                          │
   │    PAYMENT-RESPONSE: base64(SettlementResponse)      │
   │ ◄────────────────────────────────────────────────────│
```

Ordering variants defined by the spec: `authorization` (verify → resource → settle → respond — **default, use this**), `upfront` (settle → resource → respond — finality before work), `escrow` (settle → resource → settle). Invariant: at least one verify/settle check always runs *before* the resource executes.

### 1.2 `PaymentRequired` (what we return on 402)

Canonical location: the `PAYMENT-REQUIRED` header, base64 of:

```json
{
  "x402Version": 2,
  "error": "PAYMENT-SIGNATURE header is required",
  "resource": {
    "url": "https://api.permits-now.com/v1/verify-license",
    "description": "Verify a CA ABC license status against the official daily mirror",
    "mimeType": "application/json",
    "serviceName": "Permits N More API",
    "tags": ["permits", "licenses", "verification"],
    "iconUrl": "https://api.permits-now.com/icon.png"
  },
  "accepts": [
    {
      "scheme": "exact",
      "network": "eip155:8453",
      "amount": "250000",
      "asset": "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913",
      "payTo": "0x<MERCHANT_WALLET>",
      "maxTimeoutSeconds": 300,
      "extra": { "name": "USDC", "version": "2" }
    }
  ],
  "extensions": {}
}
```

Field semantics (v2 spec):
- `x402Version`: must be `2`. (V1 used an `X-PAYMENT` header — do not mix.)
- `resource.url`: the exact URL being sold. **We must later verify the echoed payload's resource matches the actual request** (binds payment to resource).
- `accepts[]`: each entry is an offer. Required fields: `scheme` (`exact` | `upto` | `batch-settlement`), `network` (CAIP-2, `eip155:8453` = Base mainnet, `eip155:84532` = Base Sepolia), `amount` (**atomic units** — USDC has 6 decimals, so $0.25 = `250000`), `asset` (ERC-20 contract; Base native USDC is `0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913` — confirm programmatically via facilitator `GET /supported`), `payTo` (our merchant wallet), `maxTimeoutSeconds` (payment window we'll accept; 300s is reasonable for agent wallets).
- We may offer **multiple `accepts[]` entries** (e.g., Base exact for agents, Stripe-fiat pseudo-entry later) — clients pick one.

### 1.3 `PaymentPayload` (what the client sends back)

```json
{
  "x402Version": 2,
  "resource": { "url": "https://api.permits-now.com/v1/verify-license", "description": "…" },
  "accepted": { "scheme": "exact", "network": "eip155:8453", "amount": "250000",
                "asset": "0x833589…", "payTo": "0x<MERCHANT_WALLET>",
                "maxTimeoutSeconds": 300, "extra": { "name": "USDC", "version": "2" } },
  "payload": {
    "signature": "0x…",   /* EIP-712 signature over the authorization */
    "authorization": {
      "from": "0x<BUYER>", "to": "0x<MERCHANT_WALLET>", "value": "250000",
      "validAfter": "1755600000", "validBefore": "1755600300",
      "nonce": "0x<32-byte-random>"
    }
  },
  "extensions": {}
}
```

Key point: **only the inner `authorization` + `signature` are cryptographically signed by the payer.** The outer `accepted`/`resource` are echoed by the client and **must be re-validated by us byte-for-byte against the offer we issued** (amount, asset, network, payTo, scheme) — a malicious client can echo different values in `accepted`, but the signed `authorization.to` and the facilitator's signature/recipient checks catch fund-diversion attempts.

### 1.4 Verification and settlement (facilitator interface)

Facilitators expose (v2 spec §7):

- **`POST /verify`** — read-only validation: signature validity, amount match, recipient match, nonce not yet used, balance sufficient, time window valid. **Always free.** Response `{isValid, invalidReason?, payer?}`. Error codes incl. `insufficient_funds`, `invalid_exact_evm_payload_signature`, `invalid_exact_evm_payload_recipient_mismatch`, `invalid_exact_evm_payload_authorization_valid_before`, etc.
- **`POST /settle`** — durably commits payment state (broadcasts the EIP-3009 transfer). Response `{success, errorReason?, payer, transaction, network}`.
- **`GET /supported`** — programmatic source of truth for schemes/networks/signers.
- New in 2026: **`settlement_pending`** — non-terminal error: tx was broadcast but confirmation couldn't be established (RPC timeout). Response carries the tx hash for reconciliation. **Serverless platforms must bound the receipt wait below their request deadline** (see §2.3).

EIP-3009 (`transferWithAuthorization`) semantics matter for security: the nonce is unique per authorization and **consumed on-chain on first use**, and the signature binds `to` (our address) — so the same signed payload can never pay twice, and no third party can redirect funds. The remaining app-layer risk is *us* executing paid work twice for one authorization (replay against two of our endpoints) — handled by the ledger (§5).

### 1.5 What "paymaster" means here

x402 has no separate paymaster call. The **facilitator plays the paymaster role**: it submits the transfer and covers settlement gas as part of its service. The **EIP-2612 gas-sponsorship extension** (supported by CDP Facilitator) additionally sponsors the buyer's first Permit2 approval so buyers don't need native Base ETH for the approval tx. Practical consequence: buyers only need USDC in a wallet — no ETH required — which is exactly what you want for agent buyers.

### 1.6 Transport notes

- `PAYMENT-RESPONSE` returns `200 OK` + resource body on success; on verification/settlement failure, server returns `402` again with error details (client retries after fixing, e.g., funding the wallet).
- MCP transport also exists (paid MCP tools) — relevant later for the 14-tool MCP server (§4.4).
- Some HTTP middleware rewrites unknown status codes/headers — verify 402 pass-through in your proxy chain (see §2.2).

---

## 2. Recommended AWS topology

### 2.1 Reference architecture

```
                    ┌──────────────────────── AWS (us-west-2 recommended) ────────────────────────┐
                    │                                                                             │
 Buyer (agent/human)│                                                                             │
 ┌──────────────┐   │  ┌───────────────────────┐   402/PAYMENT-SIGNATURE    ┌──────────────────┐  │
 │ wallet (USDC)│───┼─▶│ API Gateway HTTP API  │◄──────────────────────────▶│ Lambda:          │  │
 │ CDP Agentic  │   │  │ api.permits-now.com    │  route /v1/verify-license │ paid-resource.py  │  │
 │ Account /    │   │  │ throttling + WAF       │  /v1/search …             │ (x402 middleware)│  │
 │ browser      │   │  └───────────┬───────────┘                           └───┬───────┬───────┘  │
 └──────────────┘   │              │                                           │       │          │
                    │              │                                           │       │          │
                    │              ▼                                           ▼       ▼          │
                    │  ┌─────────────────────┐                 ┌───────────────────────────────┐ │
                    │  │ DynamoDB ledger       │◄──────────────▶│ DuckDB mirror (read-only,      │ │
                    │  │ payments / nonces /   │   query API     │ existing abc-agent data)      │ │
                    │  │ prices (single table) │                 └───────────────────────────────┘ │
                    │  └───────────┬───────────┘                                                   │
                    │              │ EventBridge (ledger state transitions)                        │
                    │              ▼                                                               │
                    │  ┌───────────────────────┐     ┌────────────────────────────────────────┐   │
                    │  │ SQS payment-reconcile  │────▶│ Lambda reconciler: reconcile settle    │   │
                    │  │ (DLQ attached)         │     │ pending, retry /settle, emit receipts  │   │
                    │  └───────────────────────┘     └────────────────────────────────────────┘   │
                    │                                                                             │
                    │  Secrets Manager (CDP API key, Stripe key) ◄── KMS CMK; IAM least-privilege │
                    │  CloudWatch: alarms (stuck settlements, verify-fail spikes, ledger drift)    │
                    │  EventBridge scheduled rule → daily settlement/revenue rollup → S3           │
                    └─────────────────────────────────────────────────────────────────────────────┘
                                   │                        │
                                   ▼                        ▼
                    ┌───────────────────────┐   ┌────────────────────────┐
                    │ Coinbase CDP          │   │ Stripe (parallel fiat  │
                    │ x402 Facilitator      │   │ rail: PaymentIntent /  │
                    │ /verify /settle, KYT, │   │ x402 preview; refunds, │
                    │ gas, Bazaar indexing  │   │ fiat payout)           │
                    └───────────────────────┘   └────────────────────────┘
```

### 2.2 Service-by-service decisions

| Concern | Choice | Rationale |
|---|---|---|
| API surface | **API Gateway HTTP API** (REST API v2-style) | Cheaper than REST API, native 402 pass-through with Lambda proxy, built-in throttling. Keep REST API option if you need usage plans/API keys for Stripe-era human dev accounts. |
| Compute | **Lambda (Python 3.11+)** | Codebase is Python (abcgov/, cdp-sdk python `cdp.x402` module + x402 Foundation Python SDK both exist). Stateless, per-call billing matches per-call metering. ECS/Fargate only if a future endpoint needs >29s synchronous work. |
| Ledger | **DynamoDB single table** | Idempotency via conditional writes is native (`ConditionExpression` on nonce PK); TTL for expired-authorization cleanup; no schema migration for a young ledger. Postgres (Aurora/RDS) is the alternative if the owner wants SQL revenue analytics — see Open Decisions D3. |
| Async settlement | **EventBridge → SQS → Lambda reconciler (+DLQ)** | `settlement_pending` and webhook fan-out; SQS gives retry with DLQ. |
| Secrets | **Secrets Manager + KMS CMK**, fetched at cold start | CDP API key (key ID + PEM private key) and Stripe secret key never in env vars or repo (AGENTS.md rule 3). |
| Observability | CloudWatch logs + metrics + alarms; X-Ray optional | Alarm on: payments stuck in `settling` > 15 min, `verify` failure spike (abuse), ledger/on-chain drift. |
| Revenue data | EventBridge daily rule → rollup to S3 (Parquet) | Feeds accounting/off-ramp decisions; keeps DuckDB mirror untouched. |
| Edge | (later, optional) CloudFront + WAF or Cloudflare | Owner has Cloudflare Monetization/Wallet signups — Cloudflare Monetization Gateway is **waitlist-only** (July 2026 announcement); treat as Phase 3 experiment, not a dependency. |

### 2.3 Deadline math (important, serverless-specific)

API Gateway HTTP API integration timeout is **30s**; REST API ~29s. The facilitator's `/settle` receipt-wait must be bounded **below** that: per x402 docs, `confirmationTimeoutMs` (TS) / `confirmation_timeout_seconds` (Python) — set **~25s**. If confirmation can't be established, the facilitator returns `settlement_pending` with a tx hash; we return `200` + `PAYMENT-RESPONSE` with that hash, and the **reconciler** verifies on-chain and flips the ledger row. Never let the Lambda wait on the receipt past ~25s or the caller gets a 5xx with no hash to reconcile against.

Work execution must also stay well under the deadline: DuckDB queries against the 128k-row mirror are single-digit ms — fine.

### 2.4 Idempotency design

Two independent layers:

1. **Protocol-level (on-chain):** EIP-3009 nonce is consumed once on-chain. The same signed payload can never move funds twice.
2. **Application-level (our ledger):** before executing *any* paid work, **conditional insert** on `PK = network#nonce`. If the nonce already exists → `409`/`402 invalid` without doing work. This stops the "one signed payload, two of our endpoints" replay and the "client retried the same request twice" duplicate-work case. Additionally, buyers can send an `Idempotency-Key` header (our convention, echoed in the ledger) so legitimate retries of the same logical call after a network blip return the same result without double-charging.

### 2.5 Ledger schema (DynamoDB, single table)

```
payments
  PK:  network#nonce            (e.g. "eip155:8453#0xf374…")   ← replay guard
  SK:  requestId                (our UUID; = idempotency key)
  attrs: status [challenged|verified|executing|settling|settled|settlement_pending|failed|refunded]
         resourceUrl, amountAtomic, asset, network, payer, payTo, scheme,
         txHash?, verifyAt, settleAt, correlationId, errorReason?, TTL (auth window expiry)
prices
  PK:  resourceUrl              → {usdPrice, usdcAtomic, scheme, active}
  (cache in Lambda; source of truth here so price changes are instant, no deploy)
```

Settlement reconciler transitions: `settling → settled` (on-chain confirmed) / `settlement_pending → settled | failed` (verify by hash) / `failed` → optionally notify buyer + issue seller-originated refund transfer (a refund is a *new* push payment, per §5.5).

---

## 3. Coinbase integration options — tradeoffs

| Option | What it is | Verdict for permits-agent |
|---|---|---|
| **CDP x402 Facilitator** (recommended) | Hosted merchant-side rail: validates signed payments, **OFAC/KYT screening**, submits settlement, manages gas, indexes you in the **Bazaar** discovery marketplace. Python SDK (`cdp.x402` → authenticated `HTTPFacilitatorClient`), or direct REST `verify`/`settle` APIs. Schemes: `exact`, `upto`, `batch-settlement` on Base/Polygon/Arbitrum/World; `exact` on Solana. Pricing: **1,000 onchain tx/month free, then $0.001/tx; verification always free**; `batch-settlement` spreads one on-chain tx across thousands of vouchers. 100M+ tx / $28M volume processed. | **Use this.** It is the x402 merchant rail with the least ops: no RPC, no signer, no gas management, KYT included. Matches our Python stack. Free tier covers Phase 0–2 volume. |
| **Coinbase Commerce / Coinbase Business** | Human checkout product: hosted payment page, payment links, card + crypto acceptance; being folded into Coinbase Business (2026). It is a **checkout**, not an x402 402-negotiation endpoint. | **Not for the agent rail.** Revisit only when ComplianceOS ships a human-facing web portal (buy verification credits, monthly subscription). Can share the same merchant wallet/off-ramp. |
| **AgentKit / CDP SDK (agentic accounts)** | Buyer-side toolkit: provision agent wallets (MPC-managed or self-custody), budgets, spend controls; Agentic Accounts = pay for x402 services from an agent-owned account. | **Not a merchant rail.** Relevant later for permits-agent *as a buyer* (agents paying upstream data vendors, LLM APIs) and for our own digest cron / MCP clients paying us. Also the mechanism to *test* our own seller flow cheaply. |
| **Raw x402 + self-facilitate** | Run the reference facilitator ourselves (EVM signer, RPC, gas, nonce/`SettlementCache`, KYT by hand). No facilitator fee. | Reject for now: we'd own RPC uptime, gas, signer security, KYT, and replay caches — the exact things CDP gives us for $0.001/tx. Revisit only if Coinbase becomes a compliance blocker or volume makes the fee material. |
| **Stripe x402 (private preview)** | Stripe turns x402 payments into PaymentIntents: capture on-chain USDC, funds land in Stripe balance with Stripe reporting, refunds, fiat payout. USDC on Base, **US eligibility + enablement required**, **1.5% per successful charge** (min ~$0.01 charges). | **Keep as the second rail**, not the primary: gives us the fiat/refund/accounting story and satisfies the owner's "x402 + Stripe" intent. Qualify for preview; in the meantime the parallel human rail is Stripe-hosted checkout for pre-paid credit. |
| Other facilitators (Circle Gateway nanopayments: batched USDC settlement down to $0.000001; thirdweb 0.3%; PayAI 10k/mo free) | Alternatives/backups. | Not needed at Phase 0. Circle is the answer if a future product needs sub-cent-per-call pricing at volume (see §4.3). |

**Recommended posture:** primary rail = CDP x402 Facilitator (`exact` scheme, USDC on Base mainnet, CDP-managed server wallet). Secondary rail = Stripe for humans/fiat. Both write to the same DynamoDB ledger via a small `PaymentRail` adapter interface (mirrors the repo's existing adapter pattern).

---

## 4. Metered usage billing → x402 micropayments

### 4.1 Pricing table (proposal — owner input needed, Decision D1)

| Paid resource (endpoint/tool) | Price (USD) | USDC atomic (6 dp) | Scheme |
|---|---|---|---|
| `POST /v1/verify-license` (verification API — anchor product) | $0.25 | 250000 | exact |
| `POST /v1/search` (advanced/compound search) | $0.10 | 100000 | exact |
| `POST /v1/lead-radar` (pending/expiring digest per zip, on demand) | $0.50 | 500000 | exact |
| Monthly agency watch pass (daily digest, up to N zips) | $49.00 | 49000000 | upto (cap) or session pass |
| Bulk/agency batch (e.g., 10k verifications) | $0.05/call | 50000 | batch-settlement |

Minting rule: `usdcAtomic = round(usdPrice * 10^6)`; prices live in the DynamoDB `prices` table and are stamped into every 402 `accepts[]` (plus `extra` metadata: `billing: "per-call"`).

### 4.2 Per-call micropayments (the core loop)

Each paid call is a complete 402 negotiation (§1.1). For agent buyers this is seamless: AgentKit/CDP or Cloudflare Agents SDK clients auto-handle the challenge, prompt the human once (or not at all), sign, retry. No accounts, no API keys, no card — the *identity* is the wallet.

The **exact** scheme is the right default: fixed price per call, verified before work, settled after work (`authorization` flow). Work here is cheap, idempotent, read-only — so "verify → work → settle" risk (settle fails after free work) is bounded by the cost of one cheap DuckDB query. For expensive or non-idempotent future endpoints, switch that endpoint to the **upfront** flow (settle → work → respond).

### 4.3 When per-call on-chain tx stops making sense

At our anchor price ($0.25) the math is fine: Coinbase fee $0.001/tx (free under 1k/mo) + Base gas ≈ $0.001–0.003 → **~0.6% cost of goods**. But if the product ever goes sub-cent-per-call (high-volume verification feeds), one on-chain tx per call is irrational. Two escapes, both supported by CDP:
- **`batch-settlement` scheme:** buyers/merchants accumulate signed vouchers off-chain (verification free); one on-chain transaction claims thousands. 
- **Circle Gateway Nanopayments** (batched netting, payments down to $0.000001) as a later swap-in.

Keep the pricing table + ledger rail-agnostic so this is a config change, not a rewrite.

### 4.4 MCP-transport paid tools (ComplianceOS-native)

The x402 MCP transport exists (paid tools/resources over MCP). The 14-tool MCP server could mark tools `paidTool` (Cloudflare-style: `server.paidTool("verify_license", …, 0.25, …)`), and compatible MCP clients handle the 402. Caveat (verify before building): mainstream MCP hosts' built-in payment handling is still emerging in 2026; our own CLI/digest cron can be an x402-capable client regardless (buyer-side, using Agentic Accounts).

---

## 5. Security considerations

### 5.1 Payment-proof verification (defense in depth)

1. **We re-validate the echoed `accepted` object** against the exact `PaymentRequired` we minted: scheme, network, amount (atomic), asset contract, `payTo` — byte-for-byte. A client can echo anything in `accepted`; only the signed `authorization` is trustworthy.
2. **Facilitator `/verify`** independently checks: EIP-712 signature validity, `authorization.to == requirements.payTo` (recipient mismatch), exact amount match, `validAfter ≤ now ≤ validBefore` (with skew tolerance), nonce unused, balance sufficient. Always call it — it's free and it's the second opinion.
3. **Bind payment to resource:** the signed authorization doesn't name the endpoint; *we* bind it by requiring `payload.resource.url` to equal the URL actually requested and by the ledger nonce guard. `exact` means the amount is also fixed per endpoint.
4. **Never trust the client's `PAYMENT-RESPONSE`-adjacent fields;** only the facilitator response and on-chain state.

### 5.2 Replay protection

- **On-chain (protocol):** EIP-3009 nonce consumed on first use; contract rejects reuse. Time window (`validAfter`/`validBefore`) bounds authorization lifetime — mint offers with `maxTimeoutSeconds: 300` and enforce skew < 30s.
- **At the ledger (app layer):** conditional insert on `PK=network#nonce` *before* executing work — the "same payload, two endpoints" and "retried call" cases die here.
- **Settlement level:** don't call `/settle` twice for the same payload; track `settleStatus` in the ledger. (Solana's duplicate-settlement race is documented in the spec; we're on Base/EVM where the nonce makes double-settle revert — still guard the call site.)
- 402 challenges are cheap and stateless — a flood of unpaid challenges is just a rate-limit problem (API Gateway throttle + WAF), not a fund-loss problem. **Only `/verify`+work+`/settle` paths touch money.**

### 5.3 Merchant key / wallet handling

- **CDP API credentials** (API key ID + PEM private key) in **Secrets Manager**; Lambda IAM role gets `secretsmanager:GetSecretValue` + `kms:Decrypt` on the specific secret only; fetch at cold start, hold in memory; **never** in env vars, logs, or the repo (AGENTS.md rule 3). Rotate quarterly; restrict the key to facilitator scopes (`x402` verify/settle), not portfolio-wide.
- **Merchant wallet custody — Decision D2:** prefer a **CDP-managed server wallet** (Coinbase MPC custody) so no private key for funds exists in our AWS account at all; `payTo` is just an address. Self-custody alternative: signer key in KMS-backed Secrets Manager — more control, more liability. Either way, a **separate low-balance hot wallet** for settlement testing; sweep to cold/off-ramp on schedule.
- Facilitator never takes custody of buyer funds (it only relays the signed transfer); KYT/OFAC screening is Coinbase's (built into CDP Facilitator) — keep its `transaction`/payer records for audit.
- AWS-side: VPC not needed for public API; keep Lambda IAM scoped to `dynamodb` (payments/prices), `secretsmanager` (2 secrets), `sqs`/`eventbridge` (reconciler), `s3` (rollup). No broad `*`.

### 5.4 Abuse & availability

- API Gateway throttling (per-key then global), WAF rate rule on the paid routes; CloudWatch alarm on `verify`-failure spike (someone replaying/abusing).
- Buyers with insufficient funds get `insufficient_funds` — friendly 402 with error, not 5xx.
- DoS via huge `PAYMENT-SIGNATURE` headers → validate base64 + size caps in middleware before any crypto work.

### 5.5 Refunds & disputes (policy decision — Decision D3-adjacent)

x402 `exact` payments are **push payments**: irreversible once settled. A refund is a **new, seller-originated USDC transfer** back to `payer` (facilitator counts it as one more on-chain tx, $0.001). Policy: no-refund for served verification responses (data is delivered as requested); refund on provable double-charge or settle-after-work failure. The Stripe rail, once live, is where refunds become self-serve for human buyers. Document the dispute path in the terms page referenced from `resource.iconUrl`/metadata.

### 5.6 Compliance notes (flag, don't guess)

Merchant KYC/KYT (Coinbase business onboarding), OFAC screening (facilitator), tax/accounting treatment of stablecoin revenue, and CA ABC data redistribution licensing (our data is public records — but *selling access* has its own terms-of-use questions; AGENTS.md rule 6 applies: flag regulatory questions to the owner, don't self-answer).

---

## 6. Phased implementation plan

**Phase 0 — smallest shippable slice (1–2 weeks). Prove the loop.**
- [ ] Create `docs/payments/` runbook + this doc; pricing table in code.
- [ ] CDP sandbox project + testnet API key; CDP-managed test wallet funded with Base Sepolia USDC (faucet).
- [ ] One paid endpoint: `POST /v1/verify-license` (existing `abcgov` query logic). Lambda + API Gateway HTTP API, Python, `cdp.x402` facilitator client pointed at **Base Sepolia** (`eip155:84532`).
- [ ] x402 middleware: mint `PaymentRequired` on no-`PAYMENT-SIGNATURE`; parse/validate payload; `/verify`; execute; `/settle`; emit `PAYMENT-RESPONSE`.
- [ ] DynamoDB ledger v1 (conditional nonce insert + status transitions) — no reconciler yet.
- [ ] Buyer side: test with CDP Agentic Account / test wallet + `curl` scripted flow; verify 402 → sign → 200 and the failure paths (expired, replayed, wrong amount).
- [ ] Verify AGENTS.md expectations still pass (this is additive; no changes to `abcgov/` data path).

**Phase 1 — mainnet + reliability (weeks 3–4).**
- [ ] Base mainnet (`eip155:8453`), production CDP key, real merchant wallet address in `prices` table.
- [ ] EventBridge → SQS → reconciler Lambda: handle `settlement_pending` (reconcile by tx hash), retries, DLQ, CloudWatch alarms.
- [ ] `Idempotency-Key` support; Webhook/export hook (S3 rollup of daily settlements).
- [ ] Stripe rail v1 (parallel): hosted-checkout credits or x402 preview (if qualified) → same ledger via `PaymentRail` adapter.
- [ ] Internal + friendly-buyer beta: 2–3 agents/humans paying real $0.25 verifications; KYT + receipts reviewed.

**Phase 2 — productization (month 2).**
- [ ] Metering variants: `upto` for watch passes, `batch-settlement` for bulk/agency; revisit unit economics at volume.
- [ ] Bazaar listing (CDP facilitator indexes us → discovery for agent buyers); builder-code attribution optional.
- [ ] Paid MCP tools on the 14-tool server (x402 MCP transport) with CLI/digest as reference client; Agentic Accounts for our cron as a buyer.
- [ ] Refund workflow (seller-originated transfer), audit export (S3 Parquet → accounting).

**Phase 3 — scale/compliance (month 3+, gated on revenue).**
- [ ] Fiat off-ramp (Coinbase/Stripe payout), tax/accounting integration, dispute SLA.
- [ ] If volume demands: CloudFront + WAF in front, or Cloudflare Monetization Gateway (if GA), Circle nanopayments swap-in for sub-cent SKUs.
- [ ] Optional: self-facilitation evaluation only if facilitator fee > ops cost at scale.

**Exit criteria for each phase:** real money moved, ledger matches on-chain (reconciliation drift = 0), replay/double-charge tests green, runbook updated.

---

## 7. Open decisions needing owner input

- **D1 — Price points & SKUs.** Confirm the §4.1 table (especially the $0.25 verification anchor) and whether watch passes (`upto`) are in-scope for Phase 1 or Phase 2.
- **D2 — Merchant wallet custody.** CDP-managed (MPC, recommended, no key in AWS) vs self-custody KMS key. Also confirm the CDP business onboarding/KYC is done (or who owns it).
- **D3 — Ledger store & Stripe sequencing.** DynamoDB (recommended, fast) vs Postgres (SQL analytics) for the ledger; and whether the Stripe rail ships in Phase 1 (parallel) or after x402 is proven.
- (Sub-decisions flagged, not blocking: refund policy wording; whether to list on Bazaar; buyer support channel for humans paying via agents.)

---

## 8. Sources (verified Aug 2026)

- x402 Foundation — spec V2 + HTTP transport: github.com/x402-foundation/x402 (`specs/x402-specification-v2.md`, `specs/transports-v2/http.md`) — V2.0 2025-12-09, active commits 2026-08-17
- docs.x402.org — facilitator concepts, `settlement_pending`/EVM timeout guidance, facilitator paths
- Coinbase CDP x402 docs — docs.cdp.coinbase.com/x402 (seller/facilitator: pricing 1,000 tx/mo free then $0.001, networks/schemes table, EIP-2612/Bazaar/builder-code extensions, KYT)
- Coinbase launch blog — "Introducing x402" (coinbase.com/developer-platform/discover/launches/x402)
- Cloudflare blog — "Launching the x402 Foundation…" (2026; Monetization Gateway waitlist, deferred scheme proposal)
- Wavect — "x402 Payments in 2026: Coinbase, Stripe, Cloudflare, AWS and Alternatives Compared" (verified 2026-07-12: facilitator landscape, Stripe 1.5% preview, Circle nanopayments, AWS AgentCore buyer-side)
- aws-samples/sample-secure-agentic-payments-on-aws-x402 (governance layer: spend ledger, nonce replay guard, budget/allowlist patterns)
- Linux Foundation — x402 Foundation announcement (2026-04-02, MCP Dev Summit NA)
