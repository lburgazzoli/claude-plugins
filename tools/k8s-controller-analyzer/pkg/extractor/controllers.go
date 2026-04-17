package extractor

import (
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

// writeMethodSet is the set of Kubernetes client write methods.
var writeMethodSet = map[string]bool{
	MethodCreate: true, MethodUpdate: true, MethodPatch: true, MethodDelete: true,
}

var controllerRules = []string{
	RuleRBACCoverage,
	RuleRequeueSafety,
	RuleFinalizerSafety,
	RuleStatusConditions,
	RuleWatchOwns,
	RuleLibraryRendering,
}

// ExtractControllers finds reconciler structs and extracts controller facts.
func ExtractControllers(
	pkgs []*packages.Package,
	repoPath string,
) []Fact {
	var facts []Fact
	callGraph := buildRepoCallGraph(pkgs, repoPath)

	for _, pkg := range pkgs {
		index := buildPackageIndex(pkg, repoPath)
		reconcilers := findReconcilers(index)
		for _, rec := range reconcilers {
			data := buildControllerData(index, rec, repoPath, pkgs, callGraph)
			facts = append(facts, NewMultiRuleFact(
				controllerRules,
				KindController,
				rec.relPath,
				rec.line,
				data,
			))
		}
	}

	return facts
}

type packageFile struct {
	file    *ast.File
	relPath string
}

type typeDeclInfo struct {
	file    *ast.File
	relPath string
	spec    *ast.TypeSpec
	line    int
}

type methodDeclInfo struct {
	file    *ast.File
	relPath string
	decl    *ast.FuncDecl
}

type packageIndex struct {
	fset    *token.FileSet
	pkgPath string
	types   map[string]typeDeclInfo
	methods map[string]map[string]methodDeclInfo
}

type repoFunctionInfo struct {
	id          string
	pkg         *packages.Package
	file        *ast.File
	relPath     string
	decl        *ast.FuncDecl
	name        string
	receiverKey string
}

type repoCallGraph struct {
	functions    map[string]repoFunctionInfo
	edges        map[string]map[string]struct{}
	invocations  map[string][]LibraryInvocation
	reachability map[string]map[string]bool
}

type reconcilerInfo struct {
	name    string
	line    int
	relPath string
}

func buildPackageIndex(
	pkg *packages.Package,
	repoPath string,
) *packageIndex {
	index := &packageIndex{
		fset:    pkg.Fset,
		pkgPath: pkg.PkgPath,
		types:   map[string]typeDeclInfo{},
		methods: map[string]map[string]methodDeclInfo{},
	}

	for i, file := range pkg.Syntax {
		relPath := ""
		if i < len(pkg.CompiledGoFiles) {
			filePath := pkg.CompiledGoFiles[i]
			relPath, _ = filepath.Rel(repoPath, filePath)
		}

		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if ok && fd.Recv != nil {
				recv := receiverTypeName(fd)
				if recv != "" {
					if _, exists := index.methods[recv]; !exists {
						index.methods[recv] = map[string]methodDeclInfo{}
					}
					index.methods[recv][fd.Name.Name] = methodDeclInfo{
						file:    file,
						relPath: relPath,
						decl:    fd,
					}
				}
				continue
			}

			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if _, ok := ts.Type.(*ast.StructType); !ok {
					continue
				}
				index.types[ts.Name.Name] = typeDeclInfo{
					file:    file,
					relPath: relPath,
					spec:    ts,
					line:    pkg.Fset.Position(ts.Pos()).Line,
				}
			}
		}
	}

	return index
}

func findReconcilers(index *packageIndex) []reconcilerInfo {
	var reconcilers []reconcilerInfo

	for typeName, typeInfo := range index.types {
		reconcileMethod, exists := index.method(typeName, FuncReconcile)
		if !exists || !looksLikeReconcileMethod(reconcileMethod.decl) {
			continue
		}

		_, hasSetup := index.method(typeName, FuncSetupWithManager)
		if !hasSetup && !strings.HasSuffix(typeName, "Reconciler") {
			continue
		}

		reconcilers = append(reconcilers, reconcilerInfo{
			name:    typeName,
			line:    typeInfo.line,
			relPath: typeInfo.relPath,
		})
	}

	// Sort for deterministic output when multiple reconcilers exist.
	sort.Slice(reconcilers, func(i, j int) bool {
		if reconcilers[i].relPath != reconcilers[j].relPath {
			return reconcilers[i].relPath < reconcilers[j].relPath
		}
		return reconcilers[i].line < reconcilers[j].line
	})

	return reconcilers
}

func (p *packageIndex) method(
	typeName string,
	methodName string,
) (methodDeclInfo, bool) {
	methods, ok := p.methods[typeName]
	if !ok {
		return methodDeclInfo{}, false
	}
	method, ok := methods[methodName]
	if !ok {
		return methodDeclInfo{}, false
	}

	return method, true
}

