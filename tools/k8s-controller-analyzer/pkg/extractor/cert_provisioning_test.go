package extractor

import (
	"testing"

	"golang.org/x/tools/go/packages"
)

func TestExtractCertProvisioningFromGo_CertDirAssignment(t *testing.T) {
	repoRoot := "/fake/repo"
	pkg := newSyntheticPackage(t, repoRoot, "main", map[string]string{
		"cmd/main.go": `package main

import (
	ctrl "sigs.k8s.io/controller-runtime"
)

func main() {
	mgr, _ := ctrl.NewManager(nil, ctrl.Options{})

	var metricsServerOptions struct{ CertDir string }
	metricsServerOptions.CertDir = "/etc/certs"

	_ = mgr
}
`,
	})

	facts := ExtractCertProvisioningFromGo([]*packages.Package{pkg}, repoRoot)
	if len(facts) != 1 {
		t.Fatalf("expected 1 fact, got %d", len(facts))
	}

	data := facts[0].Data.(CertProvisioningData)
	if data.Mechanism != "certdir" {
		t.Errorf("expected mechanism=certdir, got %s", data.Mechanism)
	}
	if data.Source != "go" {
		t.Errorf("expected source=go, got %s", data.Source)
	}
}

func TestExtractCertProvisioningFromGo_CertDirInComposite(t *testing.T) {
	repoRoot := "/fake/repo"
	pkg := newSyntheticPackage(t, repoRoot, "main", map[string]string{
		"cmd/main.go": `package main

import (
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func main() {
	mgr, _ := ctrl.NewManager(nil, ctrl.Options{
		WebhookServer: webhook.NewServer(webhook.Options{
			CertDir: "/tmp/k8s-webhook-server/serving-certs",
		}),
	})
	_ = mgr
}
`,
	})

	facts := ExtractCertProvisioningFromGo([]*packages.Package{pkg}, repoRoot)
	if len(facts) != 1 {
		t.Fatalf("expected 1 fact, got %d", len(facts))
	}

	data := facts[0].Data.(CertProvisioningData)
	if data.Mechanism != "certdir" {
		t.Errorf("expected mechanism=certdir, got %s", data.Mechanism)
	}
}

func TestExtractCertProvisioningFromGo_NoCertDir(t *testing.T) {
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

	facts := ExtractCertProvisioningFromGo([]*packages.Package{pkg}, repoRoot)
	if len(facts) != 0 {
		t.Errorf("expected 0 facts for code without CertDir, got %d", len(facts))
	}
}

func TestExtractCertProvisioningFromGo_NonMainPackageSkipped(t *testing.T) {
	repoRoot := "/fake/repo"
	pkg := newSyntheticPackage(t, repoRoot, "internal/controller", map[string]string{
		"internal/controller/setup.go": `package controller

func setup() {
	var opts struct{ CertDir string }
	opts.CertDir = "/etc/certs"
}
`,
	})

	facts := ExtractCertProvisioningFromGo([]*packages.Package{pkg}, repoRoot)
	if len(facts) != 0 {
		t.Errorf("expected 0 facts for non-main package, got %d", len(facts))
	}
}
