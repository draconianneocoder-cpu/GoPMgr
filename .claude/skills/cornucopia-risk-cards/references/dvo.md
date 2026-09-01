<!--
SPDX-FileCopyrightText: 2025 OWASP Foundation
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: CC-BY-SA-4.0
-->

# DVO — DevOps

Source: OWASP Cornucopia, Companion edition v1.0. Card text CC-BY-SA-4.0, OWASP Cornucopia project.

This is a snapshot of the deck's questions as prompts for eliciting requirements — not a coverage claim. A card listed here is a question to ask about the code being touched, not evidence the question was already answered.

## DVO2 (value 2)
Aram's malicious actions against build, delivery, and deployment processes cannot be investigated, because there is no sufficient, complete, and accurately timestamped record of security events, or it has been tampered with

- ASVS: —
- STRIDE: Repudiation, Tampering
- CWE: —

## DVO3 (value 3)
Aryan can exploit an internal system or service, because it, its infrastructure, or other components were not properly hardened, or the configuration was not maintained over time

- ASVS: —
- STRIDE: Tampering, Elevation of Privilege
- CWE: —

## DVO4 (value 4)
Bart is able to delete, overwrite, or download backups

- ASVS: —
- STRIDE: Tampering, Information Disclosure, Denial of Service
- CWE: —

## DVO5 (value 5)
Brian can escape the runtime isolation of workloads to access host resources, execute privileged operations, or use the workloads to attack other internal systems

- ASVS: —
- STRIDE: Elevation of Privilege
- CWE: —

## DVO6 (value 6)
Daniel can cause a permanent loss of applications, source code, and data due to missing, incomplete or failed backups, or insufficient recovery documentation, training or testing

- ASVS: —
- STRIDE: Denial of Service
- CWE: —

## DVO7 (value 7)
John can deploy unauthorized or malicious changes to production because deployment approval gates, validation checks, or change control processes are missing or can be bypassed

- ASVS: —
- STRIDE: Tampering
- CWE: —

## DVO8 (value 8)
Maxim can deploy a malicious or otherwise modified artifact, because its integrity is not guaranteed or validated

- ASVS: —
- STRIDE: Tampering
- CWE: —

## DVO9 (value 9)
Nariman can control or affect pipeline execution by injecting malicious commands through poisoned or typosquatted workflow dependencies, or by manipulating CI configuration files, or in other ways

- ASVS: —
- STRIDE: Tampering, Elevation of Privilege
- CWE: —

## DVOX (value 10)
Patricia can exploit obsolete DevOps credentials, identities, services, or APIs, as well as excessive privileges, to bypass access controls and gain unauthorized access to read and modify sensitive data or functionality

- ASVS: —
- STRIDE: Elevation of Privilege, Information Disclosure
- CWE: —

## DVOJ (value J)
Pravir can exploit vulnerabilities in the application or development ecosystem, including repositories and DevOps infrastructure, because of outdated or poorly maintained dependencies

- ASVS: —
- STRIDE: Elevation of Privilege
- CWE: —

## DVOQ (value Q)
Seba can access the code repository, log files, command line history, pipelines, or other places, to gain access to secrets or other sensitive information

- ASVS: —
- STRIDE: Information Disclosure
- CWE: —

## DVOK (value K)
Timo can compromise software, development environments, or DevOps tooling by injecting malicious code via external dependencies or exploited developer credentials

- ASVS: —
- STRIDE: Tampering
- CWE: —

## DVOA (value A)
You have invented a new attack against DevOps

- ASVS: —
- STRIDE: —
- CWE: —

