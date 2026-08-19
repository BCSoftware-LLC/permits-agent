# Reddit + X Launch Playbook — ComplianceOS (permits-agent)

**Product:** ComplianceOS (agent-native platform) / Permits N More (consumer front) — CA ABC
liquor-license compliance first, expanding to zoning/permitting/licensing.
**Status:** Launch phase · **Owner:** BC Software · **Updated:** 2026-08-19
**Source of truth for all claims:** `docs/data-sources.md` (verified 2026-08-19), `README.md`,
`docs/product-brief.md`, `AGENTS.md`. If a claim isn't in those files, it doesn't go in a post.

---

## 0. Rules of the road (read first)

1. **Public posts use the Permits N More / ComplianceOS brand.** BC Software stays the internal
   platform identity; never publish the "internal project" framing from the README.
2. **Only claim what Layer 1 shipped and verified** (daily mirror, address→licenses, overdue /
   pending radar, MCP tools, CLI, daily digest). **Never claim** document assembly, guided
   e-filing, status-change history, or the verification API — those are roadmap (`⏳`/`🔒` in the
   product brief) and "guaranteed approval" is forbidden, period.
3. **We are software + information + document prep assistance, not legal services, not lawyers,
   and not an ABC filing agent (yet).** Third-party paid filing sits in regulated territory; no
   public post may describe us as offering it. Every post keeps the applicant-signs,
   human-approval-at-submit reality visible.
4. **Disclose founder affiliation on Reddit** wherever the post is even adjacent to promotion
   (sitewide self-promo guideline + most sub rules require it). On X, founder voice is the point.
5. **Numbers always carry their snapshot date** (e.g. "as of the 2026-08-19 daily export"). Data
   is from the state's official daily export, mirrored once a day. If the export is stale, we
   say so — we never serve or quote stale data as fresh.
6. **Not affiliated with or endorsed by the California Department of Alcoholic Beverage Control
   (CA ABC).** Say it where a reader could infer otherwise.

---

## 1. Positioning

### One-liner

> **ComplianceOS turns the state's own daily public records into a living compliance map —
> every license and pending application in California, refreshed daily, queryable by address,
> zip, or county.**

### Three value props (each tied to a verified capability)

| # | Value prop | Verified capability | Evidence in repo |
|---|---|---|---|
| 1 | **The daily mirror.** One snapshot of every CA ABC license *and* pending application (~128,870 records), rebuilt daily from the state's official export — no scraping, no stale third-party listings. | Daily mirror (Layer 1, shipped) | `docs/data-sources.md` §Primary; `README.md` Quick start |
| 2 | **Address → licenses.** Type one address, get every license on that premises — including multi-permit sites (e.g. a hotel holding types 47/58/66/68/77 together) and same-owner portfolios. | `abc-agent address` (verified) | `docs/data-sources.md` §Address identification |
| 3 | **Overdue + pending radar.** Spots the 2,553 CA licenses still **ACTIVE past their expiration date** (renewal-failure / auto-revocation candidates) and ~6,600 pending applications with new filings surfacing by zip/district. | `abc-agent overdue`, `abc-agent pending` (verified) | `docs/data-sources.md` §Status vocabulary |

Supporting facts (all verified, safe to cite in comments): 86 license types · 87 indexed forms ·
status vocabulary ACTIVE/PEND/SUSPEN/REVPEN/SUREND/R64B/REV/… · file numbers are **not** unique
(one file = one row per license type) · 20,030 application rows vs 108,840 license rows in the
current snapshot · the state publishes a free daily CSV anyone can download.

### Claims ledger (post every claim through this)

| Claim | Safe to make? | Source / note |
|---|---|---|
| "128,870-record daily mirror of every CA license + pending app" | ✅ | Verified 2026-08-19 snapshot |
| "2,553 licenses ACTIVE past expiration" | ✅ (as of date) | Derived OVERDUE, 2026-08-19 snapshot — describe as *candidates*, never as "being revoked" |
| "6,620 pending applications" | ✅ (as of date) | PEND status count, 2026-08-19 |
| "One address can hold 5+ licenses (hotel 47/58/66/68/77)" | ✅ | Multi-permit sites documented in data-sources |
| "We fill out and submit your application" | ❌ | Layer 3 is gated; filing services need legal review — do not describe publicly |
| "We'll get you approved / 95% success / fast-track" | ❌❌ | Forbidden. No outcomes, no success rates, no guarantees |
| "License X is about to be revoked" (naming a specific business) | ❌ | We show data; we don't predict outcomes, and we don't call out individual licensees |
| "Ask us anything — we're the legal experts" | ❌ | We are not lawyers; add "not legal advice" wherever close |
| "Verification API for lenders/insurers" | ⚠️ Vision only | Roadmap item 6 — say "what we're building toward," never "available now" |

