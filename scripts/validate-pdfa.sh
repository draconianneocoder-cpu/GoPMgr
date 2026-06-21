#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# PDF/A-3 validation gate using veraPDF.
#
# This gate ensures that the PDFs we claim are PDF/A-3 really are.
# It generates representative samples from the live renderers and validates
# each with veraPDF (3b profile). Exit 1 on any non-compliant PDF.
#
# Hard gate. By default (PMFORGE_PDFA_STRICT=1) a missing validator, a missing
# ICC profile, or a missing sample set is a FAILURE, not a silent skip: the
# release gate must never certify PDF/A-3 conformance it could not actually
# check. Run with PMFORGE_PDFA_STRICT=0 for local convenience on machines
# without Docker/veraPDF, where those preconditions degrade to a warned skip.
# `scripts/check-release.sh` always invokes this script strict.
#
# Requirements (strict mode):
#   - Docker (preferred) or a veraPDF CLI on PATH (Java-backed)
#   - `make icc` has been run at least once (sRGB.icc present)
#
# Usage:
#   make check-pdfa                       # strict by default
#   PMFORGE_PDFA_STRICT=0 make check-pdfa # degrade missing tooling to a skip

set -eu
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

. "$ROOT/scripts/validate-pdfa-lib.sh"

# Strictness. When strict (the default, and always under the release gate),
# unmet preconditions fail the gate. When non-strict, they degrade to a warned
# skip so developers without Docker/veraPDF are not blocked locally.
PDFA_STRICT="${PMFORGE_PDFA_STRICT:-1}"

# pdfa_precondition_unmet <human-readable reason>
# Fails under strict mode, skips (exit 0) otherwise.
pdfa_precondition_unmet() {
    if [ "$PDFA_STRICT" = "1" ]; then
        echo "FAIL: $1"
        echo "The PDF/A-3 gate is strict: install Docker or a veraPDF CLI and run 'make icc'."
        echo "Set PMFORGE_PDFA_STRICT=0 to downgrade this to a local skip."
        exit 1
    fi
    echo "SKIP: $1"
    echo "(non-strict) Set PMFORGE_PDFA_STRICT=1 to make this a hard failure; the release gate does."
    exit 0
}

ICC_PROFILE="${PMFORGE_ICC_PROFILE:-internal/pdfmeta/assets/sRGB.icc}"
VERAPDF_VERSION="1.28.1"
VERAPDF_DIR="/tmp/verapdf-${VERAPDF_VERSION}"
VERAPDF_CLI="${VERAPDF_DIR}/verapdf"
VERAPDF_ZIP="/tmp/verapdf.zip"
VERAPDF_JAR="/tmp/verapdf-app.jar"
SAMPLE_DIR="$ROOT/.tmp/pmforge-pdfa-test"

echo "=== PDF/A-3 Validation Gate ==="

if [ ! -s "$ICC_PROFILE" ]; then
    pdfa_precondition_unmet "No ICC profile found at $ICC_PROFILE (run 'make icc' first); PDF/A-3 OutputIntent cannot be validated."
fi

