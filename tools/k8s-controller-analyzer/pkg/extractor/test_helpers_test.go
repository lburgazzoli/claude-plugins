package extractor

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"testing"

	"golang.org/x/tools/go/packages"
)

func newSyntheticPackage(
	t *testing.T,
	repoRoot string,
	pkgPath string,
	files map[string]string,
) *packages.Package {
	t.Helper()

	fset := token.NewFileSet()
	relPaths := make([]string, 0, len(files))
	for relPath := range files {
		relPaths = append(relPaths, relPath)
	}
	sort.Strings(relPaths)

	syntax := make([]*ast.File, 0, len(relPaths))
	compiled := make([]string, 0, len(relPaths))

	for _, relPath := range relPaths {
		absPath := filepath.Join(repoRoot, relPath)
		file, err := parser.ParseFile(fset, absPath, files[relPath], parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", relPath, err)
		}
		syntax = append(syntax, file)
		compiled = append(compiled, absPath)
	}

	return &packages.Package{
		Fset:            fset,
		PkgPath:         pkgPath,
		Syntax:          syntax,
		CompiledGoFiles: compiled,
	}
}

func decodeRulesForTest(t *testing.T, fact Fact) []string {
	t.Helper()

	if fact.Rules == nil {
		t.Fatal("expected rules to be populated")
	}

	return append([]string(nil), fact.Rules...)
}
