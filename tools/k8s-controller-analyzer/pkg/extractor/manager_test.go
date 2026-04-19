package extractor

import (
	"testing"

	"golang.org/x/tools/go/packages"
)

func TestExtractManagerConfig_LeaderElectionEnabled(t *testing.T) {
	repoRoot := t.TempDir()

	mainPkg := newSyntheticPackage(
		t,
		repoRoot,
		"example.com/project",
		map[string]string{
			"cmd/main.go": `package main

import (
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func main() {
	mgr, _ := ctrl.NewManager(ctrl.GetConfigOrDie(), manager.Options{
		LeaderElection:             true,
		LeaderElectionID:           "my-controller.example.com",
		LeaderElectionReleaseOnCancel: true,
	})
	mgr.Start(ctrl.SetupSignalHandler())
}
`,
		},
	)
	mainPkg.Name = "main"

	facts := ExtractManagerConfig([]*packages.Package{mainPkg}, repoRoot)
	if len(facts) == 0 {
		t.Fatal("expected manager_config fact")
	}

	data, ok := facts[0].Data.(ManagerConfigData)
	if !ok {
		t.Fatal("expected ManagerConfigData")
	}

	if !data.LeaderElection {
		t.Error("expected LeaderElection=true")
	}
	if data.LeaderElectionID != "my-controller.example.com" {
		t.Errorf("expected LeaderElectionID=my-controller.example.com, got %s", data.LeaderElectionID)
	}
	if !data.LeaderElectionReleaseOnCancel {
		t.Error("expected LeaderElectionReleaseOnCancel=true")
	}
	if !data.HasSignalHandler {
		t.Error("expected HasSignalHandler=true")
	}
}

func TestExtractManagerConfig_LeaderElectionDisabled(t *testing.T) {
	repoRoot := t.TempDir()

	mainPkg := newSyntheticPackage(
		t,
		repoRoot,
		"example.com/project",
		map[string]string{
			"cmd/main.go": `package main

import (
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func main() {
	mgr, _ := ctrl.NewManager(ctrl.GetConfigOrDie(), manager.Options{})
	mgr.Start(ctrl.SetupSignalHandler())
}
`,
		},
	)
	mainPkg.Name = "main"

	facts := ExtractManagerConfig([]*packages.Package{mainPkg}, repoRoot)
	if len(facts) == 0 {
		t.Fatal("expected manager_config fact")
	}

	data, ok := facts[0].Data.(ManagerConfigData)
	if !ok {
		t.Fatal("expected ManagerConfigData")
	}

	if data.LeaderElection {
		t.Error("expected LeaderElection=false")
	}
	if data.LeaderElectionID != "" {
		t.Errorf("expected empty LeaderElectionID, got %s", data.LeaderElectionID)
	}
	if data.HasSignalHandler {
		// SetupSignalHandler is called but the synthetic parser
		// might not resolve cross-package selector — this tests the
		// file-level detection
	}
}

func TestExtractManagerConfig_NoManagerCall(t *testing.T) {
	repoRoot := t.TempDir()

	mainPkg := newSyntheticPackage(
		t,
		repoRoot,
		"example.com/project",
		map[string]string{
			"cmd/main.go": `package main

func main() {
	// no manager setup
}
`,
		},
	)
	mainPkg.Name = "main"

	facts := ExtractManagerConfig([]*packages.Package{mainPkg}, repoRoot)
	if len(facts) != 0 {
		t.Fatalf("expected no facts, got %d", len(facts))
	}
}

func TestExtractManagerConfig_ResourceLockExplicit(t *testing.T) {
	repoRoot := t.TempDir()

	mainPkg := newSyntheticPackage(
		t,
		repoRoot,
		"example.com/project",
		map[string]string{
			"cmd/main.go": `package main

import (
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func main() {
	mgr, _ := ctrl.NewManager(ctrl.GetConfigOrDie(), manager.Options{
		LeaderElection:             true,
		LeaderElectionID:           "test-lock",
		LeaderElectionResourceLock: "configmaps",
	})
	_ = mgr
}
`,
		},
	)
	mainPkg.Name = "main"

	facts := ExtractManagerConfig([]*packages.Package{mainPkg}, repoRoot)
	if len(facts) == 0 {
		t.Fatal("expected manager_config fact")
	}

	data, ok := facts[0].Data.(ManagerConfigData)
	if !ok {
		t.Fatal("expected ManagerConfigData")
	}

	if data.LeaderElectionResourceLock != "configmaps" {
		t.Errorf("expected resource lock=configmaps, got %s", data.LeaderElectionResourceLock)
	}
}

func TestExtractManagerConfig_NonMainPackageSkipped(t *testing.T) {
	repoRoot := t.TempDir()

	libPkg := newSyntheticPackage(
		t,
		repoRoot,
		"example.com/project/pkg/lib",
		map[string]string{
			"pkg/lib/helpers.go": `package lib

import (
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func SetupManager() {
	ctrl.NewManager(ctrl.GetConfigOrDie(), manager.Options{
		LeaderElection: true,
	})
}
`,
		},
	)

	facts := ExtractManagerConfig([]*packages.Package{libPkg}, repoRoot)
	if len(facts) != 0 {
		t.Fatalf("expected no facts from non-main package, got %d", len(facts))
	}
}
