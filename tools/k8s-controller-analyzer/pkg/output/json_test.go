package output

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/lburgazzoli/claude-plugins/tools/k8s-controller-analyzer/pkg/extractor"
)

func TestWriteJSONSingleRuleUsesArrayAndV3Schema(t *testing.T) {
	var buf bytes.Buffer

	facts := []extractor.Fact{
		extractor.NewFact(
			extractor.RuleSchemeReg,
			extractor.KindSchemeRegistration,
			"main.go",
			52,
			extractor.SchemeRegistrationData{
				Package: "example.com/api/v1alpha1",
				Call:    "AddToScheme",
			},
		),
	}

	if err := WriteJSON(&buf, "/repo", facts, nil); err != nil {
		t.Fatalf("WriteJSON returned error: %v", err)
	}

	var report map[string]any
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}

	if got := report["schema_version"]; got != "v3" {
		t.Fatalf("expected schema_version=v3, got %v", got)
	}

	serializedFacts := report["facts"].([]any)
	first := serializedFacts[0].(map[string]any)
	rules := first["rules"].([]any)
	if len(rules) != 1 || rules[0] != extractor.RuleSchemeReg {
		t.Fatalf("expected single rule array, got %v", rules)
	}
	if _, ok := first["rule"]; ok {
		t.Fatalf("did not expect legacy rule field in output: %v", first)
	}
}

func TestWriteJSONPreservesMultiRuleOrderAndNestedReconciles(t *testing.T) {
	var buf bytes.Buffer

	facts := []extractor.Fact{
		extractor.NewMultiRuleFact(
			[]string{extractor.RuleRBACCoverage, extractor.RuleRequeueSafety},
			extractor.KindController,
			"controllers/foo_controller.go",
			42,
			extractor.ControllerData{
				Name: "FooReconciler",
				Reconciles: extractor.ReconcilesTarget{
					Group:   "example.com",
					Version: "v1alpha1",
					Kind:    "Foo",
				},
			},
		),
	}

	if err := WriteJSON(&buf, "/repo", facts, nil); err != nil {
		t.Fatalf("WriteJSON returned error: %v", err)
	}

	var report map[string]any
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}

	serializedFacts := report["facts"].([]any)
	first := serializedFacts[0].(map[string]any)
	rules := first["rules"].([]any)
	if len(rules) != 2 {
		t.Fatalf("expected two rules, got %v", rules)
	}
	if rules[0] != extractor.RuleRBACCoverage || rules[1] != extractor.RuleRequeueSafety {
		t.Fatalf("expected deterministic rule order, got %v", rules)
	}

	data := first["data"].(map[string]any)
	reconciles := data["reconciles"].(map[string]any)
	if reconciles["group"] != "example.com" || reconciles["version"] != "v1alpha1" || reconciles["kind"] != "Foo" {
		t.Fatalf("unexpected reconciles payload: %v", reconciles)
	}
	if _, ok := data["reconciles_kind"]; ok {
		t.Fatalf("did not expect legacy reconciles_kind field: %v", data)
	}
	if _, ok := data["reconciles_group"]; ok {
		t.Fatalf("did not expect legacy reconciles_group field: %v", data)
	}
	if _, ok := data["reconciles_version"]; ok {
		t.Fatalf("did not expect legacy reconciles_version field: %v", data)
	}
}