# --- 1. Try Docker first (most reliable) --------------------------------
if command -v docker >/dev/null 2>&1; then
    echo "Using veraPDF via Docker..."
    VERAPDF_MODE="docker"
    VERAPDF_CMD=(docker run --rm -v "$ROOT:/work" verapdf/verapdf:latest)
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
        echo "Downloading veraPDF CLI ${VERAPDF_VERSION}..."
        mkdir -p "$VERAPDF_DIR"
        rm -f "$VERAPDF_ZIP" "$VERAPDF_JAR"

        # Try to download the CLI zip from GitHub releases
        VERAPDF_ZIP_URL="https://github.com/veraPDF/veraPDF-apps/releases/download/v${VERAPDF_VERSION}/verapdf-${VERAPDF_VERSION}.zip"
        if command -v curl >/dev/null 2>&1; then
            tmp_zip="${VERAPDF_ZIP}.$$"
            if curl -fsSL "$VERAPDF_ZIP_URL" -o "$tmp_zip" 2>/dev/null && zip_archive_is_valid "$tmp_zip"; then
                mv "$tmp_zip" "$VERAPDF_ZIP"
            else
                rm -f "$tmp_zip"
            fi
        fi
        if [ ! -f "$VERAPDF_ZIP" ] && command -v wget >/dev/null 2>&1; then
            tmp_zip="${VERAPDF_ZIP}.$$"
            if wget -q "$VERAPDF_ZIP_URL" -O "$tmp_zip" && zip_archive_is_valid "$tmp_zip"; then
                mv "$tmp_zip" "$VERAPDF_ZIP"
            else
                rm -f "$tmp_zip"
            fi
        fi

        # If that fails, try the JAR from Maven Central as a fallback
        if [ ! -f "$VERAPDF_ZIP" ]; then
            echo "GitHub download failed, trying Maven Central..."
            if command -v curl >/dev/null 2>&1; then
                tmp_jar="${VERAPDF_JAR}.$$"
                if curl -fsSL "https://repo1.maven.org/maven2/org/verapdf/veraPDF-apps/${VERAPDF_VERSION}/veraPDF-apps-${VERAPDF_VERSION}-jar-with-dependencies.jar" -o "$tmp_jar" 2>/dev/null && zip_archive_is_valid "$tmp_jar"; then
                    mv "$tmp_jar" "$VERAPDF_JAR"
                else
                    rm -f "$tmp_jar"
                fi
            fi
            if [ ! -f "$VERAPDF_JAR" ] && command -v wget >/dev/null 2>&1; then
                tmp_jar="${VERAPDF_JAR}.$$"
                if wget -q "https://repo1.maven.org/maven2/org/verapdf/veraPDF-apps/${VERAPDF_VERSION}/veraPDF-apps-${VERAPDF_VERSION}-jar-with-dependencies.jar" -O "$tmp_jar" && zip_archive_is_valid "$tmp_jar"; then
                    mv "$tmp_jar" "$VERAPDF_JAR"
                else
                    rm -f "$tmp_jar"
                fi
            fi
        fi

        # Extract the zip if we have it
        if [ -f "$VERAPDF_ZIP" ]; then
            unzip -q "$VERAPDF_ZIP" -d "$VERAPDF_DIR" 2>/dev/null || true
            # Look for the CLI in the extracted directory
            if [ -f "$VERAPDF_DIR/verapdf-${VERAPDF_VERSION}/verapdf" ]; then
                VERAPDF_CLI="$VERAPDF_DIR/verapdf-${VERAPDF_VERSION}/verapdf"
            elif [ -f "$VERAPDF_DIR/verapdf" ]; then
                # Already in the right place
                :
            else
                # Search for verapdf executable. Use our helper instead of
                # GNU find's -executable so this works on macOS/BSD find.
                FOUND_VERAPDF="$(find_verapdf_executable "$VERAPDF_DIR" || true)"
                if [ -n "$FOUND_VERAPDF" ]; then
                    VERAPDF_CLI="$FOUND_VERAPDF"
                fi
            fi
        fi

        # If we have a JAR file, create a wrapper script
        if [ -f "$VERAPDF_JAR" ] && [ ! -x "$VERAPDF_CLI" ]; then
            mkdir -p "$(dirname "$VERAPDF_CLI")"
            cat > "$VERAPDF_CLI" << 'EOF'
#!/bin/bash
java -jar "/tmp/verapdf-app.jar" "$@"
EOF
            chmod +x "$VERAPDF_CLI"
        fi
    fi

    if [ ! -x "$VERAPDF_CLI" ]; then
        pdfa_precondition_unmet "Could not obtain a veraPDF CLI (no Docker, none on PATH, and auto-install failed); cannot validate PDF/A-3."
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
	"pmforge/internal/export"
	"pmforge/internal/kernel"
)

func main() {
	tasks := map[string]*kernel.Task{
		"A": {ID: "A", Title: "Task A", Duration: 5, ES: 0, EF: 5, LS: 0, LF: 5, Float: 0, IsCritical: true},
		"B": {ID: "B", Title: "Task B", Duration: 3, ES: 5, EF: 8, LS: 5, LF: 8, Float: 0, IsCritical: true},
	}
	payload := export.ReportPayload{Tasks: tasks}
	opts := export.ExportOptions{Format: export.FormatPDF, Title: "PDF/A-3 Test Schedule"}

	data, err := export.GenerateArchivalReport(payload, opts)
	if err != nil {
		fmt.Println("ERROR generating schedule PDF:", err)
		os.Exit(1)
	}
	_ = os.WriteFile(".tmp/pmforge-pdfa-test/schedule.pdf", data, 0o644)
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

	"pmforge/internal/documents"
	"pmforge/internal/fonts"
)

