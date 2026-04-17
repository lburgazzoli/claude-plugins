package extractor

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

// ExtractCRDVersions finds CRD type definitions and extracts version info.
func ExtractCRDVersions(
	pkgs []*packages.Package,
	repoPath string,
) []Fact {
	var facts []Fact

	for _, pkg := range pkgs {
		relPkgPath, _ := filepath.Rel(repoPath, pkgDir(pkg))

		// Only scan api/ or apis/ directories
		if !strings.HasPrefix(relPkgPath, "api/") &&
			!strings.HasPrefix(relPkgPath, "apis/") &&
			!strings.Contains(relPkgPath, "/api/") &&
			!strings.Contains(relPkgPath, "/apis/") {
			continue
		}

		// Get group from package-level markers
		group := extractGroupName(pkg)

		// Get version from package path
		version := extractVersionFromPkgPath(pkg.PkgPath)

		// Collect methods defined in this package for hub/spoke detection
		methods := collectMethods(pkg)

		for i, file := range pkg.Syntax {
			filePath := pkg.CompiledGoFiles[i]
			relPath, _ := filepath.Rel(repoPath, filePath)

			ast.Inspect(file, func(n ast.Node) bool {
				gd, ok := n.(*ast.GenDecl)
				if !ok || gd.Tok != token.TYPE {
					return true
				}

				for _, spec := range gd.Specs {
					ts := spec.(*ast.TypeSpec)
					doc := DocOrNearby(file, pkg.Fset, ts.Pos(), ts.Doc)
					if doc == nil {
						doc = DocOrNearby(file, pkg.Fset, gd.Pos(), gd.Doc)
					}

					markers := ExtractMarkersFromDoc(doc, pkg.Fset)
					if !hasMarker(markers, MarkerObjectRoot) {
						continue
					}

					// Skip List types
					if strings.HasSuffix(ts.Name.Name, "List") {
						continue
					}

					data := CRDVersionData{
						Kind:    ts.Name.Name,
						Group:   group,
						Version: version,
						Storage: hasMarker(markers, MarkerStorageVersion),
						Served:  true, // default: served unless explicitly not
						Hub:     hasMethod(methods, ts.Name.Name, "Hub"),
						Spoke:   hasMethod(methods, ts.Name.Name, "ConvertTo") || hasMethod(methods, ts.Name.Name, "ConvertFrom"),
					}

					facts = append(facts, NewFact(
						RuleCRDVersion,
						KindCRDVersion,
						relPath,
						pkg.Fset.Position(ts.Pos()).Line,
						data,
					))
				}

				return true
			})
		}
	}

	return facts
}

func extractGroupName(pkg *packages.Package) string {
	for _, file := range pkg.Syntax {
		markers := ExtractMarkersFromDoc(file.Doc, pkg.Fset)
		for _, m := range markers {
			if strings.HasPrefix(m.Raw, MarkerGroupName+"=") {
				return strings.TrimPrefix(m.Raw, MarkerGroupName+"=")
			}
		}
	}

	return ""
}

func extractVersionFromPkgPath(pkgPath string) string {
	parts := strings.Split(pkgPath, "/")
	if len(parts) > 0 {
		last := parts[len(parts)-1]
		if strings.HasPrefix(last, "v") {
			return last
		}
	}

	return ""
}

type methodKey struct {
	typeName   string
	methodName string
}

func collectMethods(pkg *packages.Package) map[methodKey]bool {
	methods := map[methodKey]bool{}

	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Recv == nil {
				continue
			}
			typeName := receiverTypeName(fd)
			if typeName != "" {
				methods[methodKey{typeName, fd.Name.Name}] = true
			}
		}
	}

	return methods
}

func hasMethod(
	methods map[methodKey]bool,
	typeName string,
	methodName string,
) bool {
	return methods[methodKey{typeName, methodName}]
}

func hasMarker(markers []Marker, name string) bool {
	for _, m := range markers {
		if m.Name == name {
			return true
		}
	}

	return false
}

func pkgDir(pkg *packages.Package) string {
	if len(pkg.CompiledGoFiles) > 0 {
		return filepath.Dir(pkg.CompiledGoFiles[0])
	}

	return ""
}
