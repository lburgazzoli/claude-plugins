package extractor

import (
	"go/ast"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

// ExtractCertProvisioningFromGo scans main/cmd packages for CertDir assignments
// and cert-related flag registrations, emitting cert_provisioning facts.
func ExtractCertProvisioningFromGo(
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

			facts = append(facts, extractCertDirFromFile(file, relPath)...)
		}
	}

	return facts
}

func extractCertDirFromFile(
	file *ast.File,
	relPath string,
) []Fact {
	var facts []Fact
	seen := map[string]bool{}

	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			// Detect: someOptions.CertDir = value
			for _, lhs := range node.Lhs {
				sel, ok := lhs.(*ast.SelectorExpr)
				if !ok {
					continue
				}
				if sel.Sel.Name == "CertDir" && !seen["certdir"] {
					seen["certdir"] = true

					detail := ""
					if len(node.Rhs) > 0 {
						detail = exprToString(node.Rhs[0])
					}

					facts = append(facts, NewFact(
						RuleCertProvisioning,
						KindCertProvisioning,
						relPath,
						0,
						CertProvisioningData{
							Mechanism: "certdir",
							Source:    "go",
							Detail:    detail,
						},
					))
				}
			}

		case *ast.KeyValueExpr:
			// Detect: Options{CertDir: value} in composite literals
			key, ok := node.Key.(*ast.Ident)
			if !ok {
				return true
			}
			if key.Name == "CertDir" && !seen["certdir"] {
				seen["certdir"] = true

				facts = append(facts, NewFact(
					RuleCertProvisioning,
					KindCertProvisioning,
					relPath,
					0,
					CertProvisioningData{
						Mechanism: "certdir",
						Source:    "go",
						Detail:    exprToString(node.Value),
					},
				))
			}

		case *ast.CallExpr:
			// Detect: flag.StringVar(&var, "metrics-cert-path", ...) or "cert-dir"
			sel, ok := node.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if sel.Sel.Name != "StringVar" {
				return true
			}
			if len(node.Args) < 2 {
				return true
			}
			flagName := exprToString(node.Args[1])
			flagName = strings.Trim(flagName, `"`)
			if isCertFlag(flagName) && !seen["certflag"] {
				seen["certflag"] = true

				facts = append(facts, NewFact(
					RuleCertProvisioning,
					KindCertProvisioning,
					relPath,
					0,
					CertProvisioningData{
						Mechanism: "certdir",
						Source:    "go",
						Detail:    flagName,
					},
				))
			}
		}

		return true
	})

	return facts
}

func isCertFlag(name string) bool {
	return name == "cert-dir" ||
		name == "metrics-cert-path" ||
		name == "tls-cert-file" ||
		name == "webhook-cert-dir"
}