func buildControllerData(
	index *packageIndex,
	rec reconcilerInfo,
	repoPath string,
	pkgs []*packages.Package,
	callGraph *repoCallGraph,
) ControllerData {
	data := ControllerData{
		Name: rec.name,
	}

	// Extract RBAC markers from Reconcile method and struct declaration
	data.RBACMarkers = extractRBACMarkers(index, rec.name)

	// Extract owns/watches from SetupWithManager
	data.Owns, data.Watches = extractOwnsWatches(index, rec.name)

	// Extract from Reconcile method body
	if reconcileMethod, ok := index.method(rec.name, FuncReconcile); ok {
		reconcileFunc := reconcileMethod.decl
		fset := index.fset
		data.FinalizerOps = extractFinalizerOps(reconcileFunc, fset)
		data.OwnerRefOps = extractOwnerRefOps(reconcileFunc, fset)
		data.ExternalWriteOps = extractWriteOps(reconcileFunc, fset)
		data.APICalls = extractAPICalls(reconcileFunc, fset)
		data.StatusConditionSets = extractStatusConditions(reconcileFunc, fset)
		data.EventUsages = extractEventUsages(reconcileFunc, fset)
		data.NotFoundHandlers = extractNotFoundHandlers(reconcileFunc, fset)
		data.RequeueOps = extractRequeueOps(reconcileFunc, fset)
		data.ErrorReturns = extractErrorReturns(reconcileFunc, fset)
	}

	// Extract retry ops and status update sites from ALL methods on the reconciler.
	// Retry wrappers commonly live in helper methods (e.g., updateStatus).
	var allRetryOps []RetryOp
	var allStatusUpdateSites []StatusUpdateSite

	for methodName, methodInfo := range index.methods[rec.name] {
		fd := methodInfo.decl
		fset := index.fset
		retryAliases := retryImportAliases(methodInfo.file)

		retryOps := extractRetryOps(fd, fset, methodName, retryAliases)
		allRetryOps = append(allRetryOps, retryOps...)

		spans := collectRetrySpans(fd, retryAliases)
		sites := extractStatusUpdateSites(fd, fset, methodName, spans)
		allStatusUpdateSites = append(allStatusUpdateSites, sites...)
	}

	allLibraryInvocations := collectControllerLibraryInvocations(callGraph, index, rec)

	sort.Slice(allRetryOps, func(i, j int) bool { return allRetryOps[i].Line < allRetryOps[j].Line })
	sort.Slice(allStatusUpdateSites, func(i, j int) bool { return allStatusUpdateSites[i].Line < allStatusUpdateSites[j].Line })
	sort.Slice(allLibraryInvocations, func(i, j int) bool {
		if allLibraryInvocations[i].Method != allLibraryInvocations[j].Method {
			return allLibraryInvocations[i].Method < allLibraryInvocations[j].Method
		}
		if allLibraryInvocations[i].Line != allLibraryInvocations[j].Line {
			return allLibraryInvocations[i].Line < allLibraryInvocations[j].Line
		}
		return allLibraryInvocations[i].Call < allLibraryInvocations[j].Call
	})

	data.RetryOps = allRetryOps
	data.StatusUpdateSites = allStatusUpdateSites
	data.LibraryInvocations = allLibraryInvocations

	// Extract predicate usage from SetupWithManager
	data.PredicateUsages = extractPredicateUsages(index, rec.name)

	// Try to infer reconciled kind from struct name (FooReconciler -> Foo)
	data.Reconciles.Kind = strings.TrimSuffix(rec.name, "Reconciler")

	// Try to find group/version from api packages
	data.Reconciles.Group, data.Reconciles.Version = inferGroupVersion(
		pkgs,
		data.Reconciles.Kind,
		repoPath,
	)

	return data
}

func extractRBACMarkers(
	index *packageIndex,
	reconcilerName string,
) []RBACMarker {
	var markers []RBACMarker

	// Check markers on Reconcile method
	if method, ok := index.method(reconcilerName, FuncReconcile); ok {
		doc := DocOrNearby(method.file, index.fset, method.decl.Pos(), method.decl.Doc)
		for _, m := range ExtractMarkersFromDoc(doc, index.fset) {
			if m.Name == MarkerRBAC {
				markers = append(markers, rbacMarkerFromArgs(m))
			}
		}
	}

	// Check markers on the struct declaration
	if typeInfo, ok := index.types[reconcilerName]; ok {
		doc := DocOrNearby(typeInfo.file, index.fset, typeInfo.spec.Pos(), typeInfo.spec.Doc)
		for _, m := range ExtractMarkersFromDoc(doc, index.fset) {
			if m.Name == MarkerRBAC {
				markers = append(markers, rbacMarkerFromArgs(m))
			}
		}
	}

	return markers
}

func extractOwnsWatches(
	index *packageIndex,
	reconcilerName string,
) ([]string, []string) {
	var owns, watches []string

	setupMethod, ok := index.method(reconcilerName, FuncSetupWithManager)
	if !ok || setupMethod.decl.Body == nil {
		return owns, watches
	}

	ast.Inspect(setupMethod.decl.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		switch sel.Sel.Name {
		case FuncOwns:
			if typeName := extractTypeArgFromUnaryAnd(call); typeName != "" {
				owns = append(owns, typeName)
			}
		case FuncWatches:
			if len(call.Args) > 0 {
				if typeName := extractTypeFromExpr(call.Args[0]); typeName != "" {
					watches = append(watches, typeName)
				}
			}
		}

		return true
	})

	return owns, watches
}

func extractFinalizerOps(
	fd *ast.FuncDecl,
	fset *token.FileSet,
) []FinalizerOp {
	var ops []FinalizerOp

	ast.Inspect(fd.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		funcName := callFuncName(call)
		if funcName != FuncAddFinalizer && funcName != FuncRemoveFinalizer {
			return true
		}

		op := "AddFinalizer"
		if funcName == FuncRemoveFinalizer {
			op = "RemoveFinalizer"
		}

		value := ""
		if len(call.Args) >= 2 {
			if lit, ok := call.Args[1].(*ast.BasicLit); ok {
				value = strings.Trim(lit.Value, `"`)
			}
		}

		ops = append(ops, FinalizerOp{
			Op:    op,
			Value: value,
			Line:  fset.Position(call.Pos()).Line,
		})

		return true
	})

	return ops
}

func extractWriteOps(
	fd *ast.FuncDecl,
	fset *token.FileSet,
) []WriteOp {
	var ops []WriteOp

	ast.Inspect(fd.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		if writeMethodSet[sel.Sel.Name] {
			callStr := callFuncName(call)
			ops = append(ops, WriteOp{
				Call: callStr,
				Line: fset.Position(call.Pos()).Line,
			})
		}

		return true
	})

	return ops
}

