<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Critical review and v2 decisions — 2026-10-28

**Reviewed input:** the five-file original proposal ZIP supplied in this conversation. **Evidence:** original text, live GoPMgr roadmap/architecture context reviewed in the preceding turn, ADR-010 v2.2 proposal, and current Atlassian primary documentation. **Status:** design inspection and text revisions only; no code, migration, credential or live Jira test executed. The user approved the *framework direction in principle*, not every new ADR detail.

## Review verdict

The v1 proposal correctly separates source observations from scheduled activities, preserves Go host authority, uses typed local gRPC, and rejects false exactly-once and deletion claims. It was not sufficiently complete for *production-safe long-lived extension infrastructure*. The main defects were a contradictory ownership model for project-less linked workspaces; overloading the generic worker abstraction with provider-specific business semantics; an asserted network broker boundary without a corresponding worker egress control; an insufficiently explicit cross-process concurrency/fencing model; a Jira-centric throughput plan; and no mandatory real second-provider maturity criterion. These are corrected or elevated to hard evidence gates; no evidence is invented.

| ID | Priority | Finding in v1 | Correction in v2 | Residual proof |
|---|---|---|---|---|
| R-01 | **Critical** | Project-owned snapshots, but read-only workspace need not have a project | Introduced account-encrypted integration workspace, separately portable project evidence and idempotent materialization saga | Exact ADR-011 schema/key/recovery and separate storage migrations |
| R-02 | **High** | Registry/provider APIs risk becoming a universal business protocol | Keep only trusted session/registry/lifecycle common; domain-specific contracts by extension family | Golden contracts, different-family synthetic extension |
| R-03 | **Critical** | Broker called an isolation layer although same-user worker can bypass it | Explicit trusted-bundled residual privileges; signed host operation policies; sandbox verification and no public executables | Real OS confinement or maintained trusted-code risk acceptance |
| R-04 | **High** | Future provider endpoint addition could become arbitrary proxy or host code sprawl | Signed first-party declarative allowlisted operation policy, independently versioned | SSRF/bypass tests and provider security review |
| R-05 | **High** | Single-owner lease absent concrete local split-brain handling | Durable fencing generation plus OS-level exclusive owner lock, not clock-only leases | Competing processes, clock skew, kill/reopen |
| R-06 | **High** | Pagination/updated watermark treated too close to a complete snapshot | Clarified observed scope coverage vs Jira global consistency, stable ID dedup, overlap and periodic inventory | Real eventually consistent tenant tests |
| R-07 | **High** | 50k target lacked provider-aware performance strategy | Sparse inventory, selective field hydration, conditional 1k issue bulk-fetch, scoped budgets, fair quota scheduling | Benchmarks incl API quotas and cost, not invented times |
| R-08 | **Medium** | OAuth gateway candidate had incomplete hosted trust analysis | Alternatives; state/CSRF defenses; conditional PKCE only if supported; hosted-secret, availability and data-flow gates; no embedded client secret | Officially supported flow, threat model and maintenance cost |
| R-09 | **High** | Project-only backup narrative missed account integration cache | Independent encrypted backups, writer gates, restore/rebind and project self-containment | Real power-loss and recovery code tests |
| R-10 | **High** | Single-source proof allowed general framework claim before non-Jira exercise | Required second **real, materially different** provider before maturity claim | Conformance and regression proof |
| R-11 | **Medium** | Remote postcondition could imply attribution | Separated observed postcondition from provider-correlated commit | Provider-specific correlation support or honest uncertainty |
| R-12 | **High** | Compatibility treated mostly as protocol only | Defined host storage + session RPC + domain schema + signed policy + provider API matrix | N-1 support and migration/rollback tests |
| R-13 | **Medium** | Local observability/privacy and support workflows underdefined | Redacted local diagnostics, no telemetry, integrity/repair and retention controls | UX, privacy review, retention/purge tests |
| R-14 | **Medium** | Test plan lacked account/no-project and adversarial cross-store cases | Added PI-031 through PI-048; retained PI-001 through PI-030 | Actual tool outputs required; all NOT RUN |

