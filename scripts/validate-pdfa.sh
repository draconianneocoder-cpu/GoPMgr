#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# PDF/A-2u validation gate using veraPDF.
#
# This gate ensures that the PDFs we claim are PDF/A-2u really are.
# It generates representative samples from the live renderers, including a
# PAdES-signed document, one with Greek, Cyrillic, Vietnamese, symbols, and
# emoji, and one each in Japanese, Korean, and Simplified Chinese set in IBM
# Plex Sans JP, KR, and SC, and validates each with veraPDF (2u profile).
# Exit 1 on any non-compliant PDF.
#
# Hard gate. By default (GOPMGR_PDFA_STRICT=1) a missing validator, a missing
# ICC profile, or a missing sample set is a FAILURE, not a silent skip: the
# release gate must never certify PDF/A-2u conformance it could not actually
# check. Run with GOPMGR_PDFA_STRICT=0 for local convenience on machines
# without Docker/veraPDF, where those preconditions degrade to a warned skip.
# `scripts/check-release.sh` always invokes this script strict.
#
# Requirements (strict mode):
#   - Docker (preferred), a veraPDF CLI on PATH, or Java to run the pinned
#     veraPDF installer this gate downloads
#   - `make icc` has been run at least once (sRGB.icc present)
#
# Usage:
#   make check-pdfa                       # strict by default
#   GOPMGR_PDFA_STRICT=0 make check-pdfa # degrade missing tooling to a skip
#   GOPMGR_VERAPDF_FORCE_CLI=1 make check-pdfa # bypass Docker for CLI diagnostics

set -eu
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

. "$ROOT/scripts/validate-pdfa-lib.sh"

# Strictness. When strict (the default, and always under the release gate),
# unmet preconditions fail the gate. When non-strict, they degrade to a warned
# skip so developers without Docker/veraPDF are not blocked locally.
PDFA_STRICT="${GOPMGR_PDFA_STRICT:-1}"

# pdfa_precondition_unmet <human-readable reason>
# Fails under strict mode, skips (exit 0) otherwise.
pdfa_precondition_unmet() {
    if [ "$PDFA_STRICT" = "1" ]; then
        echo "FAIL: $1"
        echo "The PDF/A-2u gate is strict: install Docker, a veraPDF CLI, or Java, and run 'make icc'."
        echo "Set GOPMGR_PDFA_STRICT=0 to downgrade this to a local skip."
        exit 1
    fi
    echo "SKIP: $1"
    echo "(non-strict) Set GOPMGR_PDFA_STRICT=1 to make this a hard failure; the release gate does."
    exit 0
}

ICC_PROFILE="${GOPMGR_ICC_PROFILE:-internal/pdfmeta/assets/sRGB.icc}"
VERAPDF_VERSION="1.30.2"
VERAPDF_IMAGE="verapdf/cli:v${VERAPDF_VERSION}"
# Without Docker, the gate installs veraPDF's own versioned installer, pinned
# by SHA-256. veraPDF publishes no plain CLI archive for this release on
# GitHub or Maven Central; the URLs older versions of this gate used return
# 404.
VERAPDF_INSTALLER_URL="$(verapdf_installer_url "$VERAPDF_VERSION")"
VERAPDF_INSTALLER_SHA256="6cc6341cb1af644044054b81f00a6590a7918abb18f762243de115258bcad838"
# Cache and execute veraPDF from a user-owned, repo-local directory rather
# than world-writable /tmp. Downloading and then running `java -jar` from a
# predictable /tmp path let a co-tenant pre-plant /tmp/verapdf-app.jar (a
# symlink or a malicious jar) that this gate would execute. $ROOT/.tmp is the
# same gitignored, user-owned scratch dir SAMPLE_DIR already uses.
VERAPDF_CACHE="$ROOT/.tmp/verapdf"
VERAPDF_DIR="${VERAPDF_CACHE}/verapdf-${VERAPDF_VERSION}"
VERAPDF_CLI="${VERAPDF_DIR}/verapdf"
VERAPDF_INSTALLER_ZIP="${VERAPDF_CACHE}/verapdf-greenfield-${VERAPDF_VERSION}-installer.zip"
# Older versions of this gate wrapped a downloaded jar in a script; such a
# wrapper around a broken jar is replaced by a fresh install.
VERAPDF_JAR="${VERAPDF_CACHE}/verapdf-app.jar"
SAMPLE_DIR="$ROOT/.tmp/gopmgr-pdfa-test"
SAMPLES=(
    schedule.pdf
    document-charter.pdf
    document-multilingual.pdf
    document-japanese.pdf
    document-korean.pdf
    document-chinese.pdf
    combined-report.pdf
    monte-carlo-risk-report.pdf
    signed-document.pdf
)

