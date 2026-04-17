package extractor

import (
	"go/ast"
	"go/token"
	"path/filepath"

	"golang.org/x/tools/go/packages"
)

// ExtractWebhooks finds webhook definitions and extracts their configuration.
func ExtractWebhooks(
	pkgs []*packages.Package,
	repoPath string,
) []Fact {
	var facts []Fact

	for _, pkg := range pkgs {
		for i, file := range pkg.Syntax {
			filePath := pkg.CompiledGoFiles[i]
			relPath, _ := filepath.Rel(repoPath, filePath)

			facts = append(facts, extractWebhooksFromFile(file, pkg.Fset, relPath)...)
		}
	}

	return facts
}

func extractWebhooksFromFile(
	file *ast.File,
	fset *token.FileSet,
	relPath string,
) []Fact {
	var facts []Fact

	// Look for webhook markers on type declarations
	ast.Inspect(file, func(n ast.Node) bool {
		gd, ok := n.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			return true
		}

		for _, spec := range gd.Specs {
			ts := spec.(*ast.TypeSpec)
			doc := DocOrNearby(file, fset, ts.Pos(), ts.Doc)
			if doc == nil {
				doc = DocOrNearby(file, fset, gd.Pos(), gd.Doc)
			}

			markers := ExtractMarkersFromDoc(doc, fset)
			for _, m := range markers {
				if m.Name != MarkerWebhook {
					continue
				}

				whType := inferWebhookType(m, file, ts.Name.Name)

				facts = append(facts, NewFact(
					RuleWebhookAuth,
					KindWebhook,
					relPath,
					m.Line,
					WebhookData{
						Kind:              ts.Name.Name,
						Type:              whType,
						Path:              m.Args[ArgPath],
						FailurePolicy:     m.Args[ArgFailurePolicy],
						SideEffects:       m.Args[ArgSideEffects],
						TimeoutSeconds:    m.Args[ArgTimeoutSeconds],
						HasAuthAnnotation: m.Args[ArgSideEffects] == "None",
					},
				))
			}
		}

		return true
	})

	// Also check function-level webhook markers (less common but possible)
	ast.Inspect(file, func(n ast.Node) bool {
		fd, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}

		doc := DocOrNearby(file, fset, fd.Pos(), fd.Doc)
		markers := ExtractMarkersFromDoc(doc, fset)
		for _, m := range markers {
			if m.Name != MarkerWebhook {
				continue
			}

			kind := ""
			if fd.Recv != nil {
				kind = receiverTypeName(fd)
			}

			whType := "unknown"
			if m.Args[ArgMutating] == "true" {
				whType = "defaulting"
			} else if m.Args[ArgMutating] == "false" {
				whType = "validating"
			}

			facts = append(facts, NewFact(
				RuleWebhookAuth,
				KindWebhook,
				relPath,
				m.Line,
				WebhookData{
					Kind:              kind,
					Type:              whType,
					Path:              m.Args[ArgPath],
					FailurePolicy:     m.Args[ArgFailurePolicy],
					SideEffects:       m.Args[ArgSideEffects],
					TimeoutSeconds:    m.Args[ArgTimeoutSeconds],
					HasAuthAnnotation: m.Args[ArgSideEffects] == "None",
				},
			))
		}

		return true
	})

	return facts
}

func inferWebhookType(
	m Marker,
	file *ast.File,
	typeName string,
) string {
	// Check marker itself
	if m.Args[ArgMutating] == "true" {
		return "defaulting"
	}
	if m.Args[ArgMutating] == "false" {
		return "validating"
	}

	// Infer from methods on the type
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Recv == nil {
			continue
		}
		if receiverTypeName(fd) != typeName {
			continue
		}
		switch fd.Name.Name {
		case "Default":
			return "defaulting"
		case "ValidateCreate", "ValidateUpdate", "ValidateDelete":
			return "validating"
		}
	}

	return "unknown"
}