## Roadmap review: what changed and what did not

- **Unchanged:** no cloud dependency; current beta P0 first; ADR-010 MPXJ G0–G4 first installable worker; ADR-011 schema, retention, audit, edition and file-version authority; G4 + S4/S5 + Jira feasibility before C-04 read-only production release; S6 before C-06 inbound project merge; separate remote-write approval after C-06; typed native local gRPC; no marketplace or arbitrary third-party binaries.
- **Strengthened:** C-01 verifies extension family boundaries and signed broker policy; C-02 now explicitly owns account workspace vs project migration; C-03 tests conditional bulk fetch, security and auth; C-04 is project-less and read-only; C-05 has quota fairness and leases; C-08 second real provider required before generalized maturity claim; test evidence distinguishes `PASS`, `FAIL`, `BLOCKED`, `NOT RUN`.
- **Corrected apparent contradiction:** roadmap diagram now explicitly maps C-00 through C-08 dependencies; C-00 design approval is available now and does not depend on G1 implementation.
- **Not silently accepted:** a separate ADR-011 schema, Java MPXJ Windows transport proof, Go Jira Windows transport proof, OAuth topology, vendor licenses, API race semantics and stable signed packaging remain evidence gates. New open decisions require maintainer review.

## Adversarial critique of v2 and retained uncertainties

1. The new account store increases footprint and upgrade complexity; it is justified by project-less workspaces and project autonomy, but **a single-project-only first release** could eliminate the store. Such a scope alternative requires an explicit ACR and acceptance criteria; it cannot be slipped into implementation as an optimization.
2. Signing and a broker are not an OS sandbox. An adversarial signed worker could still use ambient machine privileges unless enforced; first-party code must be treated as trusted software with an honest residual risk, not as hostile safely confined code.
3. Exact application-level completeness is limited by upstream API consistency, user permissions and quotas. The proposal can guarantee only complete **local commit of the observed, declared scope** and truthful uncertainty.
4. Hosted OAuth token exchange is operationally nontrivial and may conflict with users expecting an entirely local workflow. The core app is unaffected; Jira release is BLOCKED until an acceptable user-visible optional deployment model is validated.
5. A signed operation-policy manifest provides domain extensibility but is itself a privileged release artifact. Supply-chain review remains necessary for each new provider, with license, support, route and consent review; adding a provider is deliberately *not* an unreviewed user-installable action.
6. The throughput plan is an adaptive strategy, not a guaranteed SLA. Jira quotas, network quality and tenant-specific field shape can dominate. All T2 budgets must be approved from real evidence.
7. The second-provider exercise needs a genuine distinct API/auth/data contract, not simply a renamed Jira fake, or the neutrality test is meaningless.

## Change governance

Approval status in this revision: **direction approved in principle**; supplemental ADR normative clauses and C-00 baseline acceptance still require explicit final ratification; C-01 onward remains unauthorised. The original ZIP is not overwritten. A repository change should use the normal PR review process with current HEAD, accepted ADR census, exact diff, tests and maintainer approvals. Do not allow an implementation agent to alter normative gates to turn an observed BLOCKED result into PASS.

## Primary documentation evidence consulted on 2026-10-28

- Jira enhanced JQL and consistency scope: https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issue-search/ ; https://developer.atlassian.com/cloud/jira/platform/search-and-reconcile/
- Jira REST current conditional bulk issue fetch and changelog: https://developer.atlassian.com/cloud/jira/platform/changelog/ ; https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issues/
- Jira points/quota/burst/per-record limits: https://developer.atlassian.com/cloud/jira/platform/rate-limiting/
- Jira OAuth 3LO/security: https://developer.atlassian.com/cloud/jira/platform/oauth-2-3lo-apps/ ; https://developer.atlassian.com/platform/marketplace/security-requirements/
- Jira webhooks: https://developer.atlassian.com/cloud/jira/platform/webhooks/
- Current public GoPMgr: https://github.com/draconianneocoder-cpu/GoPMgr (the original package records observed `main@ec7b73ab7d913a2398d333f6dad9ea5734ee8f93`; verify anew before code changes)
