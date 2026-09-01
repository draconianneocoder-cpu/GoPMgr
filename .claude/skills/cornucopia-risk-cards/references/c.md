# C — CORNUCOPIA

Source: OWASP Cornucopia, Website App edition v3.0. Card text CC-BY-SA-4.0, OWASP Cornucopia project.

This is a snapshot of the deck's questions as prompts for eliciting requirements — not a coverage claim. A card listed here is a question to ask about the code being touched, not evidence the question was already answered.

## C2 (value 2)
Lee can bypass application controls because dangerous/risky programming language functions have been used instead of safer alternatives, or there are type conversion errors, or because the application is unreliable when an external resource is unavailable, or there are race conditions, or there are resource initialization, leak or allocation issues, or overflows can occur

- ASVS: 1.4.1-3, 3.7.1, 11.2.4, 11.2.5, 11.3.1, 15.1.1-5, 15.2.1-2, 15.2.5, 15.4.1-4, 16.5.2-3
- STRIDE: Tampering
- CAPEC: 25, 26, 77, 100, 124, 125, 128, 129, 130, 131, 446

## C3 (value 3)
Andrew can access source code, or decompile, or debug, or otherwise access business logic to understand how the application works and any secrets contained

- ASVS: 12.1.1-5, 12.2.1-2, 12.3.1-5, 13.3.1, 13.4.1-2, 13.4.5-7, 15.2.3, 16.3.3
- STRIDE: Information Disclosure
- CAPEC: 11, 65, 94, 188, 207

## C4 (value 4)
Keith can perform an action and it is not possible to attribute it to him

- ASVS: 6.3.2, 6.7.1, 6.8.2, 16.1.1, 16.2.1-5. 16.3.1-4, 16.4.1-3
- STRIDE: Repudiation
- CAPEC: 268

## C5 (value 5)
Larry can influence the trust other parties including users have in the application, or abuse that trust elsewhere (e.g. in another application)

- ASVS: 3.1.1, 3.2.1, 3.3.1-4, 3.4.1-7, 3.5.1, 3.5.4, 3.7.1-5, 9.1.1-3, 9.2.1-4, 10.1.1-2, 10.3.1-5, 10.4.1-16, 10.5.4, 11.1.1-2, 12.1.1-4, 12.2.1-2, 12.3.1-5, 13.2.1-5
- STRIDE: Spoofing
- CAPEC: 21, 22, 57, 89, 103, 181, 473

## C6 (value 6)
Aaron can bypass controls because error/exception handling is missing, or is implemented inconsistently or partially, or does not deny access by default (i.e. errors should terminate access/execution), or relies on handling by some other service or system

- ASVS: 11.2.5, 12.2.1, 12.3.1, 12.3.3, 16.3.2-4, 16.5.3
- STRIDE: Tampering
- CAPEC: 54, 114, 217

## C7 (value 7)
Mwengu's actions cannot be investigated because there is not an adequate accurately time-stamped record of security events, or there is not a full audit trail, or these can be altered or deleted by Mwengu, or there is no centralized logging service

- ASVS: 16.1.1, 16.2.1-4, 16.3.1-4, 16.4.1-3
- STRIDE: Repudiation
- CAPEC: 268

## C8 (value 8)
David can bypass the application to gain access to data because the network and host infrastructure, and supporting services/applications, have not been securely configured, the configuration rechecked periodically and security patches applied, or the data is stored locally, or the data is not physically protected

- ASVS: 12.1.1-5, 12.2.1-2, 12.3.1-5, 13.2.1-3, 13.3.1-4, 13.4.1-7, 15.1.1-2, 15.2.1, 15.2.4, 16.3.3-4
- STRIDE: Information Disclosure
- CAPEC: 37, 121, 159, 169, 217, 220, 310, 446

## C9 (value 9)
Mike can misuse an application by using a valid feature too fast, or too frequently, or other way that is not intended, or consumes the application's resources, or causes race conditions, or over-utilizes a feature

- ASVS: 2.2.1-2, 2.3.2, 2.3.4, 2.4.1-2, 15.3.3, 15.4.1-4, 16.3.3
- STRIDE: Tampering
- CAPEC: 26, 125, 130, 212, 227, 469

## CX (value 10)
Spyros can circumvent the application's controls because code frameworks, libraries and components contain malicious code or vulnerabilities (e.g. in-house, commercial off the shelf, outsourced, open source, externally-located)

- ASVS: 6.7.1, 11.4.1, 11.4.3, 11.6.1-2, 13.3.1-3, 15.1.1-2, 15.1.4-5, 15.2.1, 15.2.4-5, 16.3.1-4
- STRIDE: Tampering
- CAPEC: 68, 159, 206, 442, 446, 523, 538, 673, 691

## CJ (value J)
Roman can exploit the application because it was insecurely compiled or deployed, or its configuration is not secure by default, or because security information was not documented, or passed on to operational teams, or the user is not warned and access blocked when the expected security features are unsupported or disabled

- ASVS: 3.1.1, 3.4.3, 3.4.7, 3.6.1, 3.7.5, 8.2.1-4, 8.3.1, 8.4.2, 13.1.1, 13.2.1-3, 13.3.2, 13.4.1-7, 15.2.3, 15.2.5, 16.3.2-4
- STRIDE: Tampering
- CAPEC: 70, 121, 127, 133, 176, 180, 191, 207

## CQ (value Q)
Jim can undertake malicious, non-normal, actions without real-time detection and response by the application

- ASVS: 6.3.5, 6.3.7, 8.1.3-4, 8.3.2, 8.4.2, 16.3.1-4
- STRIDE: Repudiation
- CAPEC: 156, 268

## CK (value K)
Grant can utilize the application to deny service to some or all of its users

- ASVS: 1.3.12, 2.3.2, 2.4.1-2, 4.2.5, 4.3.1, 5.2.1, 5.2.6, 6.1.1, 6.3.1, 6.4.5-6, 10.5.5, 10.6.2, 13.1.2-3, 13.2.6, 15.1.3, 15.3.3, 15.4.3-4, 16.3.3, 16.5.2, 17.1.2, 17.3.1-2
- STRIDE: Denial of Service
- CAPEC: 2, 25, 100, 125, 130, 227, 572, 607

## CA (value A)
You have invented a new attack of any type

- ASVS: -
- STRIDE: —
- CAPEC: -