# install_verapdf_cli downloads the pinned installer, refuses it unless its
# SHA-256 matches, and installs veraPDF into $VERAPDF_DIR without prompts.
# It installs into a fresh directory first, so a failed run never leaves a
# half-installed CLI in place.
install_verapdf_cli() {
    local tmp_zip tmp_dir installer
    if ! command -v java >/dev/null 2>&1; then
        echo "veraPDF needs Java, and no java command is on PATH."
        return 1
    fi
    mkdir -p "$VERAPDF_CACHE"
    if ! file_has_sha256 "$VERAPDF_INSTALLER_ZIP" "$VERAPDF_INSTALLER_SHA256"; then
        echo "Downloading the veraPDF ${VERAPDF_VERSION} installer..."
        rm -f "$VERAPDF_INSTALLER_ZIP"
        tmp_zip="${VERAPDF_INSTALLER_ZIP}.$$"
        if command -v curl >/dev/null 2>&1; then
            curl -fsSL "$VERAPDF_INSTALLER_URL" -o "$tmp_zip" 2>/dev/null || rm -f "$tmp_zip"
        elif command -v wget >/dev/null 2>&1; then
            wget -q "$VERAPDF_INSTALLER_URL" -O "$tmp_zip" || rm -f "$tmp_zip"
        fi
        if [ ! -s "$tmp_zip" ]; then
            rm -f "$tmp_zip"
            echo "Could not download $VERAPDF_INSTALLER_URL"
            return 1
        fi
        if ! file_has_sha256 "$tmp_zip" "$VERAPDF_INSTALLER_SHA256"; then
            rm -f "$tmp_zip"
            echo "The downloaded veraPDF installer does not match its pinned SHA-256; refusing to run it."
            return 1
        fi
        mv "$tmp_zip" "$VERAPDF_INSTALLER_ZIP"
    fi
    tmp_dir="$(mktemp -d "${VERAPDF_CACHE}/install.XXXXXX")"
    installer="$tmp_dir/installer/verapdf-greenfield-${VERAPDF_VERSION}/verapdf-izpack-installer-${VERAPDF_VERSION}.jar"
    if ! unzip -q "$VERAPDF_INSTALLER_ZIP" -d "$tmp_dir/installer" || [ ! -f "$installer" ]; then
        rm -rf "$tmp_dir"
        echo "The veraPDF installer archive does not contain $(basename "$installer")."
        return 1
    fi
    verapdf_auto_install_xml "$tmp_dir/verapdf" > "$tmp_dir/auto-install.xml"
    if ! java -jar "$installer" "$tmp_dir/auto-install.xml" >/dev/null 2>&1 || [ ! -x "$tmp_dir/verapdf/verapdf" ]; then
        rm -rf "$tmp_dir"
        echo "The veraPDF installer did not produce a verapdf command."
        return 1
    fi
    rm -rf "$VERAPDF_DIR"
    mv "$tmp_dir/verapdf" "$VERAPDF_DIR"
    rm -rf "$tmp_dir"
}

echo "=== PDF/A-2u Validation Gate ==="

if [ ! -s "$ICC_PROFILE" ]; then
    pdfa_precondition_unmet "No ICC profile found at $ICC_PROFILE (run 'make icc' first); the PDF/A OutputIntent cannot be validated."
fi

