package extractor

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

var apiRules = []string{
	RuleCRDStructure,
	RuleFieldConventions,
	RuleMarkerCorrectness,
}

// ExtractAPIFields finds CRD type definitions and extracts field-level markers,
// JSON tags, type info, and root type markers for the k8s.controller-api skill.
func ExtractAPIFields(
	pkgs []*packages.Package,
	repoPath string,
) []Fact {
	var facts []Fact

	for _, pkg := range pkgs {
		relPkgPath, _ := filepath.Rel(repoPath, pkgDir(pkg))

		if !isAPIPackage(relPkgPath) {
			continue
		}

		for i, file := range pkg.Syntax {
			filePath := pkg.CompiledGoFiles[i]
			relPath, _ := filepath.Rel(repoPath, filePath)

			// Skip generated files
			if strings.Contains(relPath, "zz_generated") {
				continue
			}

			facts = append(facts, extractAPIFieldsFromFile(file, pkg.Fset, relPath)...)
		}
	}

	return facts
}

func isAPIPackage(relPath string) bool {
	return strings.HasPrefix(relPath, "api/") ||
		strings.HasPrefix(relPath, "apis/") ||
		strings.Contains(relPath, "/api/") ||
		strings.Contains(relPath, "/apis/")
}

func extractAPIFieldsFromFile(
	file *ast.File,
	fset *token.FileSet,
	relPath string,
) []Fact {
	var facts []Fact

	ast.Inspect(file, func(n ast.Node) bool {
		gd, ok := n.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			return true
		}

		for _, spec := range gd.Specs {
			ts := spec.(*ast.TypeSpec)
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}

			groups := CollectNearbyCommentGroups(file, fset, ts.Pos(), ts.Doc, 3)
			if len(groups) == 0 {
				groups = CollectNearbyCommentGroups(file, fset, gd.Pos(), gd.Doc, 3)
			}
			markers := ExtractMarkersFromGroups(groups, fset)

			isRoot := hasMarker(markers, MarkerObjectRoot)

			// Skip List types
			if strings.HasSuffix(ts.Name.Name, "List") {
				return true
			}

			if isRoot {
				// Extract root type data
				typeData := extractCRDTypeData(ts, st, markers, fset)
				facts = append(facts, NewMultiRuleFact(
					apiRules,
					KindCRDType,
					relPath,
					fset.Position(ts.Pos()).Line,
					typeData,
				))
			}

			// Extract field-level data for all structs in API packages
			for _, field := range st.Fields.List {
				if len(field.Names) == 0 {
					continue // embedded field
				}

				fieldData := extractFieldData(ts.Name.Name, field, fset, file)
				if fieldData != nil {
					facts = append(facts, NewMultiRuleFact(
						apiRules,
						KindCRDField,
						relPath,
						fset.Position(field.Pos()).Line,
						*fieldData,
					))
				}
			}
		}

		return true
	})

	return facts
}

func extractCRDTypeData(
	ts *ast.TypeSpec,
	st *ast.StructType,
	markers []Marker,
	fset *token.FileSet,
) CRDTypeData {
	data := CRDTypeData{
		Kind:          ts.Name.Name,
		HasRootMarker: hasMarker(markers, MarkerObjectRoot),
		HasStatusSub:  hasMarker(markers, "kubebuilder:subresource:status"),
	}

	// Check for resource scope, print columns, and CEL rules
	for _, m := range markers {
		if m.Name == MarkerResource || strings.HasPrefix(m.Name, MarkerResource+":") {
			data.ResourceScope = m.Args[ArgScope]
		}
		if m.Name == MarkerPrintColumn {
			data.PrintColumns = append(data.PrintColumns, FieldMarker{
				Name:  m.Args[ArgName],
				Value: m.Raw,
				Line:  m.Line,
			})
		}
		if strings.HasPrefix(m.Name, MarkerXValidation) {
			rule := m.Args["rule"]
			data.CELRules = append(data.CELRules, CELRule{
				Rule:       rule,
				Message:    m.Args["message"],
				UsesOldSel: strings.Contains(rule, "oldSelf"),
				Line:       m.Line,
			})
		}
	}

	// Check for Status field and unsigned numeric fields
	for _, field := range st.Fields.List {
		if len(field.Names) > 0 && field.Names[0].Name == "Status" {
			data.HasStatusField = true
			data.StatusFieldType = typeExprToString(field.Type)
		}

		// Check for unsigned numeric types
		if len(field.Names) > 0 {
			fieldTypeName := typeExprToString(field.Type)
			if isUnsignedType(fieldTypeName) {
				data.UnsignedFields = append(data.UnsignedFields, CRDFieldData{
					TypeName:  ts.Name.Name,
					FieldName: field.Names[0].Name,
					FieldType: fieldTypeName,
				})
			}
		}
	}

	return data
}