func extractStatusConditions(
	fd *ast.FuncDecl,
	fset *token.FileSet,
) []StatusConditionSet {
	var conditions []StatusConditionSet

	ast.Inspect(fd.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		funcName := callFuncName(call)

		// meta.SetStatusCondition or conditions.Set or apimeta.SetStatusCondition
		isStatusConditionSet := strings.HasSuffix(funcName, "SetStatusCondition") ||
			funcName == "conditions.Set" || funcName == "apimeta.SetStatusCondition"

		if !isStatusConditionSet {
			return true
		}

		// Try to extract condition type and ObservedGeneration from composite literal argument
		for _, arg := range call.Args {
			condType, hasOG := extractConditionTypeAndOG(arg)
			if condType != "" {
				conditions = append(conditions, StatusConditionSet{
					Condition:      condType,
					HasObservedGen: hasOG,
					Line:           fset.Position(call.Pos()).Line,
				})
			}
		}

		return true
	})

	return conditions
}

func extractRequeueOps(
	fd *ast.FuncDecl,
	fset *token.FileSet,
) []RequeueOp {
	var ops []RequeueOp
	resultStates := collectReconcileResultStates(fd)

	ast.Inspect(fd.Body, func(n ast.Node) bool {
		ret, ok := n.(*ast.ReturnStmt)
		if !ok {
			return true
		}

		hasRequeue := false
		hasRequeueAfter := false

		for _, r := range ret.Results {
			state, ok := resolveReconcileResultState(r, resultStates)
			if !ok {
				continue
			}
			if state.hasRequeue {
				hasRequeue = true
			}
			if state.hasRequeueAfter {
				hasRequeueAfter = true
			}
		}

		if hasRequeue {
			ops = append(ops, RequeueOp{
				Kind: "Requeue",
				Line: fset.Position(ret.Pos()).Line,
			})
		}
		if hasRequeueAfter {
			ops = append(ops, RequeueOp{
				Kind: "RequeueAfter",
				Line: fset.Position(ret.Pos()).Line,
			})
		}

		return true
	})

	return ops
}

func extractErrorReturns(
	fd *ast.FuncDecl,
	fset *token.FileSet,
) []ErrorReturn {
	var returns []ErrorReturn
	resultStates := collectReconcileResultStates(fd)

	ast.Inspect(fd.Body, func(n ast.Node) bool {
		ret, ok := n.(*ast.ReturnStmt)
		if !ok || len(ret.Results) < 2 {
			return true
		}

		// Last result should be non-nil error
		lastResult := ret.Results[len(ret.Results)-1]
		if isNilIdent(lastResult) {
			return true
		}

		// Check if the Result in this return has Requeue set
		hasRequeue := false
		for _, r := range ret.Results[:len(ret.Results)-1] {
			state, ok := resolveReconcileResultState(r, resultStates)
			if !ok {
				continue
			}
			if state.hasRequeue || state.hasRequeueAfter {
				hasRequeue = true
				break
			}
		}

		returns = append(returns, ErrorReturn{
			Line:       fset.Position(ret.Pos()).Line,
			HasRequeue: hasRequeue,
		})

		return true
	})

	return returns
}

// Helper functions

func receiverTypeName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return ""
	}

	t := fd.Recv.List[0].Type
	if star, ok := t.(*ast.StarExpr); ok {
		t = star.X
	}
	if ident, ok := t.(*ast.Ident); ok {
		return ident.Name
	}

	return ""
}

func findMethod(
	file *ast.File,
	typeName string,
	methodName string,
) *ast.FuncDecl {
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Name.Name != methodName {
			continue
		}
		if receiverTypeName(fd) == typeName {
			return fd
		}
	}

	return nil
}

func extractTypeArgFromUnaryAnd(call *ast.CallExpr) string {
	if len(call.Args) == 0 {
		return ""
	}

	return extractTypeFromExpr(call.Args[0])
}

func extractTypeFromExpr(expr ast.Expr) string {
	// &Type{}
	if unary, ok := expr.(*ast.UnaryExpr); ok {
		if cl, ok := unary.X.(*ast.CompositeLit); ok {
			return typeExprName(cl.Type)
		}
	}
	// Type{}
	if cl, ok := expr.(*ast.CompositeLit); ok {
		return typeExprName(cl.Type)
	}

	return ""
}

func typeExprName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return t.Sel.Name
	}

	return ""
}

func callFuncName(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.SelectorExpr:
		return selectorString(fn)
	case *ast.Ident:
		return fn.Name
	}

	return ""
}

func selectorString(sel *ast.SelectorExpr) string {
	base := selectorBaseString(sel.X)
	if base == "" {
		return sel.Sel.Name
	}

	return base + "." + sel.Sel.Name
}

func selectorBaseString(expr ast.Expr) string {
	switch x := expr.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return selectorString(x)
	case *ast.CallExpr:
		callName := callExprString(x.Fun)
		if callName == "" {
			return ""
		}
		return callName + "()"
	}

	return ""
}

func callExprString(fun ast.Expr) string {
	switch fn := fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		return selectorString(fn)
	}

	return ""
}

func isReconcileResult(cl *ast.CompositeLit) bool {
	typeName := typeExprName(cl.Type)
	return typeName == "Result"
}

func looksLikeReconcileMethod(fd *ast.FuncDecl) bool {
	if fd == nil || fd.Type == nil || fd.Type.Params == nil || fd.Type.Results == nil {
		return false
	}

	paramTypes := expandedFieldTypes(fd.Type.Params.List)
	if len(paramTypes) != 2 {
		return false
	}
	if !isContextType(paramTypes[0]) || !isRequestType(paramTypes[1]) {
		return false
	}

	resultTypes := expandedFieldTypes(fd.Type.Results.List)
	if len(resultTypes) != 2 {
		return false
	}

	firstResultType := typeExprName(resultTypes[0])
	if firstResultType != "Result" {
		return false
	}
	secondResultType := typeExprName(resultTypes[1])
	if secondResultType != "error" {
		return false
	}

	return true
}

