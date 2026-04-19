package extractor

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

// ExtractManagerConfig finds manager.New() or ctrl.NewManager() calls in main/cmd
// packages and extracts leader election, signal handler, and shutdown configuration.
func ExtractManagerConfig(
	pkgs []*packages.Package,
	repoPath string,
) []Fact {
	var facts []Fact

	for _, pkg := range pkgs {
		relPkgPath, _ := filepath.Rel(repoPath, pkgDir(pkg))

		if !isMainOrCmdPackage(relPkgPath, pkg) {
			continue
		}

		for i, file := range pkg.Syntax {
			filePath := pkg.CompiledGoFiles[i]
			relPath, _ := filepath.Rel(repoPath, filePath)

			data := extractManagerConfigFromFile(file, pkg.Fset)
			if data != nil {
				facts = append(facts, NewFact(
					RuleManagerConfig,
					KindManagerConfig,
					relPath,
					0,
					*data,
				))
			}
		}
	}

	return facts
}

func isMainOrCmdPackage(
	relPath string,
	pkg *packages.Package,
) bool {
	if pkg.Name == "main" {
		return true
	}

	return strings.HasPrefix(relPath, "cmd/") ||
		strings.HasPrefix(relPath, "main/") ||
		relPath == "cmd" ||
		relPath == "main"
}

func extractManagerConfigFromFile(
	file *ast.File,
	fset *token.FileSet,
) *ManagerConfigData {
	var data *ManagerConfigData

	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Look for ctrl.NewManager(...) or manager.New(...)
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		pkg, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}

		isNewManager := (sel.Sel.Name == FuncNewManager && (pkg.Name == "ctrl" || pkg.Name == "manager"))
		if !isNewManager {
			return true
		}

		data = &ManagerConfigData{}

		// The options struct is typically the last argument
		for _, arg := range call.Args {
			extractManagerOptions(arg, data)
		}

		return true
	})

	// Check for SetupSignalHandler usage anywhere in the file
	if data != nil {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if sel.Sel.Name == FuncSetupSignalHandler {
				data.HasSignalHandler = true
			}

			return true
		})
	}

	return data
}

func extractManagerOptions(
	expr ast.Expr,
	data *ManagerConfigData,
) {
	// Handle ctrl.Options{...} or manager.Options{...}
	cl, ok := expr.(*ast.CompositeLit)
	if !ok {
		return
	}

	typeName := typeExprName(cl.Type)
	if typeName != "ctrl.Options" && typeName != "manager.Options" && typeName != "Options" {
		return
	}

	for _, elt := range cl.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}

		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}

		switch key.Name {
		case "LeaderElection":
			data.LeaderElection = isTrueExpr(kv.Value)
		case "LeaderElectionID":
			if lit, ok := kv.Value.(*ast.BasicLit); ok {
				data.LeaderElectionID = strings.Trim(lit.Value, `"`)
			}
		case "LeaderElectionResourceLock":
			if lit, ok := kv.Value.(*ast.BasicLit); ok {
				data.LeaderElectionResourceLock = strings.Trim(lit.Value, `"`)
			} else if sel, ok := kv.Value.(*ast.SelectorExpr); ok {
				data.LeaderElectionResourceLock = sel.Sel.Name
			}
		case "LeaderElectionReleaseOnCancel":
			data.LeaderElectionReleaseOnCancel = isTrueExpr(kv.Value)
		case "GracefulShutdownTimeout":
			data.GracefulShutdownTimeout = exprToString(kv.Value)
		}
	}
}

func isTrueExpr(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == "true"
}