# --- 1. Try Docker first (most reliable) --------------------------------
# The explicit CLI override keeps the helper regression hermetic on hosts such
# as GitHub runners that happen to provide Docker. It is also useful when
# diagnosing a local veraPDF installation without removing Docker from PATH.
if [ "${GOPMGR_VERAPDF_FORCE_CLI:-0}" != "1" ] &&
	command -v docker >/dev/null 2>&1; then
    echo "Using veraPDF via Docker ($VERAPDF_IMAGE)..."
    VERAPDF_MODE="docker"
    # Use veraPDF's official self-contained CLI image and bind it to the same
    # version as the installer fallback. A mutable `latest` tag would let
    # identical source tags produce different conformance results over time.
    VERAPDF_CMD=(docker run --rm -v "$ROOT:/work" "$VERAPDF_IMAGE")
else
	echo "Docker not found, falling back to veraPDF CLI..."
	# --- 2. Ensure veraPDF CLI is available -----------------------------
	if command -v verapdf >/dev/null 2>&1; then
		VERAPDF_CLI="$(command -v verapdf)"
	fi
	if verapdf_cli_needs_refresh "$VERAPDF_CLI" "$VERAPDF_JAR"; then
		rm -f "$VERAPDF_CLI"
	fi
	if [ ! -x "$VERAPDF_CLI" ]; then
		install_verapdf_cli || true
	fi
	if [ ! -x "$VERAPDF_CLI" ]; then
		pdfa_precondition_unmet "Could not obtain a veraPDF CLI (no Docker, none on PATH, and the pinned installer could not be downloaded, verified, or run); cannot validate PDF/A-2u."
	fi
	VERAPDF_MODE="cli"
	VERAPDF_CMD=("$VERAPDF_CLI")
fi

# --- 3. Generate representative PDFs ------------------------------------
# We use small Go one-off programs that exercise our real renderers.
# This keeps the gate honest.

echo "Generating sample PDFs for validation..."

rm -rf "$SAMPLE_DIR"
mkdir -p "$SAMPLE_DIR"

# Minimal schedule report PDF (uses the export package directly)
cat > "$SAMPLE_DIR/gen_schedule.go" << 'EOF'
package main

import (
	"fmt"
	"os"
	"gopmgr/internal/export"
	"gopmgr/internal/kernel"
)

func main() {
	tasks := map[string]*kernel.Task{
		"A": {ID: "A", Title: "Task A", Duration: 5, ES: 0, EF: 5, LS: 0, LF: 5, Float: 0, IsCritical: true},
		"B": {ID: "B", Title: "Task B", Duration: 3, ES: 5, EF: 8, LS: 5, LF: 8, Float: 0, IsCritical: true},
	}
	payload := export.ReportPayload{Tasks: tasks}
	opts := export.ExportOptions{Format: export.FormatPDF, Title: "PDF/A-2u Test Schedule"}

	data, err := export.GenerateArchivalReport(payload, opts)
	if err != nil {
		fmt.Println("ERROR generating schedule PDF:", err)
		os.Exit(1)
	}
	_ = os.WriteFile(".tmp/gopmgr-pdfa-test/schedule.pdf", data, 0o644)
	fmt.Println("Generated schedule.pdf")
}
EOF

# Representative document and combined-report PDFs using public document APIs.
cat > "$SAMPLE_DIR/gen_documents.go" << 'EOF'
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"gopmgr/internal/documents"
	"gopmgr/internal/fonts"
)