func expandedFieldTypes(fields []*ast.Field) []ast.Expr {
	var out []ast.Expr
	for _, f := range fields {
		count := len(f.Names)
		if count == 0 {
			count = 1
		}
		for i := 0; i < count; i++ {
			out = append(out, f.Type)
		}
	}

	return out
}

func isContextType(expr ast.Expr) bool {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name == "Context"
	case *ast.SelectorExpr:
		return t.Sel.Name == "Context"
	case *ast.StarExpr:
		return isContextType(t.X)
	default:
		return false
	}
}

func isRequestType(expr ast.Expr) bool {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name == "Request"
	case *ast.SelectorExpr:
		return t.Sel.Name == "Request"
	case *ast.StarExpr:
		return isRequestType(t.X)
	default:
		return false
	}
}

func receiverNames(fd *ast.FuncDecl) map[string]bool {
	names := map[string]bool{}
	if fd == nil || fd.Recv == nil {
		return names
	}

	for _, field := range fd.Recv.List {
		for _, name := range field.Names {
			if name != nil && name.Name != "" && name.Name != "_" {
				names[name.Name] = true
			}
		}
	}

	return names
}

func isNilIdent(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == "nil"
}

type reconcileResultState struct {
	hasRequeue      bool
	hasRequeueAfter bool
}

func collectReconcileResultStates(fd *ast.FuncDecl) map[string]reconcileResultState {
	states := map[string]reconcileResultState{}

	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.DeclStmt:
			gd, ok := node.Decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				return true
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, name := range vs.Names {
					if !isResultType(vs.Type) {
						continue
					}
					states[name.Name] = reconcileResultState{}
					if i < len(vs.Values) {
						if state, ok := stateFromResultExpr(vs.Values[i], states); ok {
							states[name.Name] = state
						}
					}
				}
			}
		case *ast.AssignStmt:
			if len(node.Lhs) == 0 || len(node.Rhs) == 0 {
				return true
			}
			if len(node.Rhs) == 1 {
				for _, lhs := range node.Lhs {
					if sel, ok := lhs.(*ast.SelectorExpr); ok {
						resultVar, ok := sel.X.(*ast.Ident)
						if !ok {
							continue
						}
						state := states[resultVar.Name]
						switch sel.Sel.Name {
						case FieldRequeue:
							if v, ok := boolLiteralValue(node.Rhs[0]); ok {
								state.hasRequeue = v
							} else {
								state.hasRequeue = true
							}
						case FieldRequeueAfter:
							state.hasRequeueAfter = isActiveRequeueAfterExpr(node.Rhs[0])
						default:
							continue
						}
						states[resultVar.Name] = state
					}
				}
			}

			for i, lhs := range node.Lhs {
				lhsIdent, ok := lhs.(*ast.Ident)
				if !ok {
					continue
				}

				rhsIndex := i
				if rhsIndex >= len(node.Rhs) {
					rhsIndex = len(node.Rhs) - 1
				}
				if rhsIndex < 0 {
					continue
				}

				if state, ok := stateFromResultExpr(node.Rhs[rhsIndex], states); ok {
					states[lhsIdent.Name] = state
				}
			}
		}
		return true
	})

	return states
}

func isResultType(expr ast.Expr) bool {
	if expr == nil {
		return false
	}
	return typeExprName(expr) == "Result"
}

func stateFromResultExpr(
	expr ast.Expr,
	states map[string]reconcileResultState,
) (reconcileResultState, bool) {
	switch node := expr.(type) {
	case *ast.CompositeLit:
		if !isReconcileResult(node) {
			return reconcileResultState{}, false
		}
		state := reconcileResultState{}
		for _, elt := range node.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, ok := kv.Key.(*ast.Ident)
			if !ok {
				continue
			}
			switch key.Name {
			case FieldRequeue:
				if v, ok := boolLiteralValue(kv.Value); ok {
					state.hasRequeue = v
				} else {
					state.hasRequeue = true
				}
			case FieldRequeueAfter:
				state.hasRequeueAfter = isActiveRequeueAfterExpr(kv.Value)
			}
		}
		return state, true
	case *ast.Ident:
		state, ok := states[node.Name]
		return state, ok
	default:
		return reconcileResultState{}, false
	}
}

func resolveReconcileResultState(
	expr ast.Expr,
	states map[string]reconcileResultState,
) (reconcileResultState, bool) {
	state, ok := stateFromResultExpr(expr, states)
	if !ok {
		return reconcileResultState{}, false
	}
	return state, true
}

func boolLiteralValue(expr ast.Expr) (bool, bool) {
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return false, false
	}

	switch ident.Name {
	case "true":
		return true, true
	case "false":
		return false, true
	default:
		return false, false
	}
}

func isActiveRequeueAfterExpr(expr ast.Expr) bool {
	switch v := expr.(type) {
	case *ast.BasicLit:
		value := strings.TrimSpace(v.Value)
		if v.Kind == token.STRING {
			if unquoted, err := strconv.Unquote(value); err == nil {
				value = unquoted
			}
		}
		value = strings.TrimSpace(value)
		value = strings.ReplaceAll(value, "_", "")

		switch value {
		case "", "0", "0s", "0m", "0h", "0ms", "0us", "0ns":
			return false
		}

		if num, err := strconv.ParseFloat(value, 64); err == nil {
			return num != 0
		}

		return true
	case *ast.Ident:
		if v.Name == "nil" {
			return false
		}
		return true
	case *ast.UnaryExpr:
		return isActiveRequeueAfterExpr(v.X)
	default:
		// Unknown expressions are treated as active to avoid missing explicit requeue paths.
		return true
	}
}

