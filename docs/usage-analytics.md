# Client and agent usage analytics

The hosted service now records operator-registered client attribution and exposes `/analytics.html` plus `GET /api/analytics`. This is application telemetry, not advertising tracking, proof that a particular model was used, or a billing ledger. It does not report traffic from a deployment that has not installed this version.

## Register Muse separately

Generate a separate key for each integration with the existing `permits-agent-server keygen TENANT` command. Keep the returned secret private and deliver it through the customer's existing secure configuration process. Add the following **non-secret** metadata to the corresponding credential record in the operator's private keys file:

```json
"client": { "id": "muse", "actor_kind": "agent", "agent_family": "muse" }
```

For a separately issued website credential:

```json
"client": { "id": "website", "actor_kind": "human" }
```

Other registered agent families can use identifiers such as `chatgpt`, `cursor` or `custom`; these are operator labels, not built-in integrations. IDs must be opaque lowercase identifiers, up to 64 characters, without applicant names/emails. `actor_kind` is `human`, `agent`, `service` or `unknown`. Only `agent` accepts an `agent_family`. Existing credentials without metadata remain `unknown`. Same-tenant credentials with the same client ID must share attribution and existing tenant quotas remain shared across all keys.

The credential file is loaded at startup. Deploy/restart under the operational change process after editing it; no running service, credential or Muse configuration was changed by this implementation. Future OAuth integration should derive attribution from a registered client and authorized subject, not a caller-supplied label.

Neither `User-Agent`, `X-Agent-Family` nor MCP `clientInfo` overrides attribution. A dedicated Muse credential demonstrates use of that credential; it cannot prove the executable, underlying model, actual human intent or that the key was never shared. Do not classify requests based solely on whether they use MCP: a human-operated tool can use MCP, and an agent can use the JSON API.

## Access and dashboard

Grant `analytics:read` only to a customer/operator who may view the credential's organization. Grant `analytics:all` only to a product operator authorized to view aggregate whole-product usage. Neither scope is granted by keygen. Do not give whole-product analytics access to Muse or a customer research key. A separate analytics credential with no case/research scopes is supported.

- `GET /api/analytics?days=30` defaults to the current organization.
- `GET /api/analytics?days=30&scope=platform` requires `analytics:all`.
- `days` accepts 1–90, representing a rolling UTC window; default 30.
- There is no caller-selected tenant parameter. Unrecognized or repeated parameters are rejected.
- The dashboard at `/analytics.html` shows totals, registered clients/channels, tool usage, daily requests and detailed daily aggregates. It requires an authorized key, held only in tab memory. Changing identity/scope clears old results and invalidates delayed responses.

The public page is only a shell. Its data comes from the authenticated endpoint. Whole-product metrics group matching client/type/family identifiers across organizations; raw tenant IDs, case IDs and customer content are not returned.

## Metric definitions

| Metric | Meaning |
|---|---|
| Requests | Admitted authenticated HTTP requests, including MCP initialization/listing and usage calls; analytics views excluded to avoid inflating the dashboard |
| Request failures/incomplete | Those requests with error outcomes or without a recorded successful completion |
| Tool attempts | Calls reaching a registered hosted tool handler, including permission/validation failures; protocol initialization is not a tool call |
| Tool executor | `client` for direct API/MCP calls, `owner_assistant` for research calls made within our assistant; originating registered client is preserved |
| Model calls/tokens | Reserved owner-assistant model requests and returned token usage, linked to their originating request/client |
| Model calls without usage | Reserved calls lacking a provider response record; not automatically free or unsuccessful billing |
| Tool elapsed time | Total recorded handler milliseconds, shown as a mean in grouped tool views; incomplete calls can understate elapsed time |
| Daily detail | UTC day, registered client/type/family, entry path, event kind, operation/tool/model, executor and counts |

Requests, tools and model calls are separate populations. Never sum them as “users” or bill all three as independent customer actions. They do not prove a workflow, filing, email or appointment completed. The current code has no such execution adapters.

Limits: only this hosted server's admitted authenticated traffic is included. Anonymous website visits, failed authentication, rate/quota-denied admission, GitHub views/clones and local CLI/stdio MCP runs are not tracked. An external agent's own model calls, tokens and spend are also outside our visibility. Old request/model rows remain visible as unattributed; past per-tool activity cannot be reconstructed. Results contain up to 2,000 grouped rows with an explicit truncation flag; totals cover the full selected window. Shorten the window when details truncate.

Muse must call an instrumented hosted endpoint for central product usage to appear. If Muse runs the connector locally, central analytics requires an explicitly disclosed telemetry design or a switch to the hosted service; this change introduces no hidden local reporting. Until the actual connection and deployment are verified, **Muse traffic is unknown, not zero**.

## Storage and privacy

Additive SQLite tables hold request attribution, tool event metadata and request-to-model links. Existing data is preserved; no guessed backfill is applied. Request admission and attribution are committed together. Tool attempts are recorded before execution; unavailable recording prevents the tool from starting. If completion recording fails after an effect, the caller receives an uncertain-outcome message and must check case state before retrying.

No request/response content, arguments, prompts, IP addresses, URLs supplied in arguments, contact details or raw tokens are added to analytics. Tool names come from the registered tool set. An unknown tool name is not persisted as arbitrary telemetry text. Provider names/models are operator configuration. Identity metadata is still confidential operational data and belongs under the deployment's encryption, access, retention and deletion policies. Automated telemetry retention/purge is not implemented here.

## Validation and next work

Tests cover tenant/platform boundaries, spoofed headers/MCP identity, protocol-versus-tool counts, legacy unknown attribution, failure metadata without payloads, fail-closed tool recording, caller-versus-assistant model attribution and browser-state clearing using a simulated DOM. Production visual/client acceptance and actual Muse traffic verification remain separate deployment checks.

Next product metrics need durable conversation/workflow IDs and reviewed, privacy-safe outcome categories: workflow starts, supported jurisdiction/procedure, source access failures, form preparation, owner review, completion/abandonment and turnaround. Do not infer topic categories by storing raw conversations in analytics. Add processor/currency cost reconciliation, genuine customer identity, anonymous traffic policy if desired, and connector version telemetry with an explicit distinction between verified identity and self-reported attributes.
