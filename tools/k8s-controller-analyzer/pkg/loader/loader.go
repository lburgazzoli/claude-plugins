package loader

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/tools/go/packages"
)

// Load parses all Go packages under the given root directory.
// It returns loaded packages with syntax, types, and file info.
// Packages that fail to load are logged to stderr but do not cause an error.
func Load(
	rootDir string,
	strict bool,
) ([]*packages.Package, error) {
	cfg := &packages.Config{
		Mode: packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedDeps |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedImports |
			packages.NeedName,
		Dir: rootDir,
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("loading packages: %w", err)
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

	return pkgs, nil
}