func extractConditionTypeAndOG(expr ast.Expr) (string, bool) {
	if unary, ok := expr.(*ast.UnaryExpr); ok {
		expr = unary.X
	}

	cl, ok := expr.(*ast.CompositeLit)
	if !ok {
		return "", false
	}

	condType := ""
	hasOG := false

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
		case FieldType:
			if lit, ok := kv.Value.(*ast.BasicLit); ok {
				condType = strings.Trim(lit.Value, `"`)
			} else if sel, ok := kv.Value.(*ast.SelectorExpr); ok {
				condType = sel.Sel.Name
			} else if ident, ok := kv.Value.(*ast.Ident); ok {
				condType = ident.Name
			}
		case FieldObservedGeneration:
			hasOG = true
		}
	}

	return condType, hasOG
}

func rbacMarkerFromArgs(m Marker) RBACMarker {
	return RBACMarker{
		Verbs:         m.Args["verbs"],
		Resource:      m.Args["resources"],
		Group:         m.Args["groups"],
		ResourceNames: m.Args["resourceNames"],
		Namespace:     m.Args["namespace"],
		Line:          m.Line,
	}
}

func extractAPICalls(
	fd *ast.FuncDecl,
	fset *token.FileSet,
) []APICall {
	var calls []APICall
	clientMethods := map[string]bool{
		MethodGet: true, MethodList: true, MethodCreate: true, MethodUpdate: true,
		MethodDelete: true, MethodPatch: true,
	}

	ast.Inspect(fd.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		methodName := sel.Sel.Name

		// Check for Status().Update() or Status().Patch()
		if innerCall, ok := sel.X.(*ast.CallExpr); ok {
			if innerSel, ok := innerCall.Fun.(*ast.SelectorExpr); ok {
				if innerSel.Sel.Name == MethodStatus && (methodName == MethodUpdate || methodName == MethodPatch) {
					receiver := ""
					if id, ok := innerSel.X.(*ast.Ident); ok {
						receiver = id.Name
					}
					objType := ""
					if len(call.Args) >= 2 {
						objType = exprToString(call.Args[1])
					}
					calls = append(calls, APICall{
						Method:   "Status." + methodName,
						Receiver: receiver,
						ObjType:  objType,
						Line:     fset.Position(call.Pos()).Line,
					})
					return true
				}
			}
		}

		if !clientMethods[methodName] {
			return true
		}

		receiver := ""
		if id, ok := sel.X.(*ast.Ident); ok {
			receiver = id.Name
		}

		objType := ""
		// For Get(ctx, key, obj) the object is the 3rd arg
		// For Create/Update/Delete(ctx, obj) the object is the 2nd arg
		switch methodName {
		case MethodGet:
			if len(call.Args) >= 3 {
				objType = exprToString(call.Args[2])
			}
		default:
			if len(call.Args) >= 2 {
				objType = exprToString(call.Args[1])
			}
		}

		calls = append(calls, APICall{
			Method:   methodName,
			Receiver: receiver,
			ObjType:  objType,
			Line:     fset.Position(call.Pos()).Line,
		})

		return true
	})

	return calls
}

func extractOwnerRefOps(
	fd *ast.FuncDecl,
	fset *token.FileSet,
) []OwnerRefOp {
	var ops []OwnerRefOp

	ownerFuncs := map[string]string{
		FuncSetCtrlRef:        "owner-reference",
		FuncSetOwnerRef:       "owner-reference",
		FuncSetCtrlRefAlt:     "owner-reference",
		FuncAddFinalizer:      "finalizer",
		FuncRemoveFinalizer:   "finalizer",
		FuncContainsFinalizer: "finalizer",
	}

	ast.Inspect(fd.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		funcName := callFuncName(call)
		if opType, found := ownerFuncs[funcName]; found {
			ops = append(ops, OwnerRefOp{
				Type: opType,
				Call: funcName,
				Line: fset.Position(call.Pos()).Line,
			})
		}

		return true
	})

	return ops
}

// retrySpan represents the AST position range of a retry closure body.
type retrySpan struct {
	start     token.Pos
	end       token.Pos
	guardKind string
}

// retryFuncInfo maps retry function names to the argument index of the closure.
var retryFuncInfo = map[string]int{
	FuncRetryOnConflict: 1, // retry.RetryOnConflict(backoff, func() error { ... })
	FuncOnError:         2, // retry.OnError(backoff, retryable, func() error { ... })
}

func libraryImportAliases(file *ast.File) map[string]string {
	aliases := map[string]string{}
	if file == nil {
		return aliases
	}

	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}

		family := ""
		for prefix, value := range libraryImportPrefixes {
			if strings.HasPrefix(path, prefix) {
				family = value
				break
			}
		}
		if family == "" {
			continue
		}

		alias := importDefaultName(path)
		if imp.Name != nil {
			if imp.Name.Name == "." || imp.Name.Name == "_" {
				continue
			}
			alias = imp.Name.Name
		}
		if alias == "" {
			continue
		}
		aliases[alias] = family
	}

	return aliases
}

func importDefaultName(path string) string {
	if path == "" {
		return ""
	}
	idx := strings.LastIndex(path, "/")
	if idx == -1 {
		return path
	}
	return path[idx+1:]
}

