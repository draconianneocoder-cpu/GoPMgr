# AAI — Agentic AI

Source: OWASP Cornucopia, Companion edition v1.0. Card text CC-BY-SA-4.0, OWASP Cornucopia project.

GoPMgr's own product has no LLM/agent features — none of these cards describe a GoPMgr threat. They're here as a structured checklist for auditing how an AI coding agent (not GoPMgr) operates on this repository: use them as prompts when reviewing a session's own tool-use conduct, the way `go-high-assurance`'s Gate A/B/C already does informally. This file is a question list, not a scorecard —
it makes no claim about which questions are currently answered well; that assessment, if done, belongs in a dated point-in-time record (e.g. an elicitation-pass document), never asserted as settled fact in this reference file itself, since a file claiming its own safety coverage is exactly the kind of unverified claim these cards warn against (see AAI3, AAI5).

## AAI2 (value 2)
Tay can misinterpret user intent due to insufficient context isolation or prompt enforcement and execute actions outside the expected task scope

- OWASP AISVS: 2.1.1-8, 2.2.3-4, 5.1.2, 5.2.1-2, 5.2.4-5, 8.2.3-5, 8.3.3, 9.3.1, 9.3.3-8, 9.5.1-5, 12.1.1-3, 12.2.1-5, 12.3.1-4, 12.4.1-3
- OWASP Top 10 for LLM Apps: LLM01:2025: Prompt Injection
- MITRE ATLAS: AML.T0051: Prompt Injection

## AAI3 (value 3)
Boo-Code can rely on unverified or attacker-influenced conversation history, propagating incorrect assumptions across reasoning chains

- OWASP AISVS: 2.1.1-8, 2.2.1-4, 7.1.1-2, 7.2.1-3, 7.3.4, 8.2.3-5, 8.3.2-3, 11.4.1-3, 12.1.1-3, 12.2.1-5, 12.3.1-4, 12.4.1-3
- OWASP Top 10 for LLM Apps: LLM04:2025: Data & Model Poisoning
- MITRE ATLAS: AML.T0080 - AI Agent Context Poisoning

## AAI4 (value 4)
MissTrial can autonomously loop or chain external tool calls without enforcing rate limits or budget controls

- OWASP AISVS: 9.1.1-3, 9.3.3-4, 9.6.1-3, 12.1.1-3, 12.2.1-5, 12.3.1-4, 12.4.1-3
- OWASP Top 10 for LLM Apps: LLM10:2025: Unbounded Consumption
- MITRE ATLAS: AML.T0034: Cost Harvesting, AML.T0029: Denial of AI Service

## AAI5 (value 5)
Watson can reveal sensitive internal instructions, policies, or reasoning artifacts when exposed to adversarial prompting patterns

- OWASP AISVS: 2.1.1-8, 2.2.3-4, 5.2.4, 7.1.1, 7.3.2, 9.5.4, 10.4.1-8, 11.1.4, 12.1.1-3, 12.2.1-5, 12.3.1-4, 12.4.1-3
- OWASP Top 10 for LLM Apps: LLM02:2025: Sensitive Information Disclosure
- MITRE ATLAS: AML.T0015: Evade AI Model

## AAI6 (value 6)
Gremlini can access and process sensitive data sources beyond user authorization due to insufficient access validation

- OWASP AISVS: 5.1.2, 5.2.1-7, 8.1.1, 8.1.3, 9.3.1-8, 9.5.1-3, 9.5.5-6, 9.6.1-3, 12.1.1-3, 12.2.1-5, 12.3.1-4, 12.4.1-3
- OWASP Top 10 for LLM Apps: LLM06:2025: Excessive Agency
- MITRE ATLAS: AML.T0086: Exfiltration via AI Agent Tool Invocation

## AAI7 (value 7)
Auto-GPT can treat external tool outputs as authoritative and execute embedded malicious instructions without validation

