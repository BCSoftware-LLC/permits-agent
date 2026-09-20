# Verification record — preparation platform candidate

Date: September 20, 2026 UTC. Native baseline: `615ebf02e3a1101a98b3ab62d8453b7f03ee3f44` (merged PR #1). Candidate source and exact CI SHA are identified by the associated pull request and its checks; release packaging records source commit/tree and refuses dirty source trees.

## Local verification

- `make check build audit`: formatting, Go vet, race-enabled package tests, module checksum verification, JavaScript syntax, website state/proxy unit tests, all three native binaries, vulnerability scan. Passed on the candidate during implementation; final exact-SHA checks must also pass.
- Native suites verify CLI/protocol invocation, filters, dates, multi-record identity, reference parsing, snapshot/history behavior and compatibility. Additional regressions cover invalid downloaded CSV preservation with a subsequent non-force retry, legacy multi-area digest, legacy scalar type descriptions, and freshness on populated/empty MCP searches.
- Hosted HTTP/MCP tests exercise initialization, discovery, tool calls, wrong argument types, page limits, missing/bad credentials, origin rejection, body bounds, tenant isolation on reads/exports/updates, write scopes, idempotent creation, case revisions, intake changes, list pagination, schema discovery, artifact labeling and durable quota enforcement.
- Simulated provider tests exercise bounded read-only tool continuation, model-call reservations/token accounting, denied mutations, cross-tenant rejection, null/invalid arguments, failure reservations, provider endpoint restrictions and 40 KB source evidence without transport duplication. **No live model evaluation or provider spend.**
- Source retrieval tests cover malformed/oversized HTML, ignored scripts/navigation, safe official links, bounded excerpts, offline/stale cache, arbitrary source rejection, redirect policy and relative links against the final approved URL.
- Node unit tests execute the actual website script in a simulated DOM. Delayed previous-tenant responses cannot repaint the current workspace after disconnect; rendered content and the assistant question are cleared. This is not full browser, layout or accessibility verification.
- Worker unit tests cover fixed origin routing, stripping ambient cookies/Access headers, preserving caller bearer identity, service credentials, unsafe configuration and redirect rejection. **No live Worker deployment or Docker image build.**

## Live public-source observations

Public source retrieval was exercised through the new HTTP tool endpoint, using a synthetic tenant and isolated local database. No customer/applicant records, signatures, messages, payments or government changes were involved.

| Source | Result |
|---|---|
| California Secretary of State startup guidance | Readable official HTML and document links |
| CalGold | Readable landing guidance, including incomplete-information warning; no claim to extract its complete interactive registry |
| CDTFA permits/licenses | Readable official guidance and links |
| Los Angeles Restaurant Beverage Program | Readable official guidance and links |
| San Diego alcohol zoning guidance | Readable official guidance and links |
| TTB Permits Online | HTTP 403, explicitly reported as unavailable |

Earlier baseline audit loaded 129,301 ABC rows from the September 19, 2026 official export in an isolated audit cache and exercised official types/forms/fees/news/requirements. Those observations substantiate the baseline, not automatic verification of every new production deployment. Native source freshness remains visible.

Local raw evidence is ignored under `dogfood-output/`, including `platform-checks.log`, `source-retrieval-smoke.json` and the synthetic local preview state. Do not commit caches, raw credentials or customer data. Public release evidence should contain aggregate results only.

## Independent reviews

Independent security and specification reviewers examined the changing candidate. Findings corrected include conflicting startup routes, map-argument validation, remote history mutation, late previous-tenant responses, protocol-error accounting, oversized case lists, hidden customer content, null model arguments, incomplete tool schemas, immutable case intake, missing pagination/hosted packaging, redirected relative links and duplicate evidence in model context. Pin final review to the committed candidate before a release claim.

## Outstanding acceptance gates

The local browser tool refused access because it could not verify its administrator-enforced policy. No alternative browser-control route was used to bypass the block. Visual/mobile/accessibility validation remains incomplete. Docker is unavailable locally; actual container build, Cloudflare deployment, TLS/origin isolation, persistent disk, backups/restores, deletion/retention, customer terms, live connector clients and model acceptance remain required. Billing, full jurisdiction coverage, official-form filling and authorized filing workflows are not implemented; see [commercial readiness](commercial-readiness.md).