func buildRepoCallGraph(
	pkgs []*packages.Package,
	repoPath string,
) *repoCallGraph {
	graph := &repoCallGraph{
		functions:    map[string]repoFunctionInfo{},
		edges:        map[string]map[string]struct{}{},
		invocations:  map[string][]LibraryInvocation{},
		reachability: map[string]map[string]bool{},
	}

	for _, pkg := range pkgs {
		for i, file := range pkg.Syntax {
			relPath := ""
			if i < len(pkg.CompiledGoFiles) {
				relPath, _ = filepath.Rel(repoPath, pkg.CompiledGoFiles[i])
			}

			for _, decl := range file.Decls {
				fd, ok := decl.(*ast.FuncDecl)
				if !ok || fd.Name == nil {
					continue
				}

				receiverKey := repoReceiverKey(pkg.PkgPath, receiverTypeName(fd))
				id := repoFunctionID(pkg.PkgPath, receiverKey, fd.Name.Name)
				graph.functions[id] = repoFunctionInfo{
					id:          id,
					pkg:         pkg,
					file:        file,
					relPath:     relPath,
					decl:        fd,
					name:        fd.Name.Name,
					receiverKey: receiverKey,
				}
			}
		}
	}

	for id, fn := range graph.functions {
		invocations := extractLibraryInvocations(fn.pkg, fn.file, fn.decl, fn.name)
		if len(invocations) > 0 {
			graph.invocations[id] = invocations
		}

		for _, calleeID := range collectRepoCallEdges(fn, graph.functions) {
			if graph.edges[id] == nil {
				graph.edges[id] = map[string]struct{}{}
			}
			graph.edges[id][calleeID] = struct{}{}
		}
	}

	return graph
}

func collectControllerLibraryInvocations(
	callGraph *repoCallGraph,
	index *packageIndex,
	rec reconcilerInfo,
) []LibraryInvocation {
	if callGraph == nil {
		return nil
	}

	reconcileID := repoFunctionID(
		index.pkgPath,
		repoReceiverKey(index.pkgPath, rec.name),
		FuncReconcile,
	)
	reachableNodes := callGraph.reachableFrom(reconcileID)

	includeNodes := map[string]bool{}
	for methodName := range index.methods[rec.name] {
		includeNodes[repoFunctionID(
			index.pkgPath,
			repoReceiverKey(index.pkgPath, rec.name),
			methodName,
		)] = true
	}
	for nodeID := range reachableNodes {
		includeNodes[nodeID] = true
	}

	var invocations []LibraryInvocation
	seen := map[string]bool{}
	for nodeID := range includeNodes {
		for _, invocation := range callGraph.invocations[nodeID] {
			invocation.InvokedInReconcileLoop = reachableNodes[nodeID]
			key := nodeID + "|" + invocation.Family + "|" + invocation.Method + "|" + invocation.Call + "|" + strconv.Itoa(invocation.Line)
			if seen[key] {
				continue
			}
			seen[key] = true
			invocations = append(invocations, invocation)
		}
	}

	return invocations
}

func (g *repoCallGraph) reachableFrom(startID string) map[string]bool {
	if g == nil {
		return map[string]bool{}
	}
	if reachable, ok := g.reachability[startID]; ok {
		return reachable
	}

	reachable := map[string]bool{}
	if _, ok := g.functions[startID]; !ok {
		g.reachability[startID] = reachable
		return reachable
	}

	queue := []string{startID}
	for len(queue) > 0 {
		nodeID := queue[0]
		queue = queue[1:]
		if reachable[nodeID] {
			continue
		}

		reachable[nodeID] = true
		for nextID := range g.edges[nodeID] {
			if !reachable[nextID] {
				queue = append(queue, nextID)
			}
		}
	}

	g.reachability[startID] = reachable
	return reachable
}

func collectRepoCallEdges(
	fn repoFunctionInfo,
	functions map[string]repoFunctionInfo,
) []string {
	var edges []string
	if fn.decl == nil || fn.decl.Body == nil {
		return edges
	}

	seen := map[string]bool{}
	recvNames := receiverNames(fn.decl)

	ast.Inspect(fn.decl.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		calleeID := resolveRepoCallTarget(fn, call, recvNames, functions)
		if calleeID == "" || calleeID == fn.id || seen[calleeID] {
			return true
		}
		seen[calleeID] = true
		edges = append(edges, calleeID)
		return true
	})

	return edges
}

func resolveRepoCallTarget(
	fn repoFunctionInfo,
	call *ast.CallExpr,
	recvNames map[string]bool,
	functions map[string]repoFunctionInfo,
) string {
	if obj := resolvedFuncObject(fn.pkg, call); obj != nil {
		calleeID := repoFunctionIDForObject(obj)
		if _, ok := functions[calleeID]; ok {
			return calleeID
		}
	}

	switch fun := call.Fun.(type) {
	case *ast.Ident:
		calleeID := repoFunctionID(fn.pkg.PkgPath, "", fun.Name)
		if _, ok := functions[calleeID]; ok {
			return calleeID
		}
	case *ast.SelectorExpr:
		recv, ok := fun.X.(*ast.Ident)
		if !ok || !recvNames[recv.Name] {
			return ""
		}
		calleeID := repoFunctionID(fn.pkg.PkgPath, fn.receiverKey, fun.Sel.Name)
		if _, ok := functions[calleeID]; ok {
			return calleeID
		}
	}

	return ""
}

func extractLibraryInvocations(
	pkg *packages.Package,
	file *ast.File,
	fd *ast.FuncDecl,
	methodName string,
) []LibraryInvocation {
	var invocations []LibraryInvocation
	if fd == nil || fd.Body == nil {
		return invocations
	}

	libraryAliases := libraryImportAliases(file)
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		family := libraryFamilyForCall(pkg, call, libraryAliases)
		if family == "" {
			return true
		}

		invocations = append(invocations, LibraryInvocation{
			Family: family,
			Call:   callFuncName(call),
			Method: methodName,
			Line:   pkg.Fset.Position(call.Pos()).Line,
		})
		return true
	})

	return invocations
}

func libraryFamilyForCall(
	pkg *packages.Package,
	call *ast.CallExpr,
	libraryAliases map[string]string,
) string {
	if obj := resolvedFuncObject(pkg, call); obj != nil && obj.Pkg() != nil {
		for prefix, family := range libraryImportPrefixes {
			if strings.HasPrefix(obj.Pkg().Path(), prefix) {
				return family
			}
		}
	}

	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	alias, ok := sel.X.(*ast.Ident)
	if !ok {
		return ""
	}

	return libraryAliases[alias.Name]
}

