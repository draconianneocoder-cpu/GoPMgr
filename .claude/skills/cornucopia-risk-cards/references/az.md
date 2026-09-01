<!--
SPDX-FileCopyrightText: 2025 OWASP Foundation
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: CC-BY-SA-4.0
-->

# AZ — AUTHORIZATION

Source: OWASP Cornucopia, Website App edition v3.0. Card text CC-BY-SA-4.0, OWASP Cornucopia project.

This is a snapshot of the deck's questions as prompts for eliciting requirements — not a coverage claim. A card listed here is a question to ask about the code being touched, not evidence the question was already answered.

## AZ2 (value 2)
Tim can influence where data is sent or forwarded to

- ASVS: 3.2.1, 3.3.2, 3.4.1-8, 3.5.1-8, 3.7.2-4, 8.3.1, 8.4.1, 10.2.1-2, 13.1.1, 13.2.4-5, 15.3.2
- STRIDE: Tampering
- CAPEC: 62, 94, 154, 157, 173, 240, 481, 569

## AZ3 (value 3)
Christian can access information, which he should not have permission to, through another mechanism that does have permission (e.g. search indexer, logger, reporting), or because it is cached, or kept for longer than necessary, or through other information leakage

- ASVS: 5.3.2, 5.4.1-3, 8.3.3, 8.4.2, 13.4.5, 14.2.2, 14.2.5, 14.2.7, 14.3.1-3
- STRIDE: Information Disclosure
- CAPEC: 30, 69, 126, 204, 234

## AZ4 (value 4)
Kelly can bypass authorization controls because they do not fail securely (i.e. they default to allowing access)

- ASVS: 16.3.2, 16.5.3
- STRIDE: Elevation of Privilege
- CAPEC: 180

## AZ5 (value 5)
Chad can access resources (including services, processes, AJAX, video, images, documents, temporary files, session data, system properties, configuration data, registry settings, logs) he should not be able to due to missing authorization, or due to excessive privileges (e.g. not using the principle of least privilege)

- ASVS: 8.1.1, 10.2.3, 13.2.2, 13.3.2, 16.3.3, 16.4.2
- STRIDE: Elevation of Privilege
- CAPEC: 54, 58, 75, 77, 87, 122, 126, 143, 144, 149, 155, 203, 240, 268

## AZ6 (value 6)
Eduardo can access data he does not have permission to, even though he has permission to the form/page/URL/entry point

- ASVS: 8.1.1-2, 8.2.2-3, 16.3.2
- STRIDE: Elevation of Privilege
- CAPEC: 58, 122

## AZ7 (value 7)
Yuanjing can access application functions, objects, or properties he is not authorized to access

- ASVS: 8.1.1-2, 8.2.1-3, 15.3.1, 16.3.2
- STRIDE: Elevation of Privilege
- CAPEC: 58, 122, 212

## AZ8 (value 8)
Tom can bypass business rules by altering the usual process sequence or flow, or by undertaking the process in the incorrect order, or by manipulating date and time values used by the application, or by using valid features for unintended purposes, or by otherwise manipulating control data

- ASVS: 2.1.3, 2.3.1-5, 3.5.7, 8.3.1, 16.3.2-3
- STRIDE: Tampering, Elevation of Privilege
- CAPEC: 39, 74, 162, 166, 172, 207, 212

## AZ9 (value 9)
Michael can bypass the application to gain access to data because administrative tools or administrative interfaces are not secured adequately

- ASVS: 6.1.1, 6.2.2, 6.2.4, 6.2.11, 6.3.1-2, 6.3.8, 6.4.2, 7.5.3, 8.4.2, 13.2.1-3, 13.3.2, 13.4.2-5, 16.3.1-4, 17.1.1
- STRIDE: Elevation of Privilege
- CAPEC: 1, 36, 49, 87, 121, 127, 169

## AZX (value 10)
Richard can bypass the centralized authorization controls since they are not being used comprehensively on all interactions, or because they have been misconfigured, or because the application does not use a centralized standard, tested, proven, recommended and approved authorization module/framework/service

- ASVS: 8.3.1-2, 8.4.1, 10.4.11, 13.2.2, 13.3.2, 16.3.2-3
- STRIDE: Elevation of Privilege
- CAPEC: 1, 22, 36, 95, 121, 179, 180

## AZJ (value J)
Dinis can access security configuration information, or access control lists

- ASVS: 3.1.1, 3.7.5, 8.1.1, 8.1.4, 8.2.1, 8.3.1, 8.4.2, 13.2.2, 13.3.2, 13.4.1, 13.4.7, 14.2.4, 16.3.2-3
- STRIDE: Information Disclosure, Elevation of Privilege
- CAPEC: 1, 11, 75, 116, 133, 176, 179, 180, 207

## AZQ (value Q)
Christopher can inject a command that the application will run at a higher privilege level

- ASVS: 1.1.1-2, 1.2.1-10, 1.3.1-10, 3.1.1, 3.4.2-3, 3.4.6-8, 3.5.2-8, 3.6.1, 5.1.1, 5.2.2, 5.3.1-2, 5.4.1-3, 6.7.1, 8.3.3, 8.4.2, 11.6.1-2, 13.3.1-3, 13.4.5, 15.2.5, 16.3.3, 16.4.1
- STRIDE: Elevation of Privilege
- CAPEC: 93, 122, 233, 242, 248, 636

## AZK (value K)
Adrian can influence or alter authorization controls and permissions, and can therefore bypass them

- ASVS: 7.2.1, 8.1.1-4, 8.2.1, 8.2.4, 8.3.1-3, 8.4.2, 13.2.3, 14.2.4, 16.3.3
- STRIDE: Elevation of Privilege
- CAPEC: 176, 207, 554

## AZA (value A)
You have invented a new attack against Authorization

- ASVS: -
- STRIDE: —
- CAPEC: -

