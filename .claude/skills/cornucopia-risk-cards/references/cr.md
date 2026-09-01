# CR — CRYPTOGRAPHY

Source: OWASP Cornucopia, Website App edition v3.0. Card text CC-BY-SA-4.0, OWASP Cornucopia project.

This is a snapshot of the deck's questions as prompts for eliciting requirements — not a coverage claim. A card listed here is a question to ask about the code being touched, not evidence the question was already answered.

## CR2 (value 2)
Kyun can access data because it has been obfuscated rather than using an approved cryptographic function

- ASVS: 11.2.1, 11.3.2-3, 11.4.1
- STRIDE: Information Disclosure
- CAPEC: 39, 97, 162, 204

## CR3 (value 3)
Axel can modify transient or permanent data (stored or in transit), or source code, or updates/patches, or configuration data, because it is not subject to integrity checking

- ASVS: 4.1.5, 6.7.1, 9.1.1-3, 11.3.3, 11.4.3, 11.3.5, 14.2.4, 16.3.1, 16.3.2, 16.3.3, 16.3.4
- STRIDE: Tampering
- CAPEC: 39, 68, 75, 94, 145, 184, 438, 442, 475, 523, 594, 690

## CR4 (value 4)
Paulo can access data in transit that is not encrypted, even though the channel is encrypted

- ASVS: 14.1.1-2, 14.2.1-8
- STRIDE: Information Disclosure
- CAPEC: 94, 117

## CR5 (value 5)
Kyle can bypass cryptographic controls because they do not fail securely (i.e. they default to unprotected)

- ASVS: 11.2.5, 12.2.1, 16.3.3, 16.3.4
- STRIDE: Tampering, Information Disclosure
- CAPEC: 24, 620

## CR6 (value 6)
Romain can read and modify unencrypted data in memory or in transit (e.g. cryptographic secrets, credentials, session identifiers, personal and commercially-sensitive data), in use or in communications within the application, or between the application and users, or between the application and external systems

- ASVS: 4.4.1, 11.7.1-2, 12.1.1-5, 12.2.1-2, 12.3.1-5, 13.4.2, 14.1.1-2, 14.2.1-8, 14.3.l-3
- STRIDE: Tampering, Information Disclosure
- CAPEC: 94, 102, 116, 117, 204

## CR7 (value 7)
Gunter can intercept or modify encrypted and/or hashed data in transit because the protocol is poorly deployed, or weakly configured, or certificates are invalid, or certificates are not trusted, or the connection can be degraded to a weaker or un-encrypted communication

- ASVS: 3.4.1, 3.7.4, 4.1.2, 11.2.1, 11.2.3-5, 11.3.1-5, 11.4.1, 11.4.3, 11.5.1-2, 11.6.1-2, 12.1.1-4, 12.2.1-2, 12.3.1-5, 16.3.3-4
- STRIDE: Tampering, Information Disclosure
- CAPEC: 39, 94, 114, 145, 157, 216, 218, 220, 272, 594, 620

## CR8 (value 8)
Eoin can access stored business data (e.g. passwords, session identifiers, PII, cardholder data) because it is not securely encrypted or securely hashed

- ASVS: 11.2.1, 11.3.1-5, 11.4.1-2, 11.6.1-2, 13.3.1, 14.1.1-2
- STRIDE: Information Disclosure
- CAPEC: 20, 31, 37, 55

## CR9 (value 9)
Andy can bypass cryptographic controls because random-number, GUID, or hashing functions are self-built, risky or weak

- ASVS: 6.8.2, 9.1.1-3, 11.2.1-5, 11.3.1-5, 11.4.1-4, 11.5.1-2, 11.6.1-2, 16.3.3
- STRIDE: Spoofing, Tampering, Information Disclosure
- CAPEC: 97, 112, 461, 473

## CRX (value 10)
Susanna can break the cryptography in use because it is not strong enough for the degree of protection required, or it is not strong enough for the amount of effort the attacker is willing to make

- ASVS: 11.2.1-5, 11.3.1-3, 11.4.1-4, 11.5.1-2, 11.6.1-2, 16.3.3
- STRIDE: Tampering, Information Disclosure
- CAPEC: 97, 112, 463

## CRJ (value J)
Justin can read credentials for accessing internal or external resources, services and others systems because they are stored in an unencrypted format, or saved in the source code

- ASVS: 11.1.1-4, 13.1.4, 13.3.1, 13.3.3, 14.2.2, 16.2.5, 17.2.1
- STRIDE: Information Disclosure
- CAPEC: 37, 57, 155, 204, 474, 639

## CRQ (value Q)
Artim can access or predict the master cryptographic secrets

- ASVS: 11.1.1-4, 11.2.1-5, 11.3.1-5, 11.4.1, 11.4.3-4, 11.5.1-2, 11.6.1-2, 13.1.4, 13.3.1, 13.3.3, 14.2.2, 16.2.5, 16.3.3, 17.2.1
- STRIDE: Information Disclosure
- CAPEC: 20, 37, 57, 97, 155, 204, 474, 639

## CRK (value K)
Dan can influence or alter cryptography code/routines (encryption, hashing, digital signatures, random number and GUID generation) and can therefore bypass them

- ASVS: 1.3.2, 1.4.1-3, 5.4.3, 13.3.1-3, 13.4.2, 15.1.1-2, 15.2.1, 15.2.4, 16.3.3, 16.5.1-4
- STRIDE: Tampering
- CAPEC: 184, 207, 444, 523

## CRA (value A)
You have invented a new attack against Cryptography

- ASVS: -
- STRIDE: —
- CAPEC: -

