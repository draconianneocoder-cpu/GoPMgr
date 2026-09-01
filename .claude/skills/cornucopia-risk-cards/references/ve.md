# VE — DATA VALIDATION & ENCODING

Source: OWASP Cornucopia, Website App edition v3.0. Card text CC-BY-SA-4.0, OWASP Cornucopia project.

This is a snapshot of the deck's questions as prompts for eliciting requirements — not a coverage claim. A card listed here is a question to ask about the code being touched, not evidence the question was already answered.

## VE2 (value 2)
Brian can gather information about the underlying configurations, schemas, logic, code, software, services and infrastructure due to the content of error messages, or poor configuration, or the presence of default installation files or old, test, backup or copies of resources, or exposure of source code

- ASVS: 2.4.1, 4.3.2, 13.2.2, 13.4.1-7, 15.2.3, 16.2.5, 16.3.4, 16.4.2, 16.5.1, 17.1.1
- STRIDE: Information Disclosure
- CAPEC: 54, 113, 116, 143, 144, 149, 150, 155, 169, 215, 224, 497, 541, 546

## VE3 (value 3)
Robert can input malicious data because the allowed protocol format is not being checked, or duplicates are accepted, or the structure is not being verified, or the individual data elements are not being sanitized, or preferably validated for format, type, range, size, length and a whitelist of allowed characters or formats

- ASVS: 1.1.1, 1.2.1-3, 1.3.1-12, 1.4.2, 1.5.3, 2.1.1, 2.2.1-2, 3.2.3, 3.5.3, 3.5.5, 4.1.1, 4.1.4, 4.2.1-5, 5.1.1, 5.2.1-6, 5.3.1-3, 5.4.1-2, 15.3.3, 15.3.5-7, 16.3.3-4, 16.5.1
- STRIDE: Tampering
- CAPEC: 28, 33, 39, 48, 64, 105, 126, 152, 153, 165, 175, 220, 231, 261, 272, 586

## VE4 (value 4)
Dave can input malicious field names or data because it is not being checked within the context of the current user and process

- ASVS: 2.1.1-3, 2.2.1-3, 2.3.1-3, 15.3.3-7, 16.3.3, 16.5.1
- STRIDE: Tampering
- CAPEC: 28, 39, 113, 137, 140, 162

## VE5 (value 5)
Jee can bypass the centralized encoding routines since they are not being used everywhere, or the wrong encodings are being used

- ASVS: 1.1.1-2, 1.2.1-3, 1.2.5, 1.3.1, 1.3.3, 1.3.7, 3.2.2, 5.4.2, 16.4.1, 16.5.1
- STRIDE: Tampering, Information Disclosure
- CAPEC: 28, 43, 64, 72, 120, 152, 267

## VE6 (value 6)
Jason can bypass the centralized validation routines since they are not being used on all inputs

- ASVS: 1.4.2, 2.1.1-3, 2.2.1-3, 2.3.1-3, 16.5.1
- STRIDE: Tampering
- CAPEC: 28, 43, 77, 140, 152, 153

## VE7 (value 7)
Jan can craft special payloads to foil input validation because the character set is not specified/enforced, or the data is encoded multiple times, or the data is not fully converted into the same format the application uses (e.g. canonicalization) before being validated, or variables are not strongly typed

- ASVS: 1.1.1-2, 1.2.1-3, 1.2.9-10, 2.1.1, 2.2.1, 1.2.1-3, 3.2.2-3, 4.1.1, 5.4.2, 15.3.5-6, 16.5.1
- STRIDE: Tampering
- CAPEC: 3, 4, 28, 43, 52, 64, 71, 72, 78, 79, 80, 120, 126, 152, 153, 267

## VE8 (value 8)
Oana can bypass the centralized sanitization routines since they are not being used comprehensively

- ASVS: 1.1.1-2, 1.2.3, 1.3.1-12, 16.5.1-4
- STRIDE: Tampering
- CAPEC: 28, 66, 88, 135, 136, 141, 152, 160, 183, 250, 267, 664, 676

## VE9 (value 9)
Shamun can bypass input validation or output validation checks because validation failures are not rejected and/or sanitized

- ASVS: 16.3.3, 16.5.1-4
- STRIDE: Tampering
- CAPEC: 24, 28, 152, 153, 198

## VEX (value 10)
Darío can exploit the trust the application places in a source of data (e.g. user-definable data, manipulation of locally stored data, alteration to state data on a client device, lacking and/or improper enforcement of client-side controls, lack of verification of identity during data validation such as Darío can pretend to be Colin)

- ASVS: 1.3.6, 2.2.1-2, 3.2.1, 3.3.1-5, 3.4.3-4, 3.4.6-7, 3.5.1-5, 3.6.1, 3.7.4, 4.1.3, 4.1.5, 6.8.2, 9.1.1-3, 9.2.2-4, 10.4.1
- STRIDE: Spoofing
- CAPEC: 22, 39, 51, 62, 111, 145, 194, 200, 218, 220, 466, 475, 502, 543, 690

## VEJ (value J)
Toby has control over input validation, output validation, sanitization or output encoding code or routines so they can be bypassed

- ASVS: 2.2.2, 3.1.1, 3.4.3, 3.4.6-7, 3.5.3-7, 3.6.1, 3.7.5
- STRIDE: Tampering
- CAPEC: 77, 87, 202, 207, 271, 446, 554

## VEQ (value Q)
Xavier can inject data into a client or device side interpreter because a parameterised interface is not being used, or has not been implemented correctly, or the data has not been encoded, sanitized or escaped correctly for the context, or there is no restrictive policy on code or data includes

- ASVS: 1.2.1-3, 1.3.1-7, 3.1.1, 3.2.1-3, 3.4.3, 3.4.6-8, 3.5.4-8, 3.6.1, 3.7.1, 3.7.5, 4.1.1, 15.3.5-6, 16.3.3
- STRIDE: Tampering
- CAPEC: 19, 63, 104, 152, 160, 182, 267

## VEK (value K)
Gabe can inject data into a server-side interpreter (e.g. SQL, OS commands, Xpath, Server JavaScript, SMTP) because a strongly typed parameterised interface is not being used, not implemented correctly, or properly configured

- ASVS: 1.1.1-2, 1.2.2, 1.2.4-10, 1.3.2-11, 1.5.1-3, 2.1.1-3, 2.2.1-3, 5.1.1, 5.2.1-6, 5.3.1-3, 5.4.1-3, 15.3.1-3, 15.3.5-7, 16.4.1, 16.5.1
- STRIDE: Tampering
- CAPEC: 19, 23, 28, 66, 83, 88, 93, 126, 136, 137, 153, 160, 175, 183, 250, 253, 261, 664, 676

## VEA (value A)
You have invented a new attack against Data Validation and Encoding

- ASVS: -
- STRIDE: —
- CAPEC: -

