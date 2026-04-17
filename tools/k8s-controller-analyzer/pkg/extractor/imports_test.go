package extractor

import (
	"testing"

	"golang.org/x/tools/go/packages"
)

func TestExtractImports_VendorLoggingAndMetrics(t *testing.T) {
	repoRoot := t.TempDir()

	controllerPkg := newSyntheticPackage(
		t,
		repoRoot,
		"example.com/project/controllers",
		map[string]string{
			"controllers/main.go": `package controllers

import (
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/prometheus/client_golang/prometheus"
)

func run() {
	fmt.Printf("hello")
	_ = s3.Client{}
	_ = prometheus.Labels{}
}
`,
		},
	)

	facts := ExtractImports([]*packages.Package{controllerPkg}, repoRoot)
	if len(facts) != 1 {
		t.Fatalf("expected 1 import_analysis fact, got %d", len(facts))
	}

	if facts[0].Kind != KindImportAnalysis {
		t.Fatalf("expected kind=%s, got %s", KindImportAnalysis, facts[0].Kind)
	}

	rules := decodeRulesForTest(t, facts[0])
	hasRule := func(rule string) bool {
		for _, r := range rules {
			if r == rule {
				return true
			}
		}
		return false
	}
	if !hasRule(RuleVendorIsolation) || !hasRule(RuleStructuredLogging) || !hasRule(RuleMetricsCoverage) {
		t.Fatalf("unexpected rules: %v", rules)
	}
	if hasRule(RuleLibraryImports) {
		t.Fatalf("did not expect %q rule for non-helm/kustomize imports: %v", RuleLibraryImports, rules)
	}

	data, ok := facts[0].Data.(ImportData)
	if !ok {
		t.Fatalf("expected ImportData, got %T", facts[0].Data)
	}
	if len(data.VendorImports) != 1 || data.VendorImports[0].Vendor != "aws" {
		t.Fatalf("expected aws vendor import, got %+v", data.VendorImports)
	}
	if len(data.UnstructuredLogging) != 1 || data.UnstructuredLogging[0].Call != "fmt.Printf" {
		t.Fatalf("expected fmt.Printf logging call, got %+v", data.UnstructuredLogging)
	}
	if !data.HasMetrics {
		t.Fatalf("expected has_metrics=true, got %+v", data)
	}
	if len(data.LibraryImports) != 0 {
		t.Fatalf("expected no library imports, got %+v", data.LibraryImports)
	}
}

func TestExtractImports_LibraryImportsHelmAndKustomize(t *testing.T) {
	repoRoot := t.TempDir()

	controllerPkg := newSyntheticPackage(
		t,
		repoRoot,
		"example.com/project/controllers",
		map[string]string{
			"controllers/main.go": `package controllers

import (
	"helm.sh/helm/v3/pkg/action"
	"sigs.k8s.io/kustomize/api/krusty"
	"k8s.io/apimachinery/pkg/runtime"
)

func run() {
	_ = action.Configuration{}
	_ = krusty.Options{}
	_ = runtime.Scheme{}
}
`,
		},
	)

	facts := ExtractImports([]*packages.Package{controllerPkg}, repoRoot)
	if len(facts) != 1 {
		t.Fatalf("expected 1 import_analysis fact, got %d", len(facts))
	}
	if facts[0].Kind != KindImportAnalysis {
		t.Fatalf("expected kind=%s, got %s", KindImportAnalysis, facts[0].Kind)
	}

	rules := decodeRulesForTest(t, facts[0])
	hasRule := func(rule string) bool {
		for _, r := range rules {
			if r == rule {
				return true
			}
		}
		return false
	}
	if !hasRule(RuleLibraryImports) {
		t.Fatalf("expected %q rule, got %v", RuleLibraryImports, rules)
	}
	if hasRule(RuleVendorIsolation) {
		t.Fatalf("did not expect %q rule for helm/kustomize imports, got %v", RuleVendorIsolation, rules)
	}

	data, ok := facts[0].Data.(ImportData)
	if !ok {
		t.Fatalf("expected ImportData, got %T", facts[0].Data)
	}
	if len(data.LibraryImports) != 2 {
		t.Fatalf("expected 2 library imports, got %+v", data.LibraryImports)
	}

	families := map[string]bool{}
	for _, li := range data.LibraryImports {
		families[li.Family] = true
	}
	if !families["helm"] || !families["kustomize"] {
		t.Fatalf("expected helm and kustomize families, got %+v", data.LibraryImports)
	}
}

func TestExtractImports_UnrelatedImportsDoNotEmitLibrarySignal(t *testing.T) {
	repoRoot := t.TempDir()

	controllerPkg := newSyntheticPackage(
		t,
		repoRoot,
		"example.com/project/controllers",
		map[string]string{
			"controllers/main.go": `package controllers

import (
	"fmt"
	"k8s.io/apimachinery/pkg/runtime"
)

func run() {
	fmt.Println("hello")
	_ = runtime.Scheme{}
}
`,
		},
	)

	facts := ExtractImports([]*packages.Package{controllerPkg}, repoRoot)
	if len(facts) != 1 {
		t.Fatalf("expected 1 import_analysis fact (for logging), got %d", len(facts))
	}

	rules := decodeRulesForTest(t, facts[0])
	for _, r := range rules {
		if r == RuleLibraryImports {
			t.Fatalf("did not expect %q rule for unrelated imports: %v", RuleLibraryImports, rules)
		}
	}

	data, ok := facts[0].Data.(ImportData)
	if !ok {
		t.Fatalf("expected ImportData, got %T", facts[0].Data)
	}
	if len(data.LibraryImports) != 0 {
		t.Fatalf("expected no library imports, got %+v", data.LibraryImports)
	}
}
