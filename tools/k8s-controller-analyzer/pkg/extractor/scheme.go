package extractor

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

// ExtractSchemeRegistrations finds scheme.AddToScheme and utilruntime.Must calls.
func ExtractSchemeRegistrations(
	pkgs []*packages.Package,
	repoPath string,
) []Fact {
	var facts []Fact

	for _, pkg := range pkgs {
		for i, file := range pkg.Syntax {
			filePath := pkg.CompiledGoFiles[i]
			relPath, _ := filepath.Rel(repoPath, filePath)

			// Look in main.go or cmd/ files
			if !isEntryPoint(relPath) {
				continue
			}

			facts = append(facts, extractSchemeFromFile(file, pkg.Fset, relPath)...)
		}
	}

	return facts
}

func isEntryPoint(relPath string) bool {
	base := filepath.Base(relPath)
	if base == "main.go" {
		return true
	}
	if strings.Contains(relPath, "cmd/") {
		return true
	}

	return false
}

func extractSchemeFromFile(
	file *ast.File,
	fset *token.FileSet,
	relPath string,
) []Fact {
	var facts []Fact

	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// utilruntime.Must(xxxv1alpha1.AddToScheme(scheme))
		// or direct: _ = xxxv1alpha1.AddToScheme(scheme)
		funcName := callFuncName(call)

		if funcName == "utilruntime.Must" || funcName == "runtime.Must" {
			// Inner call
			if len(call.Args) == 1 {
				if inner, ok := call.Args[0].(*ast.CallExpr); ok {
					if reg := extractAddToScheme(inner); reg != nil {
						facts = append(facts, NewFact(
							RuleSchemeReg,
							KindSchemeRegistration,
							relPath,
							fset.Position(call.Pos()).Line,
							*reg,
						))
					}
				}
			}
			return false // don't walk into children, we already processed the inner call
		}

		if reg := extractAddToScheme(call); reg != nil {
			facts = append(facts, NewFact(
				RuleSchemeReg,
				KindSchemeRegistration,
				relPath,
				fset.Position(call.Pos()).Line,
				*reg,
			))
		}

		return true
	})

	return facts
}

func extractAddToScheme(call *ast.CallExpr) *SchemeRegistrationData {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "AddToScheme" {
		return nil
	}

	pkgIdent, ok := sel.X.(*ast.Ident)
	if !ok {
		return nil
	}

	return &SchemeRegistrationData{
		Package: pkgIdent.Name,
		Call:    "AddToScheme",
	}
}