func main() {
	documents.UseFont(fonts.NewManager(""), fonts.DefaultFamily)

	charterContent := map[string]interface{}{
		"project_name":        "PDF/A Validation Project",
		"sponsor":             "Operations Steering Committee",
		"project_manager":     "Ada Lovelace",
		"charter_date":        "2026-06-06",
		"purpose":             "Validate document rendering through the PDF/A-2u release gate.",
		"objectives":          []string{"Generate a representative charter.", "Exercise tables and bullet lists."},
		"scope_in":            []string{"Document PDF export", "Combined report export"},
		"scope_out":           []string{"External Acrobat attestation"},
		"deliverables":        []string{"Validated PDF/A sample set"},
		"stakeholders":        []map[string]string{{"name": "Grace Hopper", "role": "Sponsor", "interest": "High influence and high interest"}},
		"high_level_schedule": "Validation samples are generated during the release gate.",
		"milestones":          []map[string]string{{"name": "Gate sample passes", "date": "2026-06-06"}},
		"high_level_budget":   12500.0,
		"assumptions":         []string{"Bundled IBM Plex Sans assets are available."},
		"constraints":         []string{"No database context is required for gate samples."},
		"risks":               []string{"Future renderers may regress PDF/A invariants."},
		"success_criteria":    []string{"veraPDF reports PDF/A-2u compliance."},
		"authorisation":       "Approved for validation gate coverage.",
	}
	charterJSON, err := json.Marshal(charterContent)
	if err != nil {
		fmt.Println("ERROR encoding charter content:", err)
		os.Exit(1)
	}

	documentPDF, err := documents.Render(documents.KindProjectCharterWord, string(charterJSON), "PDF/A Validation Project", "USD")
	if err != nil {
		fmt.Println("ERROR generating document PDF:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(".tmp/gopmgr-pdfa-test/document-charter.pdf", documentPDF, 0o644); err != nil {
		fmt.Println("ERROR writing document PDF:", err)
		os.Exit(1)
	}
	fmt.Println("Generated document-charter.pdf")

	// Level U requires every character to map to Unicode. This sample
	// covers the text outside Latin-1 that broke fpdf's own mapping, and
	// emoji, which the renderers replace with U+FFFD.
	const multilingual = "“Quoted” — dashes – … • € £ ≥ ≤ → ™ © ½. Ελληνικά, Русский, Tiếng Việt, Zażółć gęślą jaźń, İstanbul. Launch 🚀 approved 🎉"
	multilingualJSON, err := json.Marshal(map[string]interface{}{
		"project_name":    "Multilingual Validation 🚀",
		"sponsor":         "Ελένη Παπαδοπούλου",
		"project_manager": "Nguyễn Văn An",
		"purpose":         multilingual,
		"objectives":      []string{multilingual, "Ответственный: Иван Петров"},
		"risks":           []string{"Ünïcödé coverage must hold in every style."},
	})
	if err != nil {
		fmt.Println("ERROR encoding multilingual content:", err)
		os.Exit(1)
	}
	multilingualPDF, err := documents.Render(documents.KindProjectCharterWord, string(multilingualJSON), "Multilingual Validation 🚀", "EUR")
	if err != nil {
		fmt.Println("ERROR generating multilingual document PDF:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(".tmp/gopmgr-pdfa-test/document-multilingual.pdf", multilingualPDF, 0o644); err != nil {
		fmt.Println("ERROR writing multilingual document PDF:", err)
		os.Exit(1)
	}
	fmt.Println("Generated document-multilingual.pdf")

	// One document per bundled CJK family, set in that family.
	mgr := fonts.NewManager("")
	for _, doc := range []struct{ family, file, name, text string }{
		{"IBM Plex Sans JP", "document-japanese.pdf", "プロジェクト憲章", "このプロジェクト憲章は、範囲、予算、日程を定めます。"},
		{"IBM Plex Sans KR", "document-korean.pdf", "프로젝트 헌장", "이 프로젝트 헌장은 범위, 예산, 일정을 정합니다."},
		{"IBM Plex Sans SC", "document-chinese.pdf", "项目章程", "本项目章程确定范围、预算和进度。"},
	} {
		documents.UseFont(mgr, doc.family)
		cjkJSON, err := json.Marshal(map[string]interface{}{
			"project_name":      doc.name,
			"purpose":           doc.text,
			"objectives":        []string{doc.text},
			"high_level_budget": 1284500.0,
		})
		if err != nil {
			fmt.Println("ERROR encoding", doc.family, "content:", err)
			os.Exit(1)
		}
		cjkPDF, err := documents.Render(documents.KindProjectCharterWord, string(cjkJSON), doc.name, "USD")
		if err != nil {
			fmt.Println("ERROR generating", doc.file+":", err)
			os.Exit(1)
		}
		if err := os.WriteFile(".tmp/gopmgr-pdfa-test/"+doc.file, cjkPDF, 0o644); err != nil {
			fmt.Println("ERROR writing", doc.file+":", err)
			os.Exit(1)
		}
		fmt.Println("Generated", doc.file)
	}
	documents.UseFont(mgr, fonts.DefaultFamily)

	scopeContent := map[string]interface{}{
		"project_name":        "PDF/A Validation Project",
		"scope_description":   "Representative scope statement for the combined-report PDF/A gate sample.",
		"deliverables":        []string{"Archival document sample", "Archival combined report sample"},
		"acceptance_criteria": []string{"All generated PDFs pass veraPDF PDF/A-2u validation."},
		"exclusions":          []string{"Database-backed report assembly"},
		"constraints":         []string{"The gate must run without project fixtures."},
		"assumptions":         []string{"Local font assets remain embedded in the repository."},
	}
	scopeJSON, err := json.Marshal(scopeContent)
	if err != nil {
		fmt.Println("ERROR encoding scope content:", err)
		os.Exit(1)
	}

	spec := documents.ReportSpec{
		ReportTitle: "PDF/A Validation Combined Report",
		Subtitle:    "Release gate representative sample",
		Author:      "GoPMgr",
		ProjectName: "PDF/A Validation Project",
		Sections: []documents.ReportSection{
			{
				DocumentID:  "doc-charter",
				Title:       "Project Charter",
				Description: "Representative charter generated through the public document renderer.",
			},
			{
				DocumentID:  "doc-scope",
				Title:       "Scope Statement",
				Description: "Representative scope statement rendered inside a combined report.",
			},
		},
	}
	sections := []documents.ResolvedSection{
		{
			Section: spec.Sections[0],
			Kind:    documents.KindProjectCharterWord,
			Content: string(charterJSON),
			Version: 1,
			Status:  "approved",
		},
		{
			Section: spec.Sections[1],
			Kind:    documents.KindScopeStatement,
			Content: string(scopeJSON),
			Version: 1,
			Status:  "approved",
		},
	}

	combinedPDF, err := documents.BuildCombinedReport(spec, sections)
	if err != nil {
		fmt.Println("ERROR generating combined report PDF:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(".tmp/gopmgr-pdfa-test/combined-report.pdf", combinedPDF, 0o644); err != nil {
		fmt.Println("ERROR writing combined report PDF:", err)
		os.Exit(1)
	}
	fmt.Println("Generated combined-report.pdf")
}
EOF

# Representative Monte Carlo risk report PDF using the public export API.
cat > "$SAMPLE_DIR/gen_montecarlo.go" << 'EOF'
package main

import (
	"fmt"
	"os"

	"gopmgr/internal/export"
	"gopmgr/internal/kernel"
)

func main() {
	result := kernel.SimResult{
		Valid:      true,
		Iterations: 500,
		Workers:    4,
		P50:        12.4,
		P80:        15.2,
		P90:        18.7,
		FinishCDF: []kernel.ProbabilityPoint{
			{Day: 10, Probability: 0},
			{Day: 14, Probability: 0.5},
			{Day: 19, Probability: 1},
		},
		TornadoDrivers: []kernel.TornadoDriver{
			{
				TaskID:            "Build",
				CriticalFrequency: 0.92,
				P50Duration:       5.1,
				P80Duration:       7.3,
				P90Duration:       9.4,
				DurationSpread:    4.3,
				Score:             3.956,
			},
		},
	}

	data, err := export.GenerateMonteCarloRiskReport(export.MonteCarloRiskReportSpec{
		ProjectName: "PDF/A Validation Project",
		ChartTitle:  "Monte Carlo Validation Schedule",
		Result:      result,
	})
	if err != nil {
		fmt.Println("ERROR generating Monte Carlo risk report PDF:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(".tmp/gopmgr-pdfa-test/monte-carlo-risk-report.pdf", data, 0o644); err != nil {
		fmt.Println("ERROR writing Monte Carlo risk report PDF:", err)
		os.Exit(1)
	}
	fmt.Println("Generated monte-carlo-risk-report.pdf")
}
EOF

# A PAdES-signed document. Signing appends the final incremental update, so
# this sample proves the signed file is still PDF/A-2u. A throwaway
# self-signed certificate is enough: trust is the PAdES gates' concern.
cat > "$SAMPLE_DIR/gen_signed.go" << 'EOF'
package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"time"

	pmcrypto "gopmgr/internal/crypto"
	"gopmgr/internal/documents"
	"gopmgr/internal/fonts"
	"gopmgr/internal/signing"
)

func main() {
	documents.UseFont(fonts.NewManager(""), fonts.DefaultFamily)
	content, err := json.Marshal(map[string]interface{}{
		"project_name":  "Signed Validation Project",
		"sponsor":       "Operations Steering Committee",
		"purpose":       "Prove that a PAdES signature keeps the document PDF/A-2u.",
		"objectives":    []string{"Sign the document as its final change."},
		"authorisation": "Approved and signed.",
	})
	if err != nil {
		fail("encode content", err)
	}
	pdf, err := documents.Render(documents.KindProjectCharterWord, string(content), "Signed Validation Project", "USD")
	if err != nil {
		fail("render document", err)
	}

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		fail("generate key", err)
	}
	now := time.Now().UTC()
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(now.UnixNano()),
		Subject:               pkix.Name{CommonName: "GoPMgr PDF/A Gate Signer"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		fail("create certificate", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		fail("parse certificate", err)
	}

	signed, _, err := signing.ApplyPAdES(context.Background(), pdf, &pmcrypto.Signer{Cert: cert, PrivateKey: key}, nil)
	if err != nil {
		fail("sign document", err)
	}
	if err := os.WriteFile(".tmp/gopmgr-pdfa-test/signed-document.pdf", signed, 0o644); err != nil {
		fail("write signed document", err)
	}
	fmt.Println("Generated signed-document.pdf")
}

func fail(step string, err error) {
	fmt.Println("ERROR:", step+":", err)
	os.Exit(1)
}
EOF

# Compile and run the generators.
if go run "$SAMPLE_DIR/gen_schedule.go" 2>/dev/null; then
    echo "Sample schedule report generated."
else
    echo "Could not generate sample PDF (missing dependencies or build issue)."
    echo "PDF/A validation cannot run without representative samples."
    exit 1
fi
if go run "$SAMPLE_DIR/gen_documents.go" 2>/dev/null; then
    echo "Sample document and combined report generated."
else
    echo "Could not generate document PDF samples (missing dependencies or build issue)."
    echo "PDF/A validation cannot run without representative document samples."
    exit 1
fi
if go run "$SAMPLE_DIR/gen_montecarlo.go" 2>/dev/null; then
    echo "Sample Monte Carlo risk report generated."
else
    echo "Could not generate Monte Carlo risk report PDF sample (missing dependencies or build issue)."
    echo "PDF/A validation cannot run without the Monte Carlo risk report sample."
    exit 1
fi

if go run "$SAMPLE_DIR/gen_signed.go" 2>/dev/null; then
    echo "Sample signed document generated."
else
    echo "Could not generate the signed document PDF sample (missing dependencies or build issue)."
    echo "PDF/A validation cannot run without the signed document sample."
    exit 1
fi

# --- 4. Run veraPDF ------------------------------------------------------
for name in "${SAMPLES[@]}"; do
    if [ ! -s "$SAMPLE_DIR/$name" ]; then
        echo "FAIL: sample $name was not generated, although its generator succeeded."
        exit 1
    fi
done

echo "Validating PDFs with veraPDF..."

FAIL=0
for name in "${SAMPLES[@]}"; do
    pdf="$SAMPLE_DIR/$name"
    echo "  Checking $name ..."
    sample_arg="$(verapdf_sample_arg "$VERAPDF_MODE" "$ROOT" "$pdf")"
	output="$("${VERAPDF_CMD[@]}" --format xml -f 2u "$sample_arg" 2>&1 || true)"
    if printf '%s\n' "$output" | verapdf_output_is_compliant; then
        echo "    OK - $name is PDF/A-2u compliant"
    else
        echo "    FAIL - $name did not pass PDF/A-2u validation"
        printf '%s\n' "$output"
        FAIL=1
    fi
done

if [ $FAIL -eq 1 ]; then
    echo "PDF/A validation gate FAILED."
    exit 1
fi

echo "PDF/A-2u validation gate PASSED."
exit 0