func extractFieldData(
	typeName string,
	field *ast.Field,
	fset *token.FileSet,
	file *ast.File,
) *CRDFieldData {
	if len(field.Names) == 0 {
		return nil
	}

	data := CRDFieldData{
		TypeName:  typeName,
		FieldName: field.Names[0].Name,
		FieldType: typeExprToString(field.Type),
	}

	// Parse JSON tag
	if field.Tag != nil {
		tag := strings.Trim(field.Tag.Value, "`")
		if jsonTag, ok := parseJSONTag(tag); ok {
			data.JSONTag = jsonTag
			data.HasOmitempty = strings.Contains(jsonTag, "omitempty")
		}
	}

	// Extract markers from field doc
	doc := DocOrNearby(file, fset, field.Pos(), field.Doc)
	fieldMarkers := ExtractMarkersFromDoc(doc, fset)

	for _, m := range fieldMarkers {
		switch {
		case m.Name == MarkerValidationOpt || m.Raw == "optional":
			data.IsOptional = true
		case m.Name == MarkerValidationReq:
			data.IsRequired = true
		case m.Name == MarkerListType:
			data.ListType = m.Args[MarkerListType]
		case m.Name == MarkerListMapKey:
			data.ListMapKeys = append(data.ListMapKeys, m.Args[MarkerListMapKey])
		}

		// Collect CEL validation rules
		if strings.HasPrefix(m.Name, MarkerXValidation) {
			rule := m.Args["rule"]
			data.CELRules = append(data.CELRules, CELRule{
				Rule:       rule,
				Message:    m.Args["message"],
				UsesOldSel: strings.Contains(rule, "oldSelf"),
				Line:       m.Line,
			})
		}

		// Detect size-bounding markers for CEL cost analysis
		if m.Name == "kubebuilder:validation:MaxItems" {
			data.HasMaxItems = true
		}
		if m.Name == "kubebuilder:validation:MaxProperties" {
			data.HasMaxProperties = true
		}

		// Collect all validation/default markers
		if strings.Contains(m.Name, "kubebuilder:validation") ||
			strings.Contains(m.Name, "kubebuilder:default") {
			data.Markers = append(data.Markers, FieldMarker{
				Name:  m.Name,
				Value: m.Raw,
				Line:  m.Line,
			})
		}
	}

	return &data
}

func typeExprToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		if pkg, ok := t.X.(*ast.Ident); ok {
			return pkg.Name + "." + t.Sel.Name
		}
		return t.Sel.Name
	case *ast.StarExpr:
		return "*" + typeExprToString(t.X)
	case *ast.ArrayType:
		return "[]" + typeExprToString(t.Elt)
	case *ast.MapType:
		return "map[" + typeExprToString(t.Key) + "]" + typeExprToString(t.Value)
	}

	return "?"
}

func isUnsignedType(typeName string) bool {
	unsigned := []string{"uint", "uint8", "uint16", "uint32", "uint64"}
	for _, u := range unsigned {
		if typeName == u {
			return true
		}
	}

	return false
}

func parseJSONTag(tag string) (string, bool) {
	// Find json:"..." in the struct tag
	for _, part := range strings.Split(tag, " ") {
		if strings.HasPrefix(part, `json:"`) {
			value := strings.TrimPrefix(part, `json:"`)
			value = strings.TrimSuffix(value, `"`)
			return value, true
		}
	}

	return "", false
}
