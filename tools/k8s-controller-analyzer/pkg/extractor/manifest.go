package extractor

import (
	"crypto/sha256"
	"fmt"
	"go/ast"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// ValidSkills lists the accepted --skill values.
var ValidSkills = map[string]bool{
	"architecture":         true,
	"api":                  true,
	"lifecycle":            true,
	"production-readiness": true,
}

// BuildManifest constructs a categorized file manifest with a verification hash,
// replicating the output of evidence_manifest.py in Go.
func BuildManifest(
	skill string,
	pkgs []*packages.Package,
	walk *RepoWalkResult,
	repoPath string,
) ManifestData {
	var entries []ManifestEntry
	seen := map[string]bool{}

	add := func(category string, path string) {
		if seen[path] {
			return
		}
		seen[path] = true
		entries = append(entries, ManifestEntry{Category: category, Path: path})
	}

	switch skill {
	case "architecture":
		for _, p := range discoverEntrypoints(pkgs, repoPath) {
			add("entrypoint", p)
		}
		for _, p := range discoverControllers(pkgs, repoPath) {
			add("controller", p)
		}
		for _, p := range discoverAPITypes(pkgs, repoPath) {
			add("api", p)
		}
		for _, d := range walk.YAMLDocs {
			switch d.Kind {
			case "Role", "ClusterRole", "RoleBinding", "ClusterRoleBinding":
				add("rbac", d.RelPath)
			case "CustomResourceDefinition":
				add("crd", d.RelPath)
			}
		}

	case "api":
		for _, p := range discoverAPITypes(pkgs, repoPath) {
			add("api", p)
		}
		for _, d := range walk.YAMLDocs {
			switch d.Kind {
			case "CustomResourceDefinition":
				add("crd", d.RelPath)
			case "ValidatingWebhookConfiguration", "MutatingWebhookConfiguration":
				add("webhook", d.RelPath)
			}
		}
		// Also include Go files with webhook markers
		for _, p := range discoverWebhookGoFiles(pkgs, repoPath) {
			add("webhook", p)
		}

	case "lifecycle":
		for _, p := range discoverEntrypoints(pkgs, repoPath) {
			add("entrypoint", p)
		}
		for _, p := range discoverControllers(pkgs, repoPath) {
			add("controller", p)
		}
		for _, d := range walk.YAMLDocs {
			switch d.Kind {
			case "Deployment", "StatefulSet":
				add("deployment", d.RelPath)
			case "ValidatingWebhookConfiguration", "MutatingWebhookConfiguration":
				add("webhook", d.RelPath)
			case "CustomResourceDefinition":
				add("crd", d.RelPath)
			}
		}

	case "production-readiness":
		for _, p := range discoverEntrypoints(pkgs, repoPath) {
			add("entrypoint", p)
		}
		for _, p := range discoverControllers(pkgs, repoPath) {
			add("controller", p)
		}
		for _, p := range walk.TestFiles {
			add("test", p)
		}
		for _, d := range walk.YAMLDocs {
			switch d.Kind {
			case "Deployment", "StatefulSet":
				add("deployment", d.RelPath)
			case "Kustomization":
				add("deployment", d.RelPath)
			}
		}
	}

	// Compute hash matching evidence_manifest.py algorithm
	var lines []string
	for _, e := range entries {
		lines = append(lines, fmt.Sprintf("[%s] %s", e.Category, e.Path))
	}
	body := strings.Join(lines, "\n")
	h := sha256.Sum256([]byte(skill + "\n" + body))
	hash := fmt.Sprintf("%x", h)[:12]

	return ManifestData{
		Skill:   skill,
		Count:   len(entries),
		Hash:    hash,
		Entries: entries,
	}
}

func discoverEntrypoints(
	pkgs []*packages.Package,
	repoPath string,
) []string {
	var paths []string

	for _, pkg := range pkgs {
		for i, file := range pkg.Syntax {
			if i >= len(pkg.CompiledGoFiles) {
				continue
			}
			relPath, _ := filepath.Rel(repoPath, pkg.CompiledGoFiles[i])

			// Check for manager setup imports/calls
			if hasImport(file, "sigs.k8s.io/controller-runtime") ||
				hasImport(file, "sigs.k8s.io/controller-runtime/pkg/manager") {
				if filepath.Base(relPath) == "main.go" || strings.Contains(relPath, "cmd/") {
					paths = append(paths, relPath)
				}
			}
		}
	}

	sort.Strings(paths)
	return paths
}

func discoverControllers(
	pkgs []*packages.Package,
	repoPath string,
) []string {
	var paths []string

	for _, pkg := range pkgs {
		for i, file := range pkg.Syntax {
			if i >= len(pkg.CompiledGoFiles) {
				continue
			}
			relPath, _ := filepath.Rel(repoPath, pkg.CompiledGoFiles[i])

			if hasImport(file, "sigs.k8s.io/controller-runtime") ||
				hasImport(file, "sigs.k8s.io/controller-runtime/pkg/reconcile") {
				paths = append(paths, relPath)
			}
		}
	}

	sort.Strings(paths)
	return paths
}

func discoverAPITypes(
	pkgs []*packages.Package,
	repoPath string,
) []string {
	var paths []string

	for _, pkg := range pkgs {
		for i, file := range pkg.Syntax {
			if i >= len(pkg.CompiledGoFiles) {
				continue
			}
			relPath, _ := filepath.Rel(repoPath, pkg.CompiledGoFiles[i])

			// Files with kubebuilder markers or SchemeBuilder
			hasKBMarkers := false
			for _, cg := range file.Comments {
				for _, c := range cg.List {
					if strings.Contains(c.Text, "+kubebuilder:") || strings.Contains(c.Text, "+groupName") {
						hasKBMarkers = true
						break
					}
				}
				if hasKBMarkers {
					break
				}
			}

			if hasKBMarkers {
				paths = append(paths, relPath)
				continue
			}

			// *_types.go files
			if strings.HasSuffix(relPath, "_types.go") || filepath.Base(relPath) == "types.go" {
				paths = append(paths, relPath)
			}
		}
	}

	sort.Strings(paths)
	return paths
}

func discoverWebhookGoFiles(
	pkgs []*packages.Package,
	repoPath string,
) []string {
	var paths []string

	for _, pkg := range pkgs {
		for i, file := range pkg.Syntax {
			if i >= len(pkg.CompiledGoFiles) {
				continue
			}
			relPath, _ := filepath.Rel(repoPath, pkg.CompiledGoFiles[i])

			if hasImport(file, "sigs.k8s.io/controller-runtime/pkg/webhook") {
				paths = append(paths, relPath)
				continue
			}

			// Check for webhook markers
			for _, cg := range file.Comments {
				for _, c := range cg.List {
					if strings.Contains(c.Text, "+kubebuilder:webhook:") {
						paths = append(paths, relPath)
						break
					}
				}
			}
		}
	}

	sort.Strings(paths)
	return paths
}

func hasImport(file *ast.File, path string) bool {
	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		if strings.HasPrefix(importPath, path) {
			return true
		}
	}

	return false
}
