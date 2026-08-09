<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Test-only PKCS#12 fixtures

These `.p12` files hold self-signed, non-production key material used only
by `internal/crypto`'s tests. None of them sign anything real; each is a
throwaway RSA or EC key generated for this repository and nothing else.
All three use the public deterministic fixture passphrase
`gopmgr-test-only`; it is not a production credential.

- `testonly-rsa-2bag.p12` — one RSA key + its own self-signed certificate
  (the "2 safe bags" shape `golang.org/x/crypto/pkcs12`'s `Decode` requires).
  Regression fixture: every certificate that worked with `LoadCertificate`
  before the 2026-08-05 fix matches this shape.
- `testonly-rsa-3bag.p12` — the same RSA key/cert plus one unrelated extra
  certificate bundled in, reproducing the shape a commercially-issued
  signing certificate has when exported with its issuing chain. This is
  the fixture that reproduces the "expected exactly two safe bags in the
  PFX PDU" bug fixed 2026-08-05 (see `DEVELOPER_HANDBOOK.md`), and the only
  way `LoadCertificate`'s `ExtraCerts` extraction is exercised at all.
- `testonly-ec-2bag.p12` — one EC (P-256) key + its own self-signed
  certificate, for testing that `LoadCertificate` still rejects non-RSA
  keys after the rewrite.

## Regenerating

All three were built with OpenSSL 3.6's `-legacy` flag. This is
deliberate, not incidental: `golang.org/x/crypto/pkcs12` is a frozen,
decode-only package (see its own doc comment) that only understands the
older RC2/3DES-based PKCS#12 encryption schemes. OpenSSL 3.x defaults to
AES-256-CBC/PBKDF2, which that package cannot decrypt at all. Regenerating
these without `-legacy` will produce a `.p12` that OpenSSL can read but Go
cannot — if that happens, this is why.

```sh
# Leaf key + cert (used by both RSA fixtures)
openssl req -x509 -newkey rsa:2048 -keyout leaf.key -out leaf.crt \
  -days 3650 -nodes -subj "/CN=GoPMgr Test RSA Signer"

# Unrelated extra cert bundled into the 3-bag fixture
openssl req -x509 -newkey rsa:2048 -keyout extra.key -out extra.crt \
  -days 3650 -nodes -subj "/CN=GoPMgr Test Extra Cert"

openssl pkcs12 -export -legacy -out testonly-rsa-2bag.p12 \
  -inkey leaf.key -in leaf.crt -passout pass:gopmgr-test-only

openssl pkcs12 -export -legacy -out testonly-rsa-3bag.p12 \
  -inkey leaf.key -in leaf.crt -certfile extra.crt \
  -passout pass:gopmgr-test-only

# EC key + cert
openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 \
  -keyout ec.key -out ec.crt -days 3650 -nodes \
  -subj "/CN=GoPMgr Test EC Signer"

openssl pkcs12 -export -legacy -out testonly-ec-2bag.p12 \
  -inkey ec.key -in ec.crt -passout pass:gopmgr-test-only
```
