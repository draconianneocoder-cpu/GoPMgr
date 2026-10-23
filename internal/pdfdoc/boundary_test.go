// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package pdfdoc

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// rendererPackages draw GoPMgr's PDFs. Their text must go through *PDF.
var rendererPackages = []string{"../documents", "../export", "../charts/pdfrender"}

// textMethods are the fpdf methods PDF overrides.
var textMethods = map[string]bool{
	"Cell": true, "Cellf": true, "CellFormat": true, "MultiCell": true, "Text": true,
	"ClipText": true, "Write": true, "Writef": true, "WriteAligned": true,
	"WriteLinkString": true, "WriteLinkID": true, "SubWrite": true,
	"GetStringWidth": true, "GetStringSymbolWidth": true, "SplitText": true, "SplitLines": true,
}

// TestRenderersDrawTextThroughPDF fails when renderer code could reach
// fpdf's text methods without SafeText: by creating a bare fpdf document,
// by passing one around, or by calling a text method on the embedded
// *fpdf.Fpdf directly.
func TestRenderersDrawTextThroughPDF(t *testing.T) {
	fset := token.NewFileSet()
	checked := 0
	for _, dir := range rendererPackages {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			name := e.Name()
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			path := filepath.Join(dir, name)
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			checked++
			checkRendererFile(t, fset, file)
		}
	}
	if checked < 30 {
		t.Fatalf("checked only %d renderer files; did a package move?", checked)
	}
}

func checkRendererFile(t *testing.T, fset *token.FileSet, file *ast.File) {
	t.Helper()
	var enclosing string
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncDecl:
			enclosing = n.Name.Name
		case *ast.CallExpr:
			if sel, ok := n.Fun.(*ast.SelectorExpr); ok {
				if isIdent(sel.X, "fpdf") && (sel.Sel.Name == "New" || sel.Sel.Name == "NewCustom") {
					t.Errorf("%s: fpdf.%s creates a document without text safety; use pdfdoc.New", fset.Position(n.Pos()), sel.Sel.Name)
				}
				if inner, ok := sel.X.(*ast.SelectorExpr); ok && inner.Sel.Name == "Fpdf" && textMethods[sel.Sel.Name] {
					t.Errorf("%s: .Fpdf.%s bypasses pdfdoc.SafeText; call %s on the *pdfdoc.PDF", fset.Position(n.Pos()), sel.Sel.Name, sel.Sel.Name)
				}
			}
		case *ast.StarExpr:
			if sel, ok := n.X.(*ast.SelectorExpr); ok && isIdent(sel.X, "fpdf") && sel.Sel.Name == "Fpdf" {
				t.Errorf("%s: %s uses *fpdf.Fpdf; renderers must take *pdfdoc.PDF", fset.Position(n.Pos()), describe(enclosing))
			}
		}
		return true
	})
}

func isIdent(e ast.Expr, name string) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == name
}

func describe(fn string) string {
	if fn == "" {
		return "a package-level declaration"
	}
	return fn
}