---

## 2. Target subreddits & engagement-first plan

General: **week 1 = participation only, zero links, zero product mentions.** Follow Reddit's
sitewide guideline: ≥90% of your contributions on each sub are non-promotional. Every promotional
post discloses affiliation up front. Rules below are as commonly enforced — **re-read each sub's
sidebar/wiki the day you post**; when in doubt, message the mods first.

| Subreddit | Audience | Self-promo reality | Our play |
|---|---|---|---|
| **r/BarOwners** | Actual CA/US bar owners, operators | Small, tightly moderated industry sub; solicitation posts get removed fast; regular contributors get latitude | **Home court.** Weeks 1–2: answer license/ABC questions with real data. Week 2: value post #1 (expiration PSA). Week 3: mod-approved AMA ("ask me anything about ABC license status — I'll look it up in public records, no sales"). No links in week 1; state links only after |
| **r/restaurateur** | Restaurant owners, managers (industry talk) | Industry-insider vibe; promo removed; questions + war stories thrive | Value post #2 (ABC status-code field guide, buying/selling angle). Comment on existing "getting a liquor license" threads with factual walkthroughs |
| **r/smallbusiness** | General SMB owners | Promo allowed **only** in the designated self-promo Saturday thread; link-drops removed elsewhere | Compliant self-promo thread entry (short, value-first, disclosure). Regular participation on compliance/license threads |
| **r/Entrepreneur** | Founders, builders | Promo restricted to weekly threads; heavy spam filter | Optional: "what I learned shipping a compliance engine from public data" — storytelling post, product as case study, disclosure |
| **r/AskALawyer** | Legal questions; verified lawyers answer | Questions only; **no solicitation, no firm advertising** | Engagement-only. Answer ABC-license questions with *public-record pointers* + "not legal advice." Never pitch, never link the product. This builds credibility that converts later |
| r/California, r/LosAngeles *(optional)* | Local residents, business owners | No promotion; removal risk high | Only for genuinely interesting local-data posts (e.g. "what's licensed on your block") in week 4 — if mods bounce it, accept and move on |

### Participation cadence (per active day)

- Morning (PST): 15 min — check each sub for ABC/license/permit questions; answer 1–2 with
  verified facts + snapshot date.
- Noon: engage with replies on our own posts (24h response SLA, faster is better).
- Evening: 10 min — comment on 2–3 non-product threads genuinely (builds the 90/10 ratio).

### Content bank for comments (no link needed, all verified)

- "CA ABC publishes a free daily CSV export of every license + pending application (~128k rows) —
  anyone can check their own file number against it."
- "File numbers aren't unique — one file can carry multiple license types, so don't panic if your
  number appears more than once."
- "The status you care about when buying is PEND vs ACTIVE, and whether the current license is
  past its expiration date — 'overdue' licenses are the risky ones."
- "Not legal advice — the ABC's license-type pages describe which licenses a venue needs."

---

## 3. Draft posts (5)

Placeholders to fill before posting: `[WAITLIST]`, `[@handle]`, `[screenshot]`, `[mod approval]`.
All numbers cited are from the **2026-08-19** snapshot — update the date if posted later.

---

### Post 1 — Reddit text post (r/BarOwners) — *utility PSA*

**Title:** PSA: check your liquor license's expiration date — 2,500+ California licenses are still
"ACTIVE" past theirs (free way to check yours inside)

**Body:**

> Running a bar means your license is the whole business, and one thing surprised me when I
> started digging through public records: **the state's own data shows 2,553 California licenses
> still marked ACTIVE *after* their expiration date** (as of the 2026-08-19 daily export).
>
> That's the "overdue" bucket — a license past expiration that hasn't renewed, which is exactly
> the situation that leads to renewal-failure trouble and, eventually, administrative action. I'm
> not saying any of those 2,553 are getting revoked tomorrow — I'm saying the data is public and
> almost nobody checks it.
>
> How to check yours for free, right now:
> 1. CA ABC publishes a daily CSV of every license + pending application — no login, no paywall.
> 2. Search your file number or premises address. If your status shows ACTIVE with an expiration
>    date in the past, that's a conversation to have with the ABC well before renewal season.
> 3. One gotcha: file numbers are *not* unique — a single file can have multiple license types,
>    so your number appearing more than once is normal.
>
> Full disclosure: I run a small software shop building a compliance tool around this public data
> (daily mirror, address lookups, overdue/pending radar) — it's called ComplianceOS, waitlist is
> open, and I'm happy to check anyone's own license status from the public record for free if you
> comment your file number. No legal advice here, and I'm not affiliated with the ABC — just a
> nerd who reads the export so you don't have to.
>
> What's everyone's experience with renewal timing — do you renew 60/90 days out, or are you
> running it closer?

