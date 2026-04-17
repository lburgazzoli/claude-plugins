package cli

import (
	"testing"

	"github.com/lburgazzoli/claude-plugins/tools/k8s-controller-analyzer/pkg/extractor"
)

func TestFilterByRulesMatchesTypedRules(t *testing.T) {
	facts := []extractor.Fact{
		extractor.NewFact(
			extractor.RuleSchemeReg,
			extractor.KindSchemeRegistration,
			"main.go",
			10,
			extractor.SchemeRegistrationData{Package: "example.com/api/v1", Call: "AddToScheme"},
		),
		extractor.NewMultiRuleFact(
			[]string{extractor.RuleRBACCoverage, extractor.RuleRequeueSafety},
			extractor.KindController,
			"controllers/foo_controller.go",
			42,
			extractor.ControllerData{Name: "FooReconciler"},
		),
	}

	filtered := filterByRules(facts, map[string]bool{
		extractor.RuleRequeueSafety: true,
	})

	if len(filtered) != 1 {
		t.Fatalf("expected 1 filtered fact, got %d", len(filtered))
	}
	if filtered[0].Kind != extractor.KindController {
		t.Fatalf("expected controller fact, got %s", filtered[0].Kind)
	}
	if len(filtered[0].Rules) != 2 {
		t.Fatalf("expected typed rules to be preserved, got %v", filtered[0].Rules)
	}
}
