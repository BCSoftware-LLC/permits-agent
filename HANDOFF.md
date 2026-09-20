# Permits Agent — current public handoff

Updated September 20, 2026 UTC. Audience: maintainers and collaborating agents, including Muse.

## Product decision

Permits Agent is the reusable public product. Permits N More is its first customer and has separate website/operating-system projects. The owner wants all California, local CUP/zoning/building/health and business-startup workflows, state ABC/tax/entity work, federal TTB, document preparation, agent connections, scheduling/correspondence and eventual commercial hosting. Public repository visibility is intentional. Do not put private customer records or secrets here.

## Baseline and preserved work

Main at `615ebf02e3a1101a98b3ab62d8453b7f03ee3f44` merged the native Go connector. The prior local handoff predated that merge and identified unfinished history/compatibility/packaging fixes. This candidate incorporates those preserved fixes in a separate branch without resetting the old Python repository, spike or Go worktree. No existing live agent configuration or customer website was changed.

## This candidate

- Restores multi-area/multi-watch digest and legacy MCP reference shapes; adds provenance to mirror MCP results.
- Stages downloaded exports until successful validation/import; rejected downloads preserve the accepted CSV and mirror. History capture follows successful mirror installation.
- Adds a product website with a fictional example and scoped customer workspace.
- Adds a separate hosted server, JSON tool API, stateless remote MCP, tenant cases, editable intake, paginated case summaries, evidence notes, optimistic concurrency and audit records.
- Adds official source catalog/retrieval with freshness, final URL, content hash, bounded excerpts and explicit access failures. Current catalog is a seed, not a complete jurisdiction registry.
- Generates research checklists, Markdown preparation briefs, unsent inquiry drafts and owner-selected ICS reminders. **These are not filled official forms.**
- Adds an optional owner assistant using a configured Responses model and read-only research tools. Model calls require an explicit scope and monthly allowance; no live paid model requests have been made.
- Adds durable request/model usage accounting, a Dockerfile and an optional Cloudflare edge proxy. No deployed service or active billing is claimed.

## Verification and honest launch state

See [verification](docs/verification.md). Source retrieval worked against SOS, CalGold, CDTFA, Los Angeles planning and San Diego guidance. TTB returned HTTP 403 and is reported as unavailable, not fabricated success. Local visual browser verification is blocked by the browser tool’s unavailable administrator policy check. Code/protocol/HTTP and simulated provider tests are separate evidence.

**Not production complete.** This is a synthetic-data research/preparation pilot candidate. [Commercial readiness](docs/commercial-readiness.md) is the authoritative remaining-work list. [Hosting](docs/hosting.md) records deployment configuration and gates.

## Continue in this order

September 20 architecture research: read [agent harness and security decision](docs/agent-harness-security.md) before choosing a runtime. It compares Palantir, Codex App Server, OpenAI Agents API/SDK, Cloudflare Workflows, Temporal and Jev; maps current control gaps; and defines acceptance cases. The recommended direction keeps Go case/tool authority, adds durable orchestration, and evaluates a replaceable harness. This is a proposal, not an implemented integration or security certification. Customer PII controls must precede private document intake.

1. Review and merge the candidate only after exact-SHA CI and independent review. Do not silently cut over the existing Python customer integration.
2. Confirm the owner’s product domain, hosting accounts and spend limits. Deploy staging with encrypted durable state, keys, source refresh, alerts and a tested backup/restore/deletion process. Complete browser and real MCP-client acceptance.
3. Evaluate the optional owner assistant with an approved model, synthetic scenarios and hard budget. Resolve source-access gaps through supported official paths; do not bypass agency authentication or anti-bot controls.
4. Build verified jurisdiction/issuer discovery and actual versioned form templates/field mappings, beginning with representative pilot cases while accepting California-wide intake. Obtain operator acceptance from Permits N More. Unsupported paths stay explicit.
5. Add secure document storage, evidence/review states and exact-package authorization before execution adapters. Government submissions, signatures, fees, correspondence and appointments require specific customer authorization and receipt/reconciliation handling.
6. Implement customer identity/OAuth and billing entitlements/reconciliation around an approved paid offer. Cloudflare analytics and internal request counters are not invoices; x402 is optional, not a launch prerequisite.

Never call a research checklist “all required permits,” an AI answer “agency verified,” or a Markdown worksheet “ready to file.” Keep reusable product development separate from the first customer’s private assets.