**Notes:** No product link in body (rule-proof); one optional comment-level `[WAITLIST]` link only
after traction. Disclosure is up top of the last paragraph. Ask a real question at the end (subs
love discussion posts).

---

### Post 2 — Reddit text post (r/restaurateur) — *field guide*

**Title:** Buying or selling a bar/restaurant with a liquor license? A plain-English guide to the
ABC status codes in public records

**Body:**

> If you're on either side of a restaurant deal that involves a liquor license, the ABC's public
> records are your due-diligence starting point — but the status codes read like alphabet soup and
> a few of them are easy to misread. Field guide from the state's own daily export (snapshot
> 2026-08-19), based on what the records actually contain:
>
> **The codes you'll actually see:**
> - **ACTIVE** — license is in force. ~119k of ~129k rows in the current snapshot.
> - **PEND** — application in progress, not yet a license. ~6,600 pending applications statewide
>   right now. Buying a business with a PEND? You're buying a question mark, price accordingly.
> - **SUSPEN** — suspended (often unpaid penalties). 357 rows.
> - **REVPEN** — revocation pending (commonly non-payment). 410 rows. This is the "auto-revocation
>   path" — deal with it before it resolves itself.
> - **SUREND** — surrendered. 2,376 rows.
> - **OVERDUE** (derived, not a state code) — still ACTIVE *after* the expiration date: 2,553
>   statewide. Renewal-failure candidates; the top thing I'd check in due diligence.
>
> **Three traps I see people hit:**
> 1. **File numbers aren't unique** — one file can carry several license types (one row each). A
>    "duplicate" isn't always a red flag.
> 2. **License rows vs application rows** — the export mixes ~109k licenses with ~20k
>    applications. Check *both* at the address you're buying.
> 3. **One address, many licenses** — hotels and mixed-use venues can hold five or more license
>    types (e.g. 47/58/66/68/77) under one roof. Make sure the deal covers all of them.
>
> Disclosure: I build software around this public data (daily mirror + address/status lookups —
> ComplianceOS, waitlist open, link in comments; happy to run a free status check on an address
> you're serious about). Not legal advice, not affiliated with the ABC.
>
> Anyone here have a horror story from buying a place where the license status turned out to be
> worse than the broker said?

**Notes:** Pure utility; the disclosure + waitlist live in the comments so the post itself reads
as reference material. Works for r/restaurateur; adapt the title for r/Entrepreneur ("what I
learned analyzing every liquor license in California").

---

### Post 3 — X thread (founder voice) — *the 128k-row CSV*

**T1:** Every day, the state of California publishes a ~27 MB CSV containing *every liquor license
and every pending application in the state*. ~128,870 rows. No login. No paywall. Nobody reads it.
We built a daily mirror of it. Here's why that matters:

**T2:** Your bar's liquor license is public record — status, expiration, address, all of it. But
"public" and "usable" are different things. A 27 MB CSV isn't a tool; it's a chore. So we turned
it into a daily-synced local database: one refresh a day, everything queryable.

**T3:** [screenshot: CLI output of `abc-agent stats` + `abc-agent search`]

**T4:** Examples of questions it answers in seconds:
- "Every license at this address?" (yes — including hotels holding 5+ license types)
- "What's expiring in my county in the next 60 days?"
- "Any new filings in this zip code?" (our lead radar for what's opening nearby)

**T5:** The number that stopped me: **2,553 California licenses are still ACTIVE past their
expiration date** (2026-08-19 snapshot). That's the overdue bucket — renewal-failure candidates.
Not predictions, just the state's own data, surfaced.

**T6:** It's also an MCP server, so any AI assistant can check a license mid-conversation —
agent-native, because that's where compliance questions get asked in 2026.

**T7:** We're ComplianceOS — software + info + document prep around public records, not lawyers,
not an ABC filing service, not affiliated with the ABC. Early access is open: `[WAITLIST]`
Building in public @ [@handle]. What would *you* query first? 👇

**Notes:** No legal claims, no outcomes promised. "Not predictions, just data" is the spine of the
thread — keep it.

---

### Post 4 — X thread (founder voice) — *the overdue radar*

**T1:** **2,553 California liquor licenses are marked ACTIVE with an expiration date in the
past.** That's not 2,553 doomed businesses — it's 2,553 unanswered renewal questions. Here's the
radar we built to see them: 🧵

**T2:** The state's export gives every license a status (ACTIVE, PEND, SUSPEN, REVPEN…) and an
expiration date. When those two disagree — still ACTIVE, already expired — that's the overdue
bucket: renewal-failure candidates on the path to administrative trouble.

**T3:** [screenshot: `abc-agent overdue --county "LOS ANGELES"` output]

**T4:** Why we built it: license lapse doesn't happen on a calendar you control. It happens in
week 46 of a busy year, and by the time you notice, you're already in the "explain it to the
agency" conversation. The data was always public — nobody was watching it daily.

**T5:** So ComplianceOS watches daily: a morning digest of your portfolio's expirations,
overdues, and new pending filings, from the official daily export. The product in miniature is a
cron job that runs before coffee.

**T6:** Rules of the road: we surface data, we don't predict outcomes, we don't promise approvals.
Not legal advice, not affiliated with the ABC. This is software + information around records the
state publishes anyway.

**T7:** If you hold, buy, or finance licensed premises, this is your radar too. Waitlist:
`[WAITLIST]` — and @ [@handle] if you want your county's number. What's the closest call *you've*
had with a license deadline?

**Notes:** Strong hook, zero hype. The "closest call" question invites replies = engagement.

---

### Post 5 — X post (short, screenshot-led)

> One address. Five licenses. A hotel that sells beer, wine, and spirits under one roof — every
> license type on the premises in one lookup, from the state's own daily export.
> [screenshot: `abc-agent address` output showing 47/58/66/68/77 at one premises]
>
> That's ComplianceOS's address → licenses view. When you're buying, financing, or underwriting a
> licensed property, "what's licensed here" shouldn't take an afternoon of PDF archaeology.
> Waitlist: `[WAITLIST]` · not legal advice, not affiliated with the ABC. [@handle]

---

## 4. 30-day cadence calendar

**Week 1 — Foundation: participation only, zero links, zero product mentions on Reddit.**
| Day | Channel | Action |
|---|---|---|
| D1 | X + Reddit | X: Post 3 (thread) live. Reddit: set up profiles; 2–3 genuine comments in r/BarOwners threads |
| D2 | Reddit | Answer 2–3 ABC/license questions in r/BarOwners + r/restaurateur with verified facts |
| D3 | X | Post 5 (address screenshot). Reply to every reply within 24h |
| D4 | Reddit | r/AskALawyer: answer 1–2 licensing questions with public-record pointers, "not legal advice," no pitch |
| D5 | X | Quote-repost + commentary on CA ABC news/advisories (from the news feed) |
| D6 | Reddit | r/smallbusiness: participate in compliance/permits discussion threads (no promo) |
| D7 | Both | **Weekly review**: log metrics, prune what's dead, note recurring questions → answer bank |

**Week 2 — First value posts.**
| Day | Channel | Action |
|---|---|---|
| D8 | Reddit | **Post 1 live** in r/BarOwners (disclosure included; waitlist only in comments after traction) |
| D9 | Both | Reply to every comment on Post 1 (24h SLA); X: link-thread Post 1's takeaway |
| D10 | X | **Post 4** (overdue radar thread) |
| D11 | Reddit | Prep Post 2; keep answering daily questions |
| D12 | Reddit | **Post 2 live** in r/restaurateur (waitlist link in first comment) |
| D13 | X | Weekly recap thread: "this week's weirdest data find" (engagement bait, no sales) |
| D14 | Both | **Weekly review**: which posts/conversations drove signups; adjust |

**Week 3 — Deep dive + AMA.**
| Day | Channel | Action |
|---|---|---|
| D15 | Reddit | DM r/BarOwners mods: request AMA approval ("I'll answer ABC-license questions from public records — no sales") |
| D16 | X | MCP thread: "we made it an MCP server so any AI assistant can verify a license mid-chat" [screenshot of MCP tools] |
| D17 | Reddit | r/smallbusiness Self-Promo Saturday: compliant entry (3–5 lines, value-first, disclosure, waitlist link) |
| D18 | Reddit | **AMA in r/BarOwners** (if approved): 60–90 min live window, answer with data, close with "waitlist in comments" |
| D19 | X | AMA highlights thread ("5 questions bar owners asked about licenses this week") |
| D20 | Both | Cleanup: answer stragglers everywhere; convert recurring questions into content bank entries |
| D21 | Both | **Weekly review** |

**Week 4 — Convert + widen.**
| Day | Channel | Action |
|---|---|---|
| D22 | X | Founder recap: "30 days of watching 128k licenses" — lessons, numbers, waitlist push |
| D23 | Reddit | Optional local-data post (r/California or r/LosAngeles): "what's licensed on your block" — accept removal risk |
| D24 | X | Daily-digest demo post: [screenshot of morning digest email/cron output] — "the product in miniature" |
| D25 | Both | Outreach: DM the 5–10 most engaged commenters (where sub rules allow) — offer free status check of their premises |
| D26–28 | Both | Follow-up: answer all threads, thank commenters, keep participation quota |
| D29 | Both | **30-day retrospective**: score channels, posts, and signups; write learnings to this doc |
| D30 | Both | Decision: double down on best channel; schedule next sprint (Layer 2 teasers only when real) |

Posting windows: Reddit 7–9am / 5–7pm PST; X 8–10am PST. Max 1 promotional post per sub per week.

---

## 5. Compliance guardrails checklist (run before EVERY post)

- [ ] Contains no claim of legal authority, guaranteed outcomes, success rates, or approval speed.
- [ ] Describes the product as software + information + document prep — **not** legal services,
      not a filing agent, not an attorney.
- [ ] Doesn't say or imply we submit applications or sign sworn documents on anyone's behalf
      (filing is customer's own account, human approval at submit/payment, applicant signs).
- [ ] Every number cites its snapshot date ("as of the 2026-08-19 daily export").
- [ ] No named individual licensees called out as "about to be revoked" — we surface data, not
      predictions.
- [ ] Reddit: affiliation disclosed in the post or top comment; no link drops in week 1; ≥90% of
      our contributions on that sub are non-promotional; sub rules re-checked that day.
- [ ] Reddit: waitlist link in a comment, not the post body, unless the sub allows it.
- [ ] AMA/self-promo-thread posts have prior mod approval where required.
- [ ] X: screenshots show real CLI/MCP output (snapshot date visible or cited); no bought
      engagement; no sockpuppets.
- [ ] "Not legal advice" included wherever a reader could mistake us for counsel.
- [ ] "Not affiliated with or endorsed by CA ABC" included wherever an official endorsement could
      be inferred.
- [ ] Roadmap items (verification API, document assembly) only described as "what we're building
      toward," never as available.
- [ ] Landing page / waitlist copy passes the same test (no claims stronger than Layer 1).

**Never-do list:** fake reviews · astroturfed accounts · DM spamming strangers · posting in
r/BarOwners without having contributed first · copying another sub's post verbatim (each post is
adapted per sub) · arguing with mods — appeal once, move on.

---

## 6. Success metrics & tracking (no UTM needed)

### What to track (simple spreadsheet: one row per post/thread)

| Column | Definition |
|---|---|
| Date · Channel · Sub/thread · Post ID | Identity |
| Impressions / reach | X: views; Reddit: post views if available |
| Engagement | Reddit: upvotes + comments; X: likes + replies + reposts + bookmarks |
| Clicks to waitlist | Link clicks (comment-link on Reddit, profile/comment link on X) |
| Signups | Waitlist additions attributed to this post |
| Conversations | Meaningful DMs/comments → prospect chats (esp. bar owners) |
| Notes | What worked, what flopped, recurring questions |

### Attribution (simple, no UTM/bitly)

1. Waitlist form asks **"How did you hear about us?"** — dropdown: Reddit (which sub) / X (which
   thread) / word of mouth / search / other.
2. Each X post uses its own short comment-link or profile-link convention; each Reddit comment
   link is unique to the post (`[WAITLIST — r/BarOwners PSA]`).
3. Weekly 15-min review: sum by source, rank posts, feed learnings into the answer bank.

### 30-day targets

| Metric | Target |
|---|---|
| Waitlist signups | 50–100 |
| r/BarOwners PSA post | ≥50 upvotes, ≥30 comments |
| AMA (if approved) | ≥40 comments, 3+ "can you check my file number" offers |
| Prospect conversations | 5+ bar/restaurant owners in DMs/comments |
| X thread impressions | ≥10k combined across threads |
| B2B conversations (lenders/insurers/franchisors) | 2+ — from the API-as-vision angle only |

### Weekly review ritual (D7/D14/D21/D29, 20 min)

1. Log the week's numbers into the spreadsheet.
2. Identify the single best-performing post/conversation → double down next week.
3. Harvest unanswered questions into the content bank.
4. Check the 90/10 self-promo ratio per sub; add participation if it slipped.
5. Update this doc with learnings (it's a living playbook).

---

*Maintain this doc as the single source of truth for public claims. When a roadmap item ships
(Layer 2 document assembly, verification API), update the Claims Ledger here before any post
mentions it.*