func resolvedFuncObject(
	pkg *packages.Package,
	call *ast.CallExpr,
) *types.Func {
	if pkg == nil || pkg.TypesInfo == nil || call == nil {
		return nil
	}

	switch fun := call.Fun.(type) {
	case *ast.Ident:
		if obj, ok := pkg.TypesInfo.Uses[fun].(*types.Func); ok {
			return obj
		}
	case *ast.SelectorExpr:
		if sel := pkg.TypesInfo.Selections[fun]; sel != nil {
			if obj, ok := sel.Obj().(*types.Func); ok {
				return obj
			}
		}
		if obj, ok := pkg.TypesInfo.Uses[fun.Sel].(*types.Func); ok {
			return obj
		}
	}

	return nil
}

func repoFunctionIDForObject(obj *types.Func) string {
	if obj == nil || obj.Pkg() == nil {
		return ""
	}

	receiverKey := ""
	if sig, ok := obj.Type().(*types.Signature); ok && sig.Recv() != nil {
		receiverKey = repoReceiverKeyFromType(sig.Recv().Type())
	}

	return repoFunctionID(obj.Pkg().Path(), receiverKey, obj.Name())
}

func repoFunctionID(
	pkgPath string,
	receiverKey string,
	funcName string,
) string {
	return pkgPath + "|" + receiverKey + "|" + funcName
}

func repoReceiverKey(
	pkgPath string,
	receiverName string,
) string {
	if receiverName == "" {
		return ""
	}
	return pkgPath + "." + receiverName
}

func repoReceiverKeyFromType(t types.Type) string {
	for {
		ptr, ok := t.(*types.Pointer)
		if !ok {
			break
		}
		t = ptr.Elem()
	}

	named, ok := t.(*types.Named)
	if !ok {
		return ""
	}
	obj := named.Obj()
	if obj == nil {
		return ""
	}
	if obj.Pkg() == nil {
		return obj.Name()
	}

	return obj.Pkg().Path() + "." + obj.Name()
}

func retryImportAliases(file *ast.File) map[string]struct{} {
	aliases := map[string]struct{}{}
	if file == nil {
		return aliases
	}

	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || path != "k8s.io/client-go/util/retry" {
			continue
		}

		alias := "retry"
		if imp.Name != nil {
			// Dot or blank imports are not usable selector receivers.
			if imp.Name.Name == "." || imp.Name.Name == "_" {
				continue
			}
			alias = imp.Name.Name
		}
		aliases[alias] = struct{}{}
	}

	return aliases
}

// collectRetrySpans finds retry wrapper calls and returns the AST spans of their closures.
func collectRetrySpans(
	fd *ast.FuncDecl,
	retryAliases map[string]struct{},
) []retrySpan {
	var spans []retrySpan

	if fd.Body == nil {
		return spans
	}

	ast.Inspect(fd.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		funcName, closureIdx := matchRetryCall(call, retryAliases)
		if funcName == "" {
			return true
		}

		if closureIdx >= len(call.Args) {
			return true
		}

		funcLit, ok := call.Args[closureIdx].(*ast.FuncLit)
		if !ok || funcLit.Body == nil {
			return true
		}

		spans = append(spans, retrySpan{
			start:     funcLit.Body.Pos(),
			end:       funcLit.Body.End(),
			guardKind: funcName,
		})

		return true
	})

	return spans
}

// extractRetryOps extracts retry wrapper calls from a function body.
func extractRetryOps(
	fd *ast.FuncDecl,
	fset *token.FileSet,
	methodName string,
	retryAliases map[string]struct{},
) []RetryOp {
	var ops []RetryOp

	if fd.Body == nil {
		return ops
	}

	ast.Inspect(fd.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		funcName, closureIdx := matchRetryCall(call, retryAliases)
		if funcName == "" {
			return true
		}

		op := RetryOp{
			Function: funcName,
			Method:   methodName,
			Line:     fset.Position(call.Pos()).Line,
		}

		// Extract backoff kind from first argument.
		if len(call.Args) > 0 {
			op.BackoffKind = classifyBackoff(call.Args[0])
		}

		// Inspect closure body for wrapped API calls.
		if closureIdx < len(call.Args) {
			if funcLit, ok := call.Args[closureIdx].(*ast.FuncLit); ok && funcLit.Body != nil {
				op.WrappedCalls, op.WrapsStatusUpdate = inspectClosureForCalls(funcLit.Body)
			}
		}

		ops = append(ops, op)
		return true
	})

	return ops
}

// matchRetryCall checks if a CallExpr is a retry function call.
// Returns the function name and the argument index of the closure, or ("", 0) if not a match.
func matchRetryCall(
	call *ast.CallExpr,
	retryAliases map[string]struct{},
) (string, int) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", 0
	}
	recv, ok := sel.X.(*ast.Ident)
	if !ok {
		return "", 0
	}
	if _, ok := retryAliases[recv.Name]; !ok {
		return "", 0
	}

	name := sel.Sel.Name
	closureIdx, ok := retryFuncInfo[name]
	if !ok {
		return "", 0
	}

	return name, closureIdx
}

// classifyBackoff determines the backoff kind from the first argument to a retry call.
func classifyBackoff(expr ast.Expr) string {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return BackoffCustom
	}

	switch sel.Sel.Name {
	case BackoffDefaultRetry:
		return BackoffDefaultRetry
	case BackoffDefaultBackoff:
		return BackoffDefaultBackoff
	default:
		return BackoffCustom
	}
}