- OWASP AISVS: 2.1.1-8, 7.3.3-4, 8.1.3, 8.2.3-5, 9.3.2-8, 10.4.1-8, 11.4.1-3, 12.1.1-3, 12.2.1-5, 12.3.1-4, 12.4.1-3
- OWASP Top 10 for LLM Apps: LLM01:2025: Prompt Injection
- MITRE ATLAS: AML.T0051.001: LLM Prompt Injection: Indirect

## AAI8 (value 8)
PreCursor can execute unintended code or system actions when tool input validation and sandboxing controls are weak

- OWASP AISVS: 5.1.2, 5.2.1-2, 5.2.5, 9.3.1, 9.3.4-5, 9.3.7-8, 9.5.1-3, 9.5.5-6, 12.1.1-3, 12.2.1-6, 12.3.1-4, 12.4.1-3
- OWASP Top 10 for LLM Apps: LLM06:2025: Excessive Agency
- MITRE ATLAS: AML.T0011: User Execution

## AAI9 (value 9)
CoPirate can modify configurations, permissions, or system settings beyond intended authorization due to excessive autonomy

- OWASP AISVS: 5.1.1-2, 5.2.1, 5.2.5-6, 9.2.1-10, 9.3.1-5, 9.3.7-8, 9.5.1-3, 12.1.1-3, 12.2.1-5, 12.3.1-4, 12.4.1-3
- OWASP Top 10 for LLM Apps: LLM06:2025: Excessive Agency
- MITRE ATLAS: AML.T0081: Modify AI Agent Configuration

## AAIX (value 10)
DeepGeek can autonomously plan and execute multi-step operations across systems without detecting malicious intermediate objectives

- OWASP AISVS: 5.1.2, 5.2.5, 9.2.1-10, 9.3.5-7, 9.5.1-3, 9.5.5-6, 9.6.2, 11.4.1-3, 12.1.1-3, 12.2.1-5, 12.3.1-4, 12.4.1-3
- OWASP Top 10 for LLM Apps: LLM06:2025: Excessive Agency
- MITRE ATLAS: AML.T0102: Generate Malicious Commands

## AAIJ (value J)
BabyAGI can trust instructions from peer agents without verification, policy validation, or identity assurance

- OWASP AISVS: 5.1.2, 5.2.5, 9.4.1-4, 9.5.5-6, 12.1.1-3, 12.2.1-5, 12.3.1-4, 12.4.1-3
- OWASP Top 10 for LLM Apps: LLM06:2025: Excessive Agency
- MITRE ATLAS: AML.T0067: LLM Trusted Output Components Manipulation

## AAIQ (value Q)
Jane can execute attacker-defined workflows at scale once the orchestration or control plane is compromised

- OWASP AISVS: 5.1.1-2, 5.2.1, 5.2.5-6, 9.2.3-4, 9.2.6-10, 9.3.7, 9.4.1-4, 9.5.3, 9.5.5-6, 9.6.2, 12.1.1-3, 12.2.1-5, 12.3.1-4, 12.4.1-3
- OWASP Top 10 for LLM Apps: LLM03:2025: Supply Chain
- MITRE ATLAS: AML.T0081: Modify AI Agent Configuration

## AAIK (value K)
GPI-3.1415 can execute high-impact operations across integrated systems due to excessive agency and lack of transactional safeguards

- OWASP AISVS: 5.1.1-2, 5.2.1, 5.2.5-6, 9.2.1-10, 9.3.5-8, 9.5.1-6, 9.6.2, 12.1.1-3, 12.2.1-6, 12.3.1-4, 12.4.1-3
- OWASP Top 10 for LLM Apps: LLM06:2025: Excessive Agency
- MITRE ATLAS: AML.T0053: AI Agent Tool Invocation

## AAIA (value A)
You have identified an attack that misuses inherent Agentic AI functionality or a related design flaw

- OWASP AISVS: —
- OWASP Top 10 for LLM Apps: —
- MITRE ATLAS: —

