# Cloudflare Monetization Gateway + Cloudflare Wallet + cloudflare.pay — Bridge & Decision Doc

**Product:** permits-agent / ComplianceOS (BC Software)
**Status:** Decision-support · last verified against live docs **Aug 19, 2026**
**Scope:** How Cloudflare's agentic-payments stack (Monetization Gateway, Wallets,
cloudflare.pay) connects to the AWS + Stripe + x402/Coinbase payment architecture.
Companion docs: `docs/architecture.md`, `AGENTS.md` (repo rules apply: no fabricated
data, no credentials in repo, regulatory review before charging for filing services).

---

## 0. Verdict (TL;DR)

1. **Cloudflare Wallets + cloudflare.pay are buyer-side, announced Aug 4, 2026 (Agents Week) — today only free handle reservation is live.** Wallet funding, agent spending (Virtual Wallets), and fiat on/off-ramps arrive "in the coming months"/"later in 2026."
2. **Monetization Gateway is seller-side, announced July 1, 2026 — waitlist-only, no pricing, no GA date.** Cloudflare is *not* in the settlement path (payments settle peer-to-peer to the merchant's wallet); the Gateway only gates, prices, and verifies at the edge.
3. **x402 is ONE protocol shared by Cloudflare and Coinbase** — Coinbase created it, both are founding members of the neutral Linux Foundation **x402 Foundation** (operational Apr 2, 2026; members include Visa, Mastercard, Amex, Google, Shopify, **Stripe**, **AWS**, Circle, Adyen, Fiserv). Cloudflare's own docs use **Coinbase's public facilitator in every example** — wire-level interoperability is guaranteed and already tested.
4. **Recommended role for Cloudflare:** edge gatekeeper (sell side) + guardrailed buyer wallet for *our* agents (buy side) + payer-identity layer (cloudflare.pay handles) for *our verification-API customers' agents*. **AWS stays the origin; Stripe stays the revenue spine; x402/USDC is a parallel machine rail — not a replacement.**
5. **Market reality check:** x402.org self-reports 75.4M tx / $24.2M / 94K buyers / 22K sellers in the last 30 days, but independent Helios data (Aug 12, 2026) shows **daily settlement ~$28K, down 93% YTD** from the late-2025 testing wave (~$800K/day peak). Infrastructure is ahead of real demand; **merchant supply (22K sellers) is the bottleneck**. Do not bet revenue on x402 yet — build the bridge, keep Stripe primary.

**Bottom line:** Claim the handle now, join the waitlist, spike with the x402-proxy Worker now (it works today), adopt Wallets + Gateway when they GA — all without touching the AWS origin or the Stripe workstream.

---

## 1. What Cloudflare's agentic-payments stack IS today (Aug 19, 2026)

| Component | Role | Status today (2026-08-19) |
|---|---|---|
| **Monetization Gateway** | Seller side: charge any caller for any resource behind Cloudflare (pages, datasets, API routes, MCP tool calls) | **Waitlist only** (since Jul 1, 2026). No pricing, no GA date. Planned: rules API (dashboard/API/Terraform), per-verb pricing, variable pricing up to a cap, 401→402 interception, verification at edge (330+ cities), sub-second settlement target |
| **Cloudflare Wallets** | Buyer side: stablecoin wallets with human Account Wallets + agent Virtual Wallets | **Handle reservation only.** Send/receive/hold funds, delegation, spending all "coming soon" |
| **cloudflare.pay** | Optional human-readable identity handle on top of a keypair (Web Bot Auth) | **Live.** 1 free handle per Cloudflare account, first-come-first-served, publishes `HANDLE.cloudflare.pay`, registers you for Wallets launch notice |
| **x402 tooling (Agents SDK)** | The *protocol-level* sell/buy machinery — works today **without** the Gateway | **Live.** `x402-proxy` Worker template, `x402-hono` middleware, MCP `paidTool`/`withX402`, `withX402Client` (buyer), Claude Code hook + OpenCode plugin, `@x402/fetch`, `@x402/evm` |
| **MPP (Machine Payments Protocol)** | Sibling protocol, same 402 flow, adds card rails (Stripe) + stablecoins | **Live** (Cloudflare docs since Aug 5, 2026). `mpp-proxy`, `mppx` middleware, paid MCP tools; MPP clients can consume x402 services |
| **Pay Per Crawl** | Charge AI crawlers for content | Private beta (since Jul 1, 2025) — separate, crawler-specific |
| **On/off-ramp banking** | Fiat ↔ stablecoin funding/redemption | **"Later in 2026"**; self-funding via stablecoins for eligible users as the near-term alternative |

### 1.1 Monetization Gateway — what it will do (announced, not shipped)

- Rules like every other Cloudflare rule: e.g. `$0.01 for every GET/POST to /api/premium/*`, variable pricing ("up to $2 depending on compute"), or *intercept existing 401s and return priced 402s*.
- Payment verification and enforcement happen **at the edge**, protecting the origin from payment-volume floods; origin keeps rules, prices, revenue.
- **It does not touch the money**: settlement is peer-to-peer on-chain, directly to the seller's wallet. Cloudflare runs the wallet/handle/onramp product but is not a settlement intermediary — so no CF custody or CF settlement risk on the sell side.
- Sellers will be able to redeem accumulated stablecoins for fiat "in their bank account" — timing unstated (part of the "later in 2026" on/off-ramp story).

### 1.2 Cloudflare Wallets — the two-wallet model (announced, not shipped)

- **Account Wallet** (human): add funds, delegate spend to Virtual Wallets, remove funds. May carry the cloudflare.pay handle.
- **Virtual Wallet** (agent, API-key operated): spends within guardrails set by the Account Wallet owner — **allowance, allow list, maximum transaction size** — with human review/override on anomalous spend.
- This is precisely the spend-control layer ComplianceOS needs for its own agents (see §3.2).

### 1.3 cloudflare.pay — what is real right now

- Reserve one free handle per Cloudflare account at cloudflare.pay (FCFS). It publishes a page, associates the name with the account, opts you into launch notifications.
- **Rules that matter for the owner decision:**
  - Reservation does **not** guarantee wallet access at release.
  - Cloudflare may reject or **reclaim** handles for any reason.
  - There is **no way to change, release, or move** a handle once reserved (including after a rebrand — contact support to ask).
- Strategic reading: like ENS/domain land-grabs. Reserving `bcsoftware` / `complianceos` / `permits-agent` protects the brand identity for when agent-commerce identity matters; the cost is zero and the downside is immutability (name you can't move).

### 1.4 What actually works TODAY (buildable now, no waitlist)

The protocol layer is fully shippable independent of the Gateway and Wallets:

- **Sell:** deploy the `x402-proxy` Worker (one-click template) in front of *any* HTTP backend — including an AWS origin — with routes/prices in `wrangler.jsonc`; or add `x402-hono` middleware to a Worker; or gate MCP tools with `paidTool`. Uses Coinbase's public facilitator (`https://x402.org/facilitator`) in all examples.
- **Buy:** `@x402/fetch` wrapper, `withX402Client` for MCP, Claude Code hook / OpenCode plugin — an agent pays a 402 with a signed payload and retries.
- **Identity:** Web Bot Auth keypair registration (GA) — the primitive the cloudflare.pay handle will make human-readable.

---

## 2. x402: the shared protocol — Cloudflare ↔ Coinbase compatibility

### 2.1 Same protocol, neutral governance

- x402 is **one open standard** (spec: x402.org, repo: x402-foundation/x402). Coinbase published the first reference implementation (May 2025) and **contributed it to the x402 Foundation**; governance now lives at the Linux Foundation (announced Sep 23, 2025 by Cloudflare+Coinbase; operational Apr 2, 2026 at the MCP Dev Summit).
- Members include Visa, Mastercard, American Express, Google, Shopify, **Stripe**, **AWS**, Circle, Adyen, Fiserv, Solana Foundation, Stellar, Ripple, MoonPay, Monad — Cloudflare is a Premier/founding member alongside Coinbase.
- **Interop proof:** Cloudflare's official x402 docs use **Coinbase's public facilitator** in all examples, and Cloudflare's Agents SDK ships `@x402/*` SDKs built against the same spec. There is exactly one wire protocol; Cloudflare and Coinbase are two vendors of infrastructure around it.

### 2.2 The exchange (one protocol, both vendors)

```
1. Client → GET /resource                     (no payment)
2. Server → 402 Payment Required
            PAYMENT-REQUIRED: {price, token, network, merchant_address}
3. Client signs payload; retries
            PAYMENT-SIGNATURE: <base64 signed payload>
4. Server verifies (facilitator POST /verify, or own RPC),
   broadcasts settlement (facilitator POST /settle, or own broadcast)
5. Server → 200 + resource + PAYMENT-RESPONSE: <tx hash / receipt>
```

### 2.3 Merchant requirements to accept x402 (what "compatible" demands of us)

- **A wallet address per network** you accept (e.g., Base USDC address). That's the KYC-free minimum.
- **A facilitator for production**, or self-hosted RPC verification:
  - Coinbase CDP facilitator: **1,000 settled tx/month free, then $0.001/tx**; networks Base, Polygon, Arbitrum, World (EVM) + Solana (SVM). Performs KYT screening.
  - ⚠️ The public `x402.org/facilitator` used in Cloudflare examples is **testnet-only** — production needs CDP credentials or another facilitator (multi-facilitator ecosystem exists; Stellar shipped a production facilitator Mar 2026).
- **Schemes:** `exact` (fixed amount; EVM, Solana, Aptos, Stellar, Hedera, Sui) and `upto` (max authorization, charged at settlement; EVM) — relevant if we ever meter usage-based machine pricing.
- **Networks** (Cloudflare docs): Base, Ethereum, Polygon, Optimism, Arbitrum, Avalanche, Solana, Aptos, Stellar, Sui. **USDC is the default settlement asset.** Base settles ~2s, Solana ~0.5s, gas fractions of a cent.
- **No chargebacks** — settlement is irreversible; there is no dispute window on the rail itself.

### 2.4 MPP — the card-friendly sibling (the eventual Stripe↔x402 bridge)

- Machine Payments Protocol (mpp.dev) is the same HTTP-402 pattern with `WWW-Authenticate: Payment` / `Authorization: Payment` headers, and supports **non-crypto methods including cards via Stripe**, plus stablecoins; `mppx` SDK handles one-time, usage-based, and recurring payments; **MPP clients can consume x402 services without modification**.
- Cloudflare's Agents SDK supports **both** x402 and MPP today. If we later want per-call pricing that humans can pay with a card at the edge, MPP+Stripe is the path — it is the natural meeting point of our two workstreams, but it's a Phase-4 candidate, not a today decision.

### 2.5 AWS-side reality

- **AWS is an x402 Foundation member**, and per Coinbase, AWS has enabled x402 AI-traffic monetization across **Amazon CloudFront + WAF** using Coinbase's facilitator (Coinbase-reported). So an all-AWS x402 path exists as a fallback if we ever want zero Cloudflare involvement — but it would not change the protocol, the facilitator, or the merchant wallet requirements at all. The protocol layer is portable; only the gatekeeper placement differs.

---

## 3. Recommended role for Cloudflare in the AWS + Stripe + x402 architecture

### 3.1 Division of labor (the clean way to think about it)

| Layer | Owner | Handles |
|---|---|---|
| Origin, compute, data, identity, SaaS billing ledger | **AWS** (ECS/Lambda + API Gateway + DB) | permits-agent adapters, DuckDB mirror services, app logic — **unchanged** |
| Edge/CDN + payment gatekeeper + agent identity | **Cloudflare** (already fronting websites today) | TLS/CDN/caching; 402 challenge, payment verification, Web Bot Auth; later: Monetization Gateway rules |
| Human money | **Stripe** | Subscriptions, invoices, one-time card payments — **unchanged, revenue spine** |
| Machine money | **x402 + Coinbase (facilitator) / Cloudflare Wallets (buyer)** | Stablecoin micropayments, agent spend guardrails, payer identity |
| Bridge protocol (optional, later) | MPP via Cloudflare Workers | Cards-at-the-edge for machine-style per-call pricing |

### 3.2 The recommended bridge — concrete

**Sell side (customers' agents pay US — verification API, MCP tools):**
- Keep everything on AWS. Put Cloudflare edge in front of the *paid* routes only: **now** with the `x402-proxy` Worker (deployable today, origin-agnostic), **later** with Monetization Gateway rules (no origin change either way — that's the entire point of the product).
- Production settlement via **Coinbase CDP facilitator** (testnet `x402.org/facilitator` for spikes). Merchant wallet = a BC-software-owned Base USDC address per product line.
- Require (paid tiers) or encourage (free tiers) **Web Bot Auth + cloudflare.pay handle** from calling agents → org attribution with no KYC, allow-listable, priceable.

**Buy side (OUR agents pay other APIs):**
- **Cloudflare Wallets are the recommended mechanism once GA**: one Account Wallet for BC Software + one Virtual Wallet per agent (data-refresh bots, monitoring agents, e-filing prep) with allowance/allow-list/max-tx guardrails. This is exactly the "give every agent $10–$100 and let it explore APIs" model — ideal for ComplianceOS agents probing agency portals and paid data feeds.
- **Until GA:** self-custody Base USDC wallet + `@x402/fetch` / `withX402Client` / Claude Code hook inside our agent runtime, plus an **internal spend ledger** (our own allowance enforcement in the meantime).
- **cloudflare.pay handle = payer identity for our verification-API customers' agents:** when a customer's compliance agent calls our paid endpoint, the handle tells us which org it acts for. We can then offer trials, allow lists, and tiered pricing to *identified* agents while gating anonymous ones — mirroring how CF suggests VPN-like trust ("unidentified ≠ untrusted, but must prove more").

**Stripe stays the spine:** subscriptions and human payments unchanged. USDC/x402 revenue is additive and capped — a parallel rail for agent-native buyers, reconciled in the same billing ledger.

### 3.3 What connects to what

```
                        ┌──────────────────────────────────────────────┐
                        │          ComplianceOS (AWS origin)           │
                        │  adapters · DuckDB mirror · MCP server       │
                        │  billing ledger · Stripe subscriptions       │
                        └──────────────▲───────────────────────────────┘
                                       │ paid requests, origin unchanged
                        ┌──────────────┴───────────────────────────────┐
                        │      Cloudflare edge (CDN — already there)   │
                        │  x402-proxy Worker / Monetization Gateway    │
                        │  Web Bot Auth + cloudflare.pay handle checks │
                        └───────▲──────────────────────────▲───────────┘
                                │ 402 challenge + verify   │ signed payment payload
                     ┌──────────┴──────────┐    ┌──────────┴──────────┐
                     │ SELL: customers'    │    │ BUY: OUR agents     │
                     │ agents pay us       │    │ pay other APIs      │
                     │  (Cloudflare        │    │  (Cloudflare Wallet │
                     │   Wallet buyer,     │    │   Virtual Wallets   │
                     │   or any x402       │    │   w/ allowances —   │
                     │   wallet)           │    │   or @x402/fetch    │
                     └──────────┬──────────┘    │   until GA)         │
                                │               └──────────┬──────────┘
                     ┌──────────┴──────────┐    ┌──────────┴──────────┐
                     │ Coinbase CDP        │    │ Any x402 merchant:  │
                     │ facilitator         │    │ 402 + PAYMENT-      │
                     │ (/verify,/settle)   │◄───┤ REQUIRED endpoint   │
                     └──────────┬──────────┘    └─────────────────────┘
                                │ settlement (P2P, no Cloudflare in path)
                     ┌──────────▼──────────┐
                     │ On-chain USDC (Base │   ← Stripe remains separate:
                     │ default; Solana…)   │     human cards/subscriptions
                     └─────────────────────┘     (Phase 4: MPP bridges
                                                 cards ↔ 402 at the edge)
```

### 3.4 ComplianceOS product mapping

| Revenue line | Payable by | Rail |
|---|---|---|
| Subscription (monitoring, digests, radar) | Humans | **Stripe** (unchanged) |
| Verification API / MCP tool per-call | Agents | **x402/USDC** (Cloudflare edge, CF Wallet buyers) |
| Document assembly, guided e-filing | Humans + agents | **Stripe** primary; x402 later — e-filing payments stay human-approval-gated per AGENTS.md |
| Premium data exports / reports | Agents | x402 (candidate Phase 3) |

---

## 4. Risks & dependency concerns

1. **On/off-ramp not ready ("later in 2026")** — we can *hold* USDC but converting to fiat is manual until then. Requires stablecoin custody, valuation/tax accounting, and a decision on how much float we're comfortable with. **Mitigation:** Stripe-primary revenue; USDC receipts are an experiment with caps, not the P&L.
2. **Merchant-side adoption bottleneck (the big one).** Hard numbers (Aug 2026): x402 has ~22K sellers vs 94K buyers; independent settlement ~$28K/day, **down 93% YTD** — the late-2025 spike was testing. If our paid verification API is **x402-only**, customers without a crypto wallet literally cannot pay us. **Mitigation:** dual rails (Stripe for humans, x402 for agents); price x402 tiers as additive; don't count on agent revenue before Phase 3/4.
3. **Facilitator concentration + testnet-only public facilitator.** Production settlement means a Coinbase CDP account (1k tx/mo free, then $0.001/tx) or self-hosted RPC — a real vendor dependency, though the protocol itself is neutral and multi-facilitator. **Mitigation:** abstract the facilitator behind one internal module (swap Coinbase CDP → any facilitator implementing /verify,/settle).
4. **Cloudflare lock-in:**
   - Monetization Gateway rules = CF-proprietary control plane (config-as-code via API/Terraform, but the enforcement is Cloudflare). Portable in spirit, not in mechanism.
   - Wallet = Cloudflare custody. The **handle is immutable** — once reserved you cannot change/release/move it (even after rebrand), and CF may reclaim it. Reserve deliberately.
   - The *protocol* assets (wallet keys, x402 endpoints, MPP routes) are **portable** because x402/MPP are neutral standards.
5. **CF ↔ AWS overlap.** Both Cloudflare and AWS CloudFront can host x402 monetization at the edge. **Do not double-gate** (one 402 handshake per request; one gatekeeper per asset). Default: Cloudflare (already in the path). Fallback: AWS-native CloudFront+WAF+x402 if CF pricing/terms disappoint.
6. **Regulatory (ComplianceOS-specific).** Charging for verification data = monetizing derivatives of public government data; repo rules demand legal review before charging for filing services. x402 also adds KYT/AML surface if we hold or resell funds. Keep e-filing payment gates human (AGENTS.md).
7. **Reliability.** Cloudflare suffered notable outages in 2026 (two incidents hit crypto exchanges). Edge gating = availability dependency; ensure the Worker/gateway can be disabled to pass-through without breaking origin billing.
8. **Market timing.** Independent data says the agentic economy is real but tiny. Gate *spend* on phase milestones (below), not on roadmap enthusiasm.

---

## 5. Phased adoption plan

| Phase | When | Actions | Exit criteria |
|---|---|---|---|
| **0 · Claim** | Now (no code) | Reserve `cloudflare.pay` handle(s); join Monetization Gateway waitlist; pick **Base** as default chain; create a BC-software Base USDC test wallet | Handle reserved; waitlist confirmed |
| **1 · Spike** | Next 1–2 sprints | Deploy `x402-proxy` Worker in front of **one** AWS endpoint (or the local MCP server) on `base-sepolia` + Circle test faucet; verify via Coinbase CDP testnet; measure latency, cost, UX | A real agent pays a real 402 and gets data; latency/cost recorded |
| **2 · Buy-side** | When CF Wallets GA (target: "coming months") | Account Wallet + Virtual Wallet per agent with allowance/allow-list/max-tx; until then self-custody + internal ledger | Our agents pay 3+ third-party x402 APIs autonomously within caps |
| **3 · Sell-side GA** | When Monetization Gateway GA | Move x402 gating from Worker to Gateway rules (API/Terraform); require Web Bot Auth on paid tiers; launch dual-rail pricing (Stripe + x402); reconcile both in the billing ledger | First real USDC revenue; payment failure < threshold |
| **4 · Revenue & bridge** | Once off-ramp is reliable (H2 2026+) | Accept USDC as real revenue with fiat redemption; evaluate MPP+Stripe cards-at-the-edge; re-evaluate AWS-native option against CF pricing | Off-ramp exercised end-to-end once |

**Phase gates:** do not spend real money on USDC float before Phase 3; do not let x402 revenue share exceed, say, 20% of total before off-ramp is proven.

---

## 6. Owner decisions (top 3 — decide now)

| # | Decision | Options | Recommendation |
|---|---|---|---|
| **1** | **What role does Cloudflare play?** | (a) Edge gatekeeper + buyer wallet + payer identity (recommended) · (b) All-AWS-native x402 (CloudFront+WAF+Coinbase) · (c) Both, per asset | **(a)** — Cloudflare is already fronting; the Gateway/Worker pattern leaves the AWS origin untouched; AWS-native kept as a documented fallback |
| **2** | **Do we accept revenue in USDC at all?** | (a) Yes, as a capped parallel rail (recommended) · (b) Stripe-only until off-ramp maturity · (c) x402-first | **(a)** — ship the rail (Phase 1 spike), take money only after Phase 3, with Stripe primary and a USDC cap; get regulatory sign-off on monetizing verification data first |
| **3** | **Handle + payer-identity strategy** | (a) Reserve now, require Web Bot Auth + cloudflare.pay on paid tiers (recommended) · (b) Reserve now, identity optional · (c) Skip handles, rely on API keys | **(a)** — reserve `bcsoftware`/`complianceos` handles now (free, FCFS, but immutable — pick the name once), and use handle+Web Bot Auth as the org-attribution layer for paid verification-API tiers |

**Also decide (secondary):** Base vs Solana as default settlement chain (Base: 2s, EVM SDKs; Solana: 0.5s, SVM) — recommend **Base** for ecosystem/SDK maturity; the internal facilitator abstraction must support both.

---

## 7. Sources (verified 2026-08-19)

- Cloudflare docs — Wallets: `developers.cloudflare.com/wallets/` (+ `/wallets/faq/`), updated Aug 19, 2026
- Cloudflare docs — x402: `developers.cloudflare.com/agents/tools/payments/x402/` (+ charge-for-http-content, MPP pages, updated Jun–Aug 2026)
- Cloudflare blog — *Monetization Gateway* (Jul 1, 2026) and *Cloudflare Wallets* (Aug 4, 2026); press release Aug 4, 2026
- x402.org — protocol, foundation members, last-30-days stats; x402 Foundation / Linux Foundation operational-launch release (Apr 2, 2026)
- Coinbase CDP x402 docs — facilitator pricing/limits, network support (via secondary reporting: Wavect, Eco, Dropstab)
- Market data: Helios Analytics via Jamie Coutts (Aug 12, 2026; settlement down 93% YTD, ~$28.4K/day), CCN/Yahoo Finance/CryptoPotato coverage
- InfoQ — Cloudflare & AWS x402 at the edge (Jul 2026); Wavect — *Cloudflare Wallets: what is live* (Aug 5, 2026)

*Note: NET Dollar (Cloudflare's stablecoin product) and Pay Per Crawl are adjacent but distinct products — out of scope for this bridge except where noted.*
