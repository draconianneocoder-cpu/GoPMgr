<!--
SPDX-FileCopyrightText: 2025 OWASP Foundation
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: CC-BY-SA-4.0
-->

# AT — AUTHENTICATION

Source: OWASP Cornucopia, Website App edition v3.0. Card text CC-BY-SA-4.0, OWASP Cornucopia project.

This is a snapshot of the deck's questions as prompts for eliciting requirements — not a coverage claim. A card listed here is a question to ask about the code being touched, not evidence the question was already answered.

## AT2 (value 2)
James can undertake authentication functions without the real user ever being aware this has occurred (e.g. attempt to log in, log in with stolen credentials, reset the password) 

- ASVS: 6.3.5, 6.3.7, 7.5.2, 16.1.1, 16.2.1-5, 16.3.1-4, 16.4.3, 16.5.1-4
- STRIDE: Repudiation
- CAPEC: 21, 49, 50, 151, 600

## AT3 (value 3)
Muhammad can obtain a user's password or other secrets such as MFA codes or biometrics, by observation during entry, or from a local cache, or from memory, or in transit, or by reading it from some unprotected location, or because it is widely known or leaked

- ASVS: 6.2.6, 6.2.10-12, 6.3.1-3, 6.3.8, 6.5.7, 12.2.1, 12.3.1-5, 13.3.1, 14.2.1-2, 14.2.6, 14.3.1-3
- STRIDE: Information Disclosure
- CAPEC: 37, 49, 116, 151, 543, 560, 568, 654

## AT4 (value 4)
Sebastien can easily identify user names or can enumerate them

- ASVS: 4.4.1-4, 6.1.1, 6.3.1-2, 6.3.8, 8.2.4, 16.3.3, 16.5.1, 16.5.3
- STRIDE: Information Disclosure
- CAPEC: 70, 116, 383

## AT5 (value 5)
Javier can use default, test or easily guessable credentials to authenticate, or can use an old account, or an account not necessary for the application

- ASVS: 6.2.2, 6.2.4, 6.2.11-12, 6.3.2, 6.4.2, 13.2.3
- STRIDE: Spoofing
- CAPEC: 16, 70, 116, 151, 560, 565, 600

## AT6 (value 6)
Sven can reuse a temporary password, a recovery-, activation-, auth-, or MFA code because it's not changed after use, or it can not be reset by the user or admin, or it has an insufficiently implemented, too long or no expiry, or is still valid after being used, reset or revoked, or it does not use a secure out-of-band delivery method (e.g. post, mobile app, SMS)

- ASVS: 6.3.6, 6.4.1, 6.4.6, 6.5.1, 6.5.5-6, 6.5.8, 6.6.1-2, 6.8.3, 9.2.1, 10.4.2-3, 10.4.5, 10.4.9, 13.3.4
- STRIDE: Spoofing
- CAPEC: 50, 151, 633

## AT7 (value 7)
Cecilia can use brute force and dictionary attacks against one or many accounts without limit, or these attacks are simplified due to insufficient complexity, length, expiration and requirements for the use of passwords, recovery-, activation-, or MFA codes

- ASVS: 6.1.1, 6.2.1-2, 6.2.4-5, 6.2.8-12, 6.3.1-3, 6.3.5, 6.3.8, 6.4.1-3, 6.5.1-5, 6.6.3-4, 16.3.1, 16.3.3
- STRIDE: Spoofing
- CAPEC: 2, 49

## AT8 (value 8)
Kate can bypass authentication because it does not fail secure (i.e. it defaults to allowing unauthenticated access)

- ASVS: 16.3.1, 16.3.3-4, 16.5.2-3
- STRIDE: Spoofing
- CAPEC: 114, 461

## AT9 (value 9)
Claudia can undertake more critical functions because authentication requirements are inconsistent, too weak (e.g. do not use passkeys or other strong authentication such as a recommended MFA method), or there is no requirement to re-authenticate for these

- ASVS: 6.1.1, 6.1.3, 6.2.3-4, 6.3.3-7, 6.4.2-4, 6.4.6, 6.5.1-8, 6.6.1-4, 6.7.1-2, 7.5.1, 7.5.3, 8.1.3-4, 8.4.2
- STRIDE: Spoofing
- CAPEC: 114, 151

## ATX (value 10)
Pravin can bypass authentication controls because a centralized standard, tested, proven, recommended and approved authentication module/framework/service, separate to the resource being requested, is not being used, has been misconfigured, or has been improperly implemented

- ASVS: 6.1.1-3, 6.3.1-6, 6.4.2, 6.6.1, 6.7.1, 6.8.1-4, 10.1.1-2, 10.3.1-5, 10.5.1-5
- STRIDE: Spoofing
- CAPEC: 114, 115

## ATJ (value J)
Mark can access resources or services because there is no authentication requirement, or because authentication is missing due to misconfiguration, improper design or implementation, or it was mistakenly assumed authentication would be undertaken by some other system or performed in some previous action

- ASVS: 4.4.4, 6.1.3, 6.3.3-4, 6.4.3-4, 7.5.1, 7.5.3, 8.4.2, 12.3.5, 13.2.1, 13.2.3
- STRIDE: Spoofing
- CAPEC: 115

## ATQ (value Q)
Johan can bypass authentication because it is not enforced with equal rigor for all types of authentication functionality (e.g. register, password change, password recovery, log out, administration) or across all versions/channels (e.g. mobile website, mobile app, full website, API, call centre)

- ASVS: 6.1.3, 6.4.3-4, 6.8.4, 7.5.1, 7.5.3, 13.2.1
- STRIDE: Spoofing
- CAPEC: 50, 114, 115, 179

## ATK (value K)
Olga can influence or alter authentication code/routines so they can be bypassed

- ASVS: 5.4.3, 7.2.1, 10.1.1-2, 10.2.1, 13.2.1, 13.2.3, 13.3.1, 13.3.1-4, 15.1.1-2, 15.2.4-5, 16.3.3-4, 16.5.3
- STRIDE: Tampering
- CAPEC: 115, 207, 443, 445, 446, 461, 511, 523, 554

## ATA (value A)
You have invented a new attack against Authentication

- ASVS: -
- STRIDE: —
- CAPEC: -

