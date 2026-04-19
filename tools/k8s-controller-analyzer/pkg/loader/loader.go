package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

// Load parses all Go packages under the given root directory.
// It returns loaded packages with syntax, types, and file info.
// Packages that fail to load are logged to stderr but do not cause an error.
//
// When the repository contains nested Go modules (e.g., a separate api/ module
// as in kubebuilder v4+ projects), Load discovers them and loads their packages
// too so that CRD type extractors can inspect the API types.
func Load(
	rootDir string,
	strict bool,
) ([]*packages.Package, error) {
	moduleDirs := discoverModuleDirs(rootDir)

	cfg := &packages.Config{
		Mode: packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedDeps |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedImports |
			packages.NeedName,
	}

	var allPkgs []*packages.Package

	for _, dir := range moduleDirs {
		cfg.Dir = dir

		pkgs, err := packages.Load(cfg, "./...")
		if err != nil {
			rel, _ := filepath.Rel(rootDir, dir)
			if dir == rootDir {
				return nil, fmt.Errorf("loading packages: %w", err)
			}
			fmt.Fprintf(os.Stderr, "warning: loading nested module %s: %v\n", rel, err)
			continue
		}

		var pkgErrors []string
		for _, pkg := range pkgs {
			for _, e := range pkg.Errors {
				msg := fmt.Sprintf("%s: %s", pkg.PkgPath, e)
				pkgErrors = append(pkgErrors, msg)
				if !strict {
					fmt.Fprintf(os.Stderr, "warning: %s\n", msg)
				}
			}
		}
		if strict && len(pkgErrors) > 0 {
			return nil, fmt.Errorf("package load errors:\n%s", strings.Join(pkgErrors, "\n"))
		}

		allPkgs = append(allPkgs, pkgs...)
	}

	return allPkgs, nil
}

// discoverModuleDirs returns the root directory plus any subdirectories
// that contain their own go.mod (nested modules).
func discoverModuleDirs(rootDir string) []string {
	dirs := []string{rootDir}

	_ = filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		// Skip hidden directories and vendor.
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "testdata" {
				return filepath.SkipDir
			}
		}

		if d.Name() == "go.mod" && path != filepath.Join(rootDir, "go.mod") {
			dirs = append(dirs, filepath.Dir(path))
			return filepath.SkipDir
		}

		return nil
	})

	return dirs
}
