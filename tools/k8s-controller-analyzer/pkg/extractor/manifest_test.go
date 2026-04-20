package extractor

import (
	"testing"

	"golang.org/x/tools/go/packages"
)

func TestBuildManifest_LifecycleCategories(t *testing.T) {
	walk := &RepoWalkResult{
		YAMLDocs: []YAMLDoc{
			{Kind: "Deployment", RelPath: "config/manager/manager.yaml"},
			{Kind: "ValidatingWebhookConfiguration", RelPath: "config/webhook/manifests.yaml"},
			{Kind: "CustomResourceDefinition", RelPath: "config/crd/bases/foo.yaml"},
		},
	}

	repoRoot := "/fake/repo"
	pkg := newSyntheticPackage(t, repoRoot, "main", map[string]string{
		"cmd/main.go": `package main

import (
	ctrl "sigs.k8s.io/controller-runtime"
)

func main() {
	mgr, _ := ctrl.NewManager(nil, ctrl.Options{})
	_ = mgr
}
`,
	})

	m := BuildManifest("lifecycle", []*packages.Package{pkg}, walk, repoRoot)

	if m.Skill != "lifecycle" {
		t.Errorf("expected skill=lifecycle, got %s", m.Skill)
	}
	if m.Count == 0 {
		t.Error("expected non-zero manifest count")
	}
	if len(m.Hash) != 12 {
		t.Errorf("expected 12-char hash, got %d chars: %s", len(m.Hash), m.Hash)
	}

	categories := map[string]bool{}
	for _, e := range m.Entries {
		categories[e.Category] = true
	}

	for _, expected := range []string{"entrypoint", "deployment", "webhook", "crd"} {
		if !categories[expected] {
			t.Errorf("missing expected category %q in lifecycle manifest", expected)
		}
	}
}

func TestBuildManifest_ProductionReadinessNetworkPolicy(t *testing.T) {
	walk := &RepoWalkResult{
		YAMLDocs: []YAMLDoc{
			{Kind: "Deployment", RelPath: "config/manager/manager.yaml"},
			{Kind: "NetworkPolicy", RelPath: "config/networkpolicy/allow-webhook.yaml"},
		},
		TestFiles: []string{"internal/controller/foo_test.go"},
	}

	repoRoot := "/fake/repo"
	pkg := newSyntheticPackage(t, repoRoot, "main", map[string]string{
		"cmd/main.go": `package main

import (
	ctrl "sigs.k8s.io/controller-runtime"
)

func main() {
	mgr, _ := ctrl.NewManager(nil, ctrl.Options{})
	_ = mgr
}
`,
	})

	m := BuildManifest("production-readiness", []*packages.Package{pkg}, walk, repoRoot)

	if m.Skill != "production-readiness" {
		t.Errorf("expected skill=production-readiness, got %s", m.Skill)
	}

	hasNetworkPolicy := false
	for _, e := range m.Entries {
		if e.Category == "networkpolicy" {
			hasNetworkPolicy = true
			break
		}
	}
	if !hasNetworkPolicy {
		t.Error("production-readiness manifest missing networkpolicy category")
	}
}