func main() {
	documents.UseFont(fonts.NewManager(""), "Source Sans 3")

	charterContent := map[string]interface{}{
		"project_name":        "PDF/A Validation Project",
		"sponsor":             "Operations Steering Committee",
		"project_manager":     "Ada Lovelace",
		"charter_date":        "2026-06-06",
		"purpose":             "Validate document rendering through the PDF/A-3b release gate.",
		"objectives":          []string{"Generate a representative charter.", "Exercise tables and bullet lists."},
		"scope_in":            []string{"Document PDF export", "Combined report export"},
		"scope_out":           []string{"External Acrobat attestation"},
		"deliverables":        []string{"Validated PDF/A sample set"},
		"stakeholders":        []map[string]string{{"name": "Grace Hopper", "role": "Sponsor", "interest": "High influence and high interest"}},
		"high_level_schedule": "Validation samples are generated during the release gate.",
		"milestones":          []map[string]string{{"name": "Gate sample passes", "date": "2026-06-06"}},
		"high_level_budget":   12500.0,
		"assumptions":         []string{"Bundled Source Sans 3 assets are available."},
		"constraints":         []string{"No database context is required for gate samples."},
		"risks":               []string{"Future renderers may regress PDF/A invariants."},
		"success_criteria":    []string{"veraPDF reports PDF/A-3b compliance."},
		"authorisation":       "Approved for validation gate coverage.",
	}
	charterJSON, err := json.Marshal(charterContent)
	if err != nil {
		fmt.Println("ERROR encoding charter content:", err)
		os.Exit(1)
	}

	documentPDF, err := documents.Render(documents.KindProjectCharterWord, string(charterJSON), "PDF/A Validation Project")
	if err != nil {
		fmt.Println("ERROR generating document PDF:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(".tmp/pmforge-pdfa-test/document-charter.pdf", documentPDF, 0o644); err != nil {
		fmt.Println("ERROR writing document PDF:", err)
		os.Exit(1)
	}
	fmt.Println("Generated document-charter.pdf")

	scopeContent := map[string]interface{}{
		"project_name":        "PDF/A Validation Project",
		"scope_description":   "Representative scope statement for the combined-report PDF/A gate sample.",
		"deliverables":        []string{"Archival document sample", "Archival combined report sample"},
		"acceptance_criteria": []string{"All generated PDFs pass veraPDF PDF/A-3b validation."},
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
		Author:      "PMForge",
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
	if err := os.WriteFile(".tmp/pmforge-pdfa-test/combined-report.pdf", combinedPDF, 0o644); err != nil {
		fmt.Println("ERROR writing combined report PDF:", err)
		os.Exit(1)
	}
	fmt.Println("Generated combined-report.pdf")
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

# --- 4. Run veraPDF ------------------------------------------------------
SAMPLES=()
while IFS= read -r -d '' pdf; do
    SAMPLES+=("$pdf")
    if [ "${#SAMPLES[@]}" -ge 5 ]; then
        break
    fi
done < <(find "$SAMPLE_DIR" -name "*.pdf" -print0 2>/dev/null)

if [ "${#SAMPLES[@]}" -eq 0 ]; then
    pdfa_precondition_unmet "No sample PDFs were found to validate after generation."
fi

echo "Validating PDFs with veraPDF..."

FAIL=0
for pdf in "${SAMPLES[@]}"; do
    echo "  Checking $(basename "$pdf") ..."
    sample_arg="$(verapdf_sample_arg "$VERAPDF_MODE" "$ROOT" "$pdf")"
	output="$("${VERAPDF_CMD[@]}" --format xml -f 3b "$sample_arg" 2>&1 || true)"
    if printf '%s\n' "$output" | verapdf_output_is_compliant; then
        echo "    OK - $(basename "$pdf") is PDF/A compliant"
    else
        echo "    FAIL - $(basename "$pdf") did not pass PDF/A validation"
        printf '%s\n' "$output"
        FAIL=1
    fi
done

if [ $FAIL -eq 1 ]; then
    echo "PDF/A validation gate FAILED."
    exit 1
fi

echo "PDF/A-3 validation gate PASSED."
exit 0
