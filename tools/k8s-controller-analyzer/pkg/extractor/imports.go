package extractor

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

var vendorPrefixes = map[string]string{
	"github.com/openshift":            "openshift",
	"github.com/aws":                  "aws",
	"github.com/Azure":                "azure",
	"cloud.google.com":                "gcp",
	"github.com/oracle":               "oracle",
	"github.com/IBM":                  "ibm",
	"github.com/vmware":               "vmware",
	"github.com/nutanix":              "nutanix",
	"github.com/digitalocean":         "digitalocean",
	"github.com/linode":               "linode",
	"github.com/openshift/api":        "openshift",
	"github.com/openshift/client-go":  "openshift",
	"github.com/openshift/library-go": "openshift",
}

var libraryImportPrefixes = map[string]string{
	"helm.sh/helm/":          "helm",
	"github.com/helm/helm/":  "helm", // legacy Helm import path
	"sigs.k8s.io/kustomize/": "kustomize",
}

var unstructuredLogPatterns = map[string]bool{
	"fmt.Printf":  true,
	"fmt.Println": true,
	"fmt.Fprintf": true,
	"log.Printf":  true,
	"log.Println": true,
	"log.Print":   true,
	"log.Fatalf":  true,
	"log.Fatal":   true,
}

// ExtractImports scans all non-test Go files for vendor imports,
// unstructured logging, and metrics usage.
func ExtractImports(
	pkgs []*packages.Package,
	repoPath string,
) []Fact {
	var facts []Fact

	for _, pkg := range pkgs {
		for i, file := range pkg.Syntax {
			filePath := pkg.CompiledGoFiles[i]
			relPath, _ := filepath.Rel(repoPath, filePath)

			// Skip test files and generated files
			if strings.HasSuffix(relPath, "_test.go") || strings.Contains(relPath, "zz_generated") {
				continue
			}

			data := extractImportData(file, pkg.Fset, relPath)
			if data == nil {
				continue
			}

			rules := []string{}
			if len(data.VendorImports) > 0 {
				rules = append(rules, RuleVendorIsolation)
			}
			if len(data.LibraryImports) > 0 {
				rules = append(rules, RuleLibraryImports)
			}
			if len(data.UnstructuredLogging) > 0 {
				rules = append(rules, RuleStructuredLogging)
			}
			if data.HasMetrics {
				rules = append(rules, RuleMetricsCoverage)
			}

			if len(rules) == 0 {
				continue
			}

			facts = append(facts, NewMultiRuleFact(
				rules,
				KindImportAnalysis,
				relPath,
				1,
				*data,
			))
		}
	}

	return facts
}

func extractImportData(
	file *ast.File,
	fset *token.FileSet,
	relPath string,
) *ImportData {
	data := &ImportData{}
	hasContent := false

	// Scan imports for vendor-specific packages and metrics
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)

		// Vendor imports
		for prefix, vendor := range vendorPrefixes {
			if strings.HasPrefix(path, prefix) {
				data.VendorImports = append(data.VendorImports, VendorImport{
					Path:   path,
					Vendor: vendor,
					Line:   fset.Position(imp.Pos()).Line,
				})
				hasContent = true
				break
			}
		}

		// Helm/Kustomize library imports
		for prefix, family := range libraryImportPrefixes {
			if strings.HasPrefix(path, prefix) {
				data.LibraryImports = append(data.LibraryImports, LibraryImport{
					Family: family,
					Path:   path,
					Line:   fset.Position(imp.Pos()).Line,
				})
				hasContent = true
				break
			}
		}

		// Metrics
		if strings.Contains(path, "prometheus") || strings.Contains(path, "promclient") {
			data.HasMetrics = true
			data.MetricsPackage = path
			hasContent = true
		}
	}

	// Scan for unstructured logging calls
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		funcName := callFuncName(call)
		if unstructuredLogPatterns[funcName] {
			data.UnstructuredLogging = append(data.UnstructuredLogging, LoggingCall{
				Call: funcName,
				Line: fset.Position(call.Pos()).Line,
			})
			hasContent = true
		}

		return true
	})

	if !hasContent {
		return nil
	}

	return data
}