// inspectClosureForCalls looks inside a retry closure body for API calls.
func inspectClosureForCalls(body *ast.BlockStmt) ([]string, bool) {
	var calls []string
	wrapsStatus := false

	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		methodName := sel.Sel.Name

		// Check for Status().Update() or Status().Patch() chain.
		if methodName == MethodUpdate || methodName == MethodPatch {
			if innerCall, ok := sel.X.(*ast.CallExpr); ok {
				if innerSel, ok := innerCall.Fun.(*ast.SelectorExpr); ok {
					if innerSel.Sel.Name == MethodStatus {
						callName := MethodStatus + "." + methodName
						calls = append(calls, callName)
						wrapsStatus = true
						return true
					}
				}
			}
		}

		// Check for direct write ops (Create, Update, Patch, Delete).
		if writeMethodSet[methodName] {
			calls = append(calls, methodName)
		}

		return true
	})

	return calls, wrapsStatus
}

func extractStatusUpdateSites(
	fd *ast.FuncDecl,
	fset *token.FileSet,
	methodName string,
	spans []retrySpan,
) []StatusUpdateSite {
	var sites []StatusUpdateSite

	if fd.Body == nil {
		return sites
	}

	ast.Inspect(fd.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Check for Status().Update() / Status().Patch() patterns.
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || (sel.Sel.Name != MethodUpdate && sel.Sel.Name != MethodPatch) {
			return true
		}

		innerCall, ok := sel.X.(*ast.CallExpr)
		if !ok {
			return true
		}

		innerSel, ok := innerCall.Fun.(*ast.SelectorExpr)
		if !ok || innerSel.Sel.Name != MethodStatus {
			return true
		}

		site := StatusUpdateSite{
			Method: methodName,
			Line:   fset.Position(call.Pos()).Line,
		}

		// Check if this call is inside a retry span.
		pos := call.Pos()
		for _, span := range spans {
			if pos >= span.start && pos <= span.end {
				site.IsGuarded = true
				site.GuardKind = span.guardKind
				break
			}
		}

		sites = append(sites, site)

		return true
	})

	return sites
}

func extractEventUsages(
	fd *ast.FuncDecl,
	fset *token.FileSet,
) []EventUsage {
	var events []EventUsage
	eventMethods := map[string]bool{
		MethodEvent: true, MethodEventf: true,
	}

	ast.Inspect(fd.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || !eventMethods[sel.Sel.Name] {
			return true
		}

		receiver := ""
		if id, ok := sel.X.(*ast.Ident); ok {
			receiver = id.Name
		}

		events = append(events, EventUsage{
			Receiver: receiver,
			Method:   sel.Sel.Name,
			Line:     fset.Position(call.Pos()).Line,
		})

		return true
	})

	return events
}

func extractNotFoundHandlers(
	fd *ast.FuncDecl,
	fset *token.FileSet,
) []NotFoundHandling {
	var handlers []NotFoundHandling

	ast.Inspect(fd.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		funcName := callFuncName(call)

		switch {
		case strings.HasSuffix(funcName, FuncIsNotFound):
			handlers = append(handlers, NotFoundHandling{
				Pattern: FuncIsNotFound,
				Line:    fset.Position(call.Pos()).Line,
			})
		case strings.HasSuffix(funcName, FuncIgnoreNotFound):
			handlers = append(handlers, NotFoundHandling{
				Pattern: FuncIgnoreNotFound,
				Line:    fset.Position(call.Pos()).Line,
			})
		}

		return true
	})

	return handlers
}

func extractPredicateUsages(
	index *packageIndex,
	reconcilerName string,
) []PredicateUsage {
	var predicates []PredicateUsage

	setupMethod, ok := index.method(reconcilerName, FuncSetupWithManager)
	if !ok || setupMethod.decl.Body == nil {
		return predicates
	}

	ast.Inspect(setupMethod.decl.Body, func(n ast.Node) bool {
		// Look for predicate.XxxPredicate{} or predicate.Funcs{} usage
		cl, ok := n.(*ast.CompositeLit)
		if ok {
			typeName := typeExprName(cl.Type)
			if typeName != "" {
				// Check if it looks like a predicate type
				if sel, ok := cl.Type.(*ast.SelectorExpr); ok {
					if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "predicate" {
						predicates = append(predicates, PredicateUsage{
							Name: typeName,
							Line: index.fset.Position(cl.Pos()).Line,
						})
					}
				}
			}
		}

		// Also look for WithEventFilter calls
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if sel.Sel.Name == FuncWithEventFilter {
			predicates = append(predicates, PredicateUsage{
				Name: "WithEventFilter",
				Line: index.fset.Position(call.Pos()).Line,
			})
		}

		return true
	})

	return predicates
}

func exprToString(expr ast.Expr) string {
	if unary, ok := expr.(*ast.UnaryExpr); ok {
		return "&" + exprToString(unary.X)
	}
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		return selectorString(sel)
	}
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	if cl, ok := expr.(*ast.CompositeLit); ok {
		return typeExprName(cl.Type) + "{}"
	}

	return "?"
}

func inferGroupVersion(
	pkgs []*packages.Package,
	kind string,
	repoPath string,
) (string, string) {
	for _, pkg := range pkgs {
		relPkgPath, _ := filepath.Rel(repoPath, pkgDir(pkg))
		if !isAPIPackage(relPkgPath) {
			continue
		}

		if !packageHasRootKind(pkg, kind) {
			continue
		}

		group := extractGroupName(pkg)
		version := extractVersionFromPkgPath(pkg.PkgPath)
		if group != "" || version != "" {
			return group, version
		}
	}

	return "", ""
}

func packageHasRootKind(pkg *packages.Package, kind string) bool {
	for _, file := range pkg.Syntax {
		found := false

		ast.Inspect(file, func(n ast.Node) bool {
			gd, ok := n.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				return true
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || ts.Name.Name != kind {
					continue
				}
				doc := DocOrNearby(file, pkg.Fset, ts.Pos(), ts.Doc)
				for _, m := range ExtractMarkersFromDoc(doc, pkg.Fset) {
					if m.Name == MarkerObjectRoot {
						found = true
						return false
					}
				}
			}
			return true
		})

		if found {
			return true
		}
	}

	return false
}
