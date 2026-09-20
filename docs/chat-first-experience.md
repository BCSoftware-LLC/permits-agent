# Chat-first product experience

Owner direction, September 20, 2026. This specifies the next product experience; signup, durable conversations and automatic workflow routing are not implemented yet. Read alongside the [harness/security architecture](agent-harness-security.md) and [commercial readiness](commercial-readiness.md). The owner clarified the intended layout: a familiar conversational application in the vein of Venice.ai, ChatGPT and Cursor, with tabs specific to permitting. This is an interface reference, not a request to use those vendors as the application's model provider.

## Entry experience

After signup/sign-in, open the application directly to a conversation composer. Do not require a business profile, case creation form, model selection or connector configuration before the first question. Keep the marketing site separate from the authenticated application.

Heading: **What can we help you get started?**

Show exactly these three suggested prompts, with normal capitalization:

1. What type of permits do I need to open a restaurant in Los Angeles?
2. I need a Type 21 license in San Diego County.
3. Who would I speak to in the City of Irvine to approve my CUP?

Selecting a suggestion fills the composer so the owner can edit and send it. Free text is equally supported. The conversation is the primary interface. A sidebar holds previous conversations and business projects. An optional details panel shows the current plan, sources, documents, unresolved questions and actions needing review. On mobile, expose those details through accessible tabs or a drawer.

Recommended navigation within the selected business/project:

| Tab | Purpose |
|---|---|
| Chat (default) | Ask questions, answer follow-ups and direct the agent |
| Workflows | Track permit research/preparation, dependencies, missing information and actions awaiting approval |
| Documents | Review source forms, supporting materials and prepared packages once secure document handling is available |
| Contacts | View verified agency offices/contact channels and review correspondence drafts |
| Sources | Inspect official evidence, applicability, retrieval dates and coverage gaps |

Keep account settings, billing/usage and external-agent connections in settings. These tabs are views of the same business records; a document prepared in chat appears in Documents and its associated workflow. The active business is always visible. Do not infer cross-case retrieval permission from sharing an organization, and never change the active business silently during a conversation. Unimplemented tabs must have honest empty/availability states until backed by working services.

## A question starts the appropriate work

The following are workflow specifications, not researched answers to the example questions.

| Opening question | Initial workflow | Progressive clarification | Result the owner sees |
|---|---|---|---|
| Restaurant in Los Angeles | Business-opening discovery; retrieve candidate agency/source records and assess applicability | Establish whether the owner means the city or another location; ask about premises and relevant activities when needed | A sourced initial answer and saved research plan with confirmed facts, unknowns, dependencies and next steps |
| Type 21 in San Diego County | Specific-license research using ABC reference/mirror tools and local-source discovery | Determine transaction intent and premises; distinguish county location from actual issuing land-use authority | Relevant source evidence, transaction questions, candidate documents and a preparation plan; no presumption of eligibility or automatic local approval |
| Irvine CUP contact | Official contact and approval-process discovery | Ask for proposed use or application context only when necessary to resolve the responsible office/process | Verified department/contact channel and source date, explanation of the applicable decision process if established, and an optional inquiry draft |

For the contact question, distinguish the office that receives inquiries from the person or body empowered to make a decision. Do not infer authority from a staff directory entry. If current official evidence is unavailable, state the limitation and offer the official verification route. A contact lookup can be useful without requiring an address or a full business case first.

Every question creates a persistent conversation/work item. Create or link a richer business case as useful facts emerge, rather than inventing required intake values. Ask short, relevant follow-up questions within the conversation. Preserve corrections as versioned case facts. The owner can inspect and edit the structured details at any time.

## Connected data and workflow behavior

Route each request through the same server-enforced permissions used by external agents. The model calls typed tools for agency discovery, official documents, license records, case facts and approved actions. It must not receive general database credentials or arbitrary SQL access.

Maintain distinct data domains:

- Shared official knowledge: issuer registry, jurisdiction boundaries, source documents, form versions, public records and contact channels, with provenance and freshness.
- Private customer work: conversations, premises facts, documents, notes, plans and approvals, filtered by customer and case grants before retrieval.
- Run history: tool requests/results, policy decisions, progress, costs and checkpoints under the privacy rules in the security design.

The agent can answer from approved evidence, ask a question, start research, prepare a document, or request review. The owner sees concrete progress such as “Checking the responsible planning authority,” only when that work actually occurs. Source failures, stale evidence and unknown requirements remain visible. An empty source result is not evidence that no permit is required.

The response should combine a plain-language answer with source links and a small number of relevant next actions. Research plans and documents appear alongside chat instead of being buried in prose. The same workflow must be available to an authorized external agent through run/status/result operations; those operations are proposed and are not present in the current connector.

## Current implementation gaps

The candidate currently uses operator-provisioned access keys, a form-first workspace, and a request-driven assistant requiring an existing case. The case schema requires a business type and a city or county. Conversations therefore need their own storage and lifecycle before they can support questions that lack those fields. Current source seeds include ABC/state/federal entry points and limited city planning pages; there is no complete statewide issuer/contact registry or Irvine-specific connector. San Diego city guidance must not be used as county-wide guidance.

Preserve existing case tools and the read-only assistant while adding customer sign-in, tenant-scoped conversations, durable runs, structured routing and progressively collected facts. Build the chat home against those services. Do not replace missing services with fabricated chat answers, fake progress or a signup button that does not create an account.

## Acceptance

- A new authenticated customer lands on chat and the three prompts without completing an intake form.
- Each prompt follows its appropriate workflow; unsupported coverage is clearly reported.
- A general question can start without a case; later case linking preserves the conversation and provenance.
- Refresh/reconnect restores authorized conversation and job state; switching accounts clears private client state and cannot reveal earlier results.
- Progress reflects recorded execution; a failed source or paused job never displays successful completion.
- Sources show when they were retrieved and distinguish verified evidence from owner assertions or model inference.
- Read-only research can proceed under the customer's granted scope. Sending, filing, signing, payment and booked appointments use the exact-action approval contract.
- Keyboard, screen-reader and mobile flows support suggestions, composer, progress, follow-up questions and document review.

This experience sets the product direction. It does not expand source coverage, establish regulatory requirements, select an identity provider or authorize production data transfers by itself.
