<!--
SPDX-FileCopyrightText: 2025 OWASP Foundation
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: CC-BY-SA-4.0
-->

# FRE — Frontend

Source: OWASP Cornucopia, Companion edition v1.0. Card text CC-BY-SA-4.0, OWASP Cornucopia project.

This is a snapshot of the deck's questions as prompts for eliciting requirements — not a coverage claim. A card listed here is a question to ask about the code being touched, not evidence the question was already answered.

## FRE2 (value 2)
Marcus bypasses client-side validation and sends malformed or malicious input directly to backend APIs, triggering logic flaws, human errors, and usability issues

- ASVS: —
- STRIDE: Tampering
- CWE: CWE-20

## FRE3 (value 3)
Lena can access sensitive or confidential information because it's not removed after logout or when the client session ends, or should have ended

- ASVS: —
- STRIDE: Information Disclosure
- CWE: —

## FRE4 (value 4)
James injects JavaScript through user-controlled data that is written into the DOM, executing arbitrary code in the victim’s browser

- ASVS: —
- STRIDE: Tampering
- CWE: —

## FRE5 (value 5)
Victor compromises or replaces a third-party script loaded from a CDN and runs malicious code in every user’s browser

- ASVS: —
- STRIDE: Tampering
- CWE: —

## FRE6 (value 6)
Olga exploits malicious JavaScript to steal authentication tokens and hijack user sessions, gaining access to accounts without credentials

- ASVS: —
- STRIDE: Spoofing
- CWE: CWE-284

## FRE7 (value 7)
Carlos exploits misconfigured CORS, unsafe postMessage handling, or other client-side security vulnerabilities to read or manipulate sensitive frontend data from a malicious origin

- ASVS: —
- STRIDE: Tampering
- CWE: —

## FRE8 (value 8)
Nathan tampers with frontend JavaScript to unlock restricted features or access data that should require server-side authorization

- ASVS: —
- STRIDE: Elevation of Privilege
- CWE: —

## FRE9 (value 9)
Sophia reuses, predicts, or forges JWTs or access tokens to impersonate users and take over active sessions

- ASVS: —
- STRIDE: Spoofing
- CWE: —

## FREX (value 10)
Piotr embeds the application in a hidden or disguised frame to trick users into clicking UI elements that perform sensitive actions

- ASVS: —
- STRIDE: Tampering
- CWE: —

## FREJ (value J)
Elena uses a malicious or over-privileged browser extension to read the DOM, steal tokens, and invoke internal frontend APIs

- ASVS: —
- STRIDE: Information Disclosure
- CWE: —

## FREQ (value Q)
Kim injects persistent malicious code into frontend assets, allowing long-term control over all users’ browsers until the application is redeployed

- ASVS: —
- STRIDE: Tampering
- CWE: —

## FREK (value K)
Darius utilizes a JavaScript application to manage and control users' systems, tenants, and data

- ASVS: —
- STRIDE: Elevation of Privilege
- CWE: —

## FREA (value A)
You have invented a new attack against Frontend

- ASVS: —
- STRIDE: —
- CWE: —

