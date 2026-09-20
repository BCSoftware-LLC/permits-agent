# Commercial readiness and remaining work

Owner decision: launch **Permits Agent** as the reusable product; **Permits N More** is its first customer. Public source is intentional for collaborating agents. Statewide scope means California-wide intake and jurisdiction routing; it must not be marketed as complete verified filing automation before that work is actually validated.

## What exists in this candidate

| Capability | Actual behavior |
|---|---|
| Native ABC CLI/MCP | Existing daily mirror and official reference tools, with compatibility, freshness and cache-integrity fixes |
| Website | Product pages, fictional demo, authenticated customer workspace, agent guide, pilot data notice |
| Hosted connector | Stateless Streamable HTTP MCP and JSON tool API; 21 ABC tools plus ten preparation tools; shared domain behavior |
| Customer cases | Credential-derived tenant boundary, idempotent creation, paginated list summaries, version-checked intake updates and notes and audit rows |
| Discovery | Curated official HTML retrieval with cache/freshness, final URL and content fingerprint; California research checklist; exact local jurisdiction still needs verification |
| Preparation | Markdown review worksheet, owner questions, checklist, source links, unsent agency inquiry and ICS review reminder |
| Owner assistant | Optional Responses tool loop over selected case and bounded ABC and official-source read tools; no execution authority |
| Usage controls | Durable HTTP request reservations, tenant quotas, rolling rate limits, model-call reservations and token accounting |
| Hosting artifacts | Native server, embedded assets, Dockerfile and optional Cloudflare edge proxy; not deployed |

## What is still required for the user's full product

These are real implementation/operational gaps, not completed features hidden behind credentials.

1. **Verified jurisdiction and agency discovery.** Resolve address/parcel and city vs unincorporated county jurisdiction. Maintain a versioned issuer registry covering California local governments and special districts. Agency pages must be fetched with provenance, timestamps and freshness policies; prevent arbitrary URL access and prompt injection. Seed links are not complete coverage. CalGold itself warns of incomplete information.
2. **Requirements and workflow verification.** Rules need activity, transaction, premises, local authority, effective date and primary-source evidence. Confirm ABC/CUP/local plan dependencies, TTB registration vs permit paths, health, building/fire, employment/tax and industry-specific branches. Track unsupported paths explicitly. Validate representative cases with Permits N More before promising coverage.
3. **Actual official form completion.** Acquire and version exact agency forms/instructions; maintain reviewed field mappings; gather supporting materials; validate required fields and cross-form consistency; fill the actual official formats; render and review output. Current Markdown briefs are not official filing packages. Secure document upload, encrypted object storage, OCR/redaction and retention are prerequisites for personal applicant material.
4. **Execution with authority.** Represent the owner and representative's actual signing/filing authority separately from connector access. Bind approval to a content hash, destination, attachments and expiry. Implement submission, calendar/email operations only through supported agency/provider paths, with idempotency, receipts, uncertain-result reconciliation and human handoff for MFA/CAPTCHA/unsupported portals. No final action can be authorized by model text alone.
5. **Customer identity and access.** OAuth 2.1 for remote clients that require it; customer sign-in, organizations, staff roles, key management, support access and revocation. Pilot keys are operator-managed, not a finished public SaaS onboarding experience.
6. **Production operations and privacy.** Real host/account/domain, secrets, encrypted durable storage, tested backup/restore/deletion, alerts, incident/rollback runbooks and operator-approved privacy/service terms. Complete visual/accessibility and deployed security testing. Existing Python runtime remains untouched.
7. **Billing and reconciliation.** Decide a paid pilot offer, currency, included usage, overage consent and caps. Implement Stripe checkout/customer portal or approved invoicing; verify signed webhooks with event deduplication; derive entitlements server-side; export reconciled, idempotent meter events. Refunds, tax handling, cancellations, failures, customer-facing usage and support processes need acceptance tests. Current counters do not charge anyone.
8. **Agent quality.** Evaluate answer grounding, unsupported jurisdictions, ambiguity, outdated sources, malicious notes, account boundary, refusals to invent/submits and failure recovery using an approved model. No live model evaluation has been performed. Add cross-provider support only against tested contracts.

## Recommended commercial sequence

Start with an assisted, paid preparation pilot sold to permit professionals: one organization subscription with included cases and a capped research/assistant allowance. Treat this as a proposed offer, not published pricing. Measure actual operator time and model/data costs with Permits N More, then price using observed value and margin. Keep filing fees and professional services distinct from software usage. Invoice only after the customer approves the exact service scope and price.

For agent builders, add a metered API tier after identity, entitlements, reconciliation and support are working. Count business-valued operations deliberately rather than billing for incidental MCP initialization/listing traffic. Current HTTP quotas count every authenticated request, including failed requests and protocol/usage calls; they are protective allowances, **not the billable event definition**.

Cloudflare is a reasonable edge and model-observability choice. It does not eliminate customer identity, billing or durable application state. Use Stripe for ordinary subscriptions/invoicing once configured; treat x402/Cloudflare Monetization Gateway as an optional later channel, subject to confirmed availability and a viable customer need. The old local AWS/Stripe/x402 proposals were design notes, not deployed systems.

## Release acceptance

A code build passing tests is not a blanket production certification. The proposed first release is a **synthetic-data preparation pilot candidate** until the hosting/privacy/browser/live-client gates in [hosting](hosting.md) pass. The full owner-requested filing product additionally requires the verified-source, document and execution work above. Never change the website to say “all permits handled” or “ready to file” based only on these checklists.
