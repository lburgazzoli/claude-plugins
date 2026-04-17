package extractor

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"golang.org/x/tools/go/packages"
)

func testdataDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "testdata", "simple-operator")
}

func loadTestPackages(t *testing.T) ([]*packages.Package, string) {
	t.Helper()

	dir := testdataDir()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Skipf("testdata not found: %s", dir)
	}

	cfg := &packages.Config{
		Mode: packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedDeps |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedImports |
			packages.NeedName,
		Dir: dir,
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		t.Fatalf("loading packages: %v", err)
	}

	return pkgs, dir
}

func loadPackagesFromDir(t *testing.T, dir string) []*packages.Package {
	t.Helper()

	cfg := &packages.Config{
		Mode: packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedDeps |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedImports |
			packages.NeedName,
		Dir: dir,
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		t.Fatalf("loading packages: %v", err)
	}

	return pkgs
}

func writeTestFile(t *testing.T, root string, relPath string, content string) {
	t.Helper()

	path := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", relPath, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", relPath, err)
	}
}

func TestExtractControllers(t *testing.T) {
	pkgs, repoPath := loadTestPackages(t)

	facts := ExtractControllers(pkgs, repoPath)
	if len(facts) == 0 {
		t.Fatal("expected at least one controller fact")
	}

	// Find the FooReconciler fact
	var controllerFact *Fact
	for i := range facts {
		if facts[i].Kind == "controller" {
			controllerFact = &facts[i]
			break
		}
	}

	if controllerFact == nil {
		t.Fatal("expected a controller fact")
	}

	data, ok := controllerFact.Data.(ControllerData)
	if !ok {
		t.Fatalf("expected ControllerData, got %T", controllerFact.Data)
	}

	if data.Name != "FooReconciler" {
		t.Errorf("expected FooReconciler, got %s", data.Name)
	}

	if data.Reconciles.Kind != "Foo" {
		t.Errorf("expected Foo, got %s", data.Reconciles.Kind)
	}

	if len(data.RBACMarkers) < 3 {
		t.Errorf("expected at least 3 RBAC markers, got %d", len(data.RBACMarkers))
	}

	if len(data.Owns) != 2 {
		t.Errorf("expected 2 owns, got %d: %v", len(data.Owns), data.Owns)
	}

	if len(data.FinalizerOps) != 2 {
		t.Errorf("expected 2 finalizer ops, got %d", len(data.FinalizerOps))
	}

	if len(data.StatusConditionSets) < 2 {
		t.Errorf("expected at least 2 status conditions, got %d", len(data.StatusConditionSets))
	}

	if len(data.RequeueOps) < 1 {
		t.Errorf("expected at least 1 requeue op, got %d", len(data.RequeueOps))
	}

	if len(data.ErrorReturns) < 1 {
		t.Errorf("expected at least 1 error return, got %d", len(data.ErrorReturns))
	}
}

func TestExtractCRDVersions(t *testing.T) {
	pkgs, repoPath := loadTestPackages(t)

	facts := ExtractCRDVersions(pkgs, repoPath)
	if len(facts) == 0 {
		t.Fatal("expected at least one CRD version fact")
	}

	var crdFact *Fact
	for i := range facts {
		if facts[i].Kind == "crd_version" {
			crdFact = &facts[i]
			break
		}
	}

	if crdFact == nil {
		t.Fatal("expected a crd_version fact")
	}

	data, ok := crdFact.Data.(CRDVersionData)
	if !ok {
		t.Fatalf("expected CRDVersionData, got %T", crdFact.Data)
	}

	if data.Kind != "Foo" {
		t.Errorf("expected Foo, got %s", data.Kind)
	}

	if data.Version != "v1alpha1" {
		t.Errorf("expected v1alpha1, got %s", data.Version)
	}

	if !data.Storage {
		t.Error("expected storage=true")
	}
}

func TestExtractSchemeRegistrations(t *testing.T) {
	pkgs, repoPath := loadTestPackages(t)

	facts := ExtractSchemeRegistrations(pkgs, repoPath)
	if len(facts) == 0 {
		t.Fatal("expected at least one scheme registration fact")
	}

	data, ok := facts[0].Data.(SchemeRegistrationData)
	if !ok {
		t.Fatalf("expected SchemeRegistrationData, got %T", facts[0].Data)
	}

	if data.Call != "AddToScheme" {
		t.Errorf("expected AddToScheme, got %s", data.Call)
	}
}

func TestExtractControllers_AcrossFilesAndKindInference(t *testing.T) {
	repoRoot := t.TempDir()

	controllerPkg := newSyntheticPackage(
		t,
		repoRoot,
		"example.com/project/controllers",
		map[string]string{
			"controllers/types.go": `package controllers
type FooReconciler struct{}
`,
			"controllers/reconcile.go": `package controllers
// +kubebuilder:rbac:groups=example.com,resources=foos,verbs=get
func (r *FooReconciler) Reconcile(ctx Context, req Request) (Result, error) { return Result{}, nil }
type Context struct{}
type Request struct{}
type Result struct {
	Requeue bool
	RequeueAfter int
}
`,
			"controllers/setup.go": `package controllers
var predicate predicatePkg
type predicatePkg struct{}
type Funcs struct{}
func (predicatePkg) Funcs() Funcs { return Funcs{} }

type Owned struct{}
type builder struct{}
var b builder

func (builder) Owns(any) builder { return builder{} }
func (builder) WithEventFilter(any) builder { return builder{} }
func (builder) Complete(any) error { return nil }

func (r *FooReconciler) SetupWithManager(mgr any) error {
	return b.Owns(&Owned{}).WithEventFilter(predicate.Funcs{}).Complete(r)
}
`,
		},
	)

	apiPkg := newSyntheticPackage(
		t,
		repoRoot,
		"example.com/project/api/v1alpha1",
		map[string]string{
			"api/v1alpha1/doc.go": `// +groupName=example.com
package v1alpha1
`,
			"api/v1alpha1/foo_types.go": `package v1alpha1
// +kubebuilder:object:root=true
type Foo struct{}
`,
		},
	)

	facts := ExtractControllers([]*packages.Package{controllerPkg, apiPkg}, repoRoot)
	if len(facts) != 1 {
		t.Fatalf("expected 1 controller fact, got %d", len(facts))
	}

	data, ok := facts[0].Data.(ControllerData)
	if !ok {
		t.Fatalf("expected ControllerData, got %T", facts[0].Data)
	}

	if data.Reconciles.Group != "example.com" || data.Reconciles.Version != "v1alpha1" {
		t.Fatalf("expected example.com/v1alpha1, got %s/%s", data.Reconciles.Group, data.Reconciles.Version)
	}
	if len(data.Owns) != 1 || data.Owns[0] != "Owned" {
		t.Fatalf("expected Owns=[Owned], got %v", data.Owns)
	}
	hasWithEventFilter := false
	for _, p := range data.PredicateUsages {
		if p.Name == "WithEventFilter" {
			hasWithEventFilter = true
		}
	}
	if !hasWithEventFilter {
		t.Fatalf("expected WithEventFilter predicate usage, got %v", data.PredicateUsages)
	}
}

func TestExtractControllers_RequeueVarsAndStatusWriteChain(t *testing.T) {
	repoRoot := t.TempDir()

	controllerPkg := newSyntheticPackage(
		t,
		repoRoot,
		"example.com/project/controllers",
		map[string]string{
			"controllers/reconcile.go": `package controllers
import retry "k8s.io/client-go/util/retry"

type Context struct{}
type Request struct{}
type Result struct {
	Requeue bool
	RequeueAfter int
}

type statusWriter struct{}
func (statusWriter) Update(any, any) error { return nil }

type FooReconciler struct{}
func (r *FooReconciler) Status() statusWriter { return statusWriter{} }

func (r *FooReconciler) Reconcile(ctx Context, req Request) (Result, error) {
	var err error
	result := Result{}
	result.Requeue = true
	result.RequeueAfter = 30
	if err := r.Status().Update(nil, nil); err != nil {
		return result, err
	}
	if err != nil {
		return result, err
	}
	return result, nil
}
`,
		},
	)

	facts := ExtractControllers([]*packages.Package{controllerPkg}, repoRoot)
	if len(facts) != 1 {
		t.Fatalf("expected 1 controller fact, got %d", len(facts))
	}

	data, ok := facts[0].Data.(ControllerData)
	if !ok {
		t.Fatalf("expected ControllerData, got %T", facts[0].Data)
	}

	hasStatusUpdateWrite := false
	for _, op := range data.ExternalWriteOps {
		if op.Call == "r.Status().Update" {
			hasStatusUpdateWrite = true
		}
	}
	if !hasStatusUpdateWrite {
		t.Fatalf("expected chained status write op, got %v", data.ExternalWriteOps)
	}

	hasRequeue := false
	hasRequeueAfter := false
	for _, op := range data.RequeueOps {
		if op.Kind == "Requeue" {
			hasRequeue = true
		}
		if op.Kind == "RequeueAfter" {
			hasRequeueAfter = true
		}
	}
	if !hasRequeue || !hasRequeueAfter {
		t.Fatalf("expected requeue and requeueAfter ops, got %v", data.RequeueOps)
	}

	hasErrorWithRequeue := false
	for _, er := range data.ErrorReturns {
		if er.HasRequeue {
			hasErrorWithRequeue = true
		}
	}
	if !hasErrorWithRequeue {
		t.Fatalf("expected error return with requeue=true, got %v", data.ErrorReturns)
	}
}

func TestExtractControllers_DoesNotClassifyNonControllerReconcileMethod(t *testing.T) {
	repoRoot := t.TempDir()

	nonControllerPkg := newSyntheticPackage(
		t,
		repoRoot,
		"example.com/project/pkg",
		map[string]string{
			"pkg/service.go": `package pkg
type Result struct{}
type ReconcileService struct{}

func (s *ReconcileService) Reconcile() (Result, error) {
	return Result{}, nil
}
`,
		},
	)

	facts := ExtractControllers([]*packages.Package{nonControllerPkg}, repoRoot)
	if len(facts) != 0 {
		t.Fatalf("expected 0 controller facts, got %d: %#v", len(facts), facts)
	}
}

func TestExtractControllers_DoesNotClassifyInvalidReconcileSignature(t *testing.T) {
	repoRoot := t.TempDir()

	controllerPkg := newSyntheticPackage(
		t,
		repoRoot,
		"example.com/project/controllers",
		map[string]string{
			"controllers/reconcile.go": `package controllers
type Context struct{}
type Request struct{}
type Result struct{}

type FooReconciler struct{}
func (r *FooReconciler) SetupWithManager(mgr any) error { return nil }

// Missing request parameter shape: should not be classified as a reconciler.
func (r *FooReconciler) Reconcile(ctx Context) (Result, error) {
	return Result{}, nil
}
`,
		},
	)

	facts := ExtractControllers([]*packages.Package{controllerPkg}, repoRoot)
	if len(facts) != 0 {
		t.Fatalf("expected 0 controller facts for invalid signature, got %d: %#v", len(facts), facts)
	}
}

func TestExtractControllers_RequeueFalseOverridesTrue(t *testing.T) {
	repoRoot := t.TempDir()

	controllerPkg := newSyntheticPackage(
		t,
		repoRoot,
		"example.com/project/controllers",
		map[string]string{
			"controllers/reconcile.go": `package controllers
type Result struct {
	Requeue bool
}
type FooReconciler struct{}

func (r *FooReconciler) Reconcile(ctx Context, req Request) (Result, error) {
	result := Result{}
	result.Requeue = true
	result.Requeue = false
	return result, nil
}
`,
		},
	)

	facts := ExtractControllers([]*packages.Package{controllerPkg}, repoRoot)
	if len(facts) != 1 {
		t.Fatalf("expected 1 controller fact, got %d", len(facts))
	}

	data, ok := facts[0].Data.(ControllerData)
	if !ok {
		t.Fatalf("expected ControllerData, got %T", facts[0].Data)
	}
	if len(data.RequeueOps) != 0 {
		t.Fatalf("expected no requeue ops when final value is false, got %v", data.RequeueOps)
	}
}

func TestExtractControllers_RequeueAfterZeroIsNotActive(t *testing.T) {
	repoRoot := t.TempDir()

	controllerPkg := newSyntheticPackage(
		t,
		repoRoot,
		"example.com/project/controllers",
		map[string]string{
			"controllers/reconcile.go": `package controllers
type Result struct {
	RequeueAfter int
}
type FooReconciler struct{}

func (r *FooReconciler) Reconcile(ctx Context, req Request) (Result, error) {
	result := Result{}
	result.RequeueAfter = 0
	return result, nil
}
`,
		},
	)

	facts := ExtractControllers([]*packages.Package{controllerPkg}, repoRoot)
	if len(facts) != 1 {
		t.Fatalf("expected 1 controller fact, got %d", len(facts))
	}

	data, ok := facts[0].Data.(ControllerData)
	if !ok {
		t.Fatalf("expected ControllerData, got %T", facts[0].Data)
	}
	if len(data.RequeueOps) != 0 {
		t.Fatalf("expected no requeue ops when RequeueAfter is zero, got %v", data.RequeueOps)
	}
}

func TestExtractControllers_RetryOnConflictWrapsStatusUpdate(t *testing.T) {
	repoRoot := t.TempDir()

	controllerPkg := newSyntheticPackage(
		t,
		repoRoot,
		"example.com/project/controllers",
		map[string]string{
			"controllers/reconcile.go": `package controllers

import "k8s.io/client-go/util/retry"

type Context struct{}
type Request struct{}
type Result struct {
	Requeue bool
	RequeueAfter int
}

type statusWriter struct{}
func (statusWriter) Update(any, any) error { return nil }

type FooReconciler struct{}
func (r *FooReconciler) Status() statusWriter { return statusWriter{} }

func (r *FooReconciler) Reconcile(ctx Context, req Request) (Result, error) {
	retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return r.Status().Update(nil, nil)
	})
	return Result{}, nil
}
`,
		},
	)

	facts := ExtractControllers([]*packages.Package{controllerPkg}, repoRoot)
	if len(facts) != 1 {
		t.Fatalf("expected 1 controller fact, got %d", len(facts))
	}

	data, ok := facts[0].Data.(ControllerData)
	if !ok {
		t.Fatalf("expected ControllerData, got %T", facts[0].Data)
	}

	if len(data.RetryOps) != 1 {
		t.Fatalf("expected 1 retry op, got %d: %+v", len(data.RetryOps), data.RetryOps)
	}
	if data.RetryOps[0].Function != "RetryOnConflict" {
		t.Errorf("expected RetryOnConflict, got %s", data.RetryOps[0].Function)
	}
	if data.RetryOps[0].BackoffKind != "DefaultRetry" {
		t.Errorf("expected DefaultRetry backoff, got %s", data.RetryOps[0].BackoffKind)
	}
	if !data.RetryOps[0].WrapsStatusUpdate {
		t.Error("expected wraps_status_update=true")
	}

	if len(data.StatusUpdateSites) != 1 {
		t.Fatalf("expected 1 status update site, got %d: %+v", len(data.StatusUpdateSites), data.StatusUpdateSites)
	}
	if !data.StatusUpdateSites[0].IsGuarded {
		t.Error("expected status update to be guarded")
	}
	if data.StatusUpdateSites[0].GuardKind != "RetryOnConflict" {
		t.Errorf("expected guard kind RetryOnConflict, got %s", data.StatusUpdateSites[0].GuardKind)
	}
}

func TestExtractControllers_RetryInHelperMethod(t *testing.T) {
	repoRoot := t.TempDir()

	controllerPkg := newSyntheticPackage(
		t,
		repoRoot,
		"example.com/project/controllers",
		map[string]string{
			"controllers/reconcile.go": `package controllers
import retry "k8s.io/client-go/util/retry"

type Context struct{}
type Request struct{}
type Result struct {
	Requeue bool
	RequeueAfter int
}

type statusWriter struct{}
func (statusWriter) Update(any, any) error { return nil }

type FooReconciler struct{}
func (r *FooReconciler) Status() statusWriter { return statusWriter{} }

func (r *FooReconciler) Reconcile(ctx Context, req Request) (Result, error) {
	return Result{}, nil
}

func (r *FooReconciler) updateStatus() error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return r.Status().Update(nil, nil)
	})
}
`,
		},
	)

	facts := ExtractControllers([]*packages.Package{controllerPkg}, repoRoot)
	if len(facts) != 1 {
		t.Fatalf("expected 1 controller fact, got %d", len(facts))
	}

	data, ok := facts[0].Data.(ControllerData)
	if !ok {
		t.Fatalf("expected ControllerData, got %T", facts[0].Data)
	}

	if len(data.RetryOps) != 1 {
		t.Fatalf("expected 1 retry op, got %d", len(data.RetryOps))
	}
	if data.RetryOps[0].Method != "updateStatus" {
		t.Errorf("expected method updateStatus, got %s", data.RetryOps[0].Method)
	}

	if len(data.StatusUpdateSites) != 1 {
		t.Fatalf("expected 1 status update site, got %d", len(data.StatusUpdateSites))
	}
	if !data.StatusUpdateSites[0].IsGuarded {
		t.Error("expected status update to be guarded")
	}
	if data.StatusUpdateSites[0].Method != "updateStatus" {
		t.Errorf("expected method updateStatus, got %s", data.StatusUpdateSites[0].Method)
	}
}

func TestExtractControllers_BareStatusUpdateNotGuarded(t *testing.T) {
	repoRoot := t.TempDir()

	controllerPkg := newSyntheticPackage(
		t,
		repoRoot,
		"example.com/project/controllers",
		map[string]string{
			"controllers/reconcile.go": `package controllers
import retry "k8s.io/client-go/util/retry"

type Context struct{}
type Request struct{}
type Result struct {
	Requeue bool
	RequeueAfter int
}

type statusWriter struct{}
func (statusWriter) Update(any, any) error { return nil }

type FooReconciler struct{}
func (r *FooReconciler) Status() statusWriter { return statusWriter{} }

func (r *FooReconciler) Reconcile(ctx Context, req Request) (Result, error) {
	r.Status().Update(nil, nil)
	return Result{}, nil
}
`,
		},
	)

	facts := ExtractControllers([]*packages.Package{controllerPkg}, repoRoot)
	if len(facts) != 1 {
		t.Fatalf("expected 1 controller fact, got %d", len(facts))
	}

	data, ok := facts[0].Data.(ControllerData)
	if !ok {
		t.Fatalf("expected ControllerData, got %T", facts[0].Data)
	}

	if len(data.StatusUpdateSites) != 1 {
		t.Fatalf("expected 1 status update site, got %d", len(data.StatusUpdateSites))
	}
	if data.StatusUpdateSites[0].IsGuarded {
		t.Error("expected status update NOT to be guarded")
	}
	if data.StatusUpdateSites[0].GuardKind != "" {
		t.Errorf("expected empty guard kind, got %s", data.StatusUpdateSites[0].GuardKind)
	}
	if len(data.RetryOps) != 0 {
		t.Errorf("expected 0 retry ops, got %d", len(data.RetryOps))
	}
}

func TestExtractControllers_StatusPatchSiteDetected(t *testing.T) {
	repoRoot := t.TempDir()

	controllerPkg := newSyntheticPackage(
		t,
		repoRoot,
		"example.com/project/controllers",
		map[string]string{
			"controllers/reconcile.go": `package controllers
import retry "k8s.io/client-go/util/retry"

type Context struct{}
type Request struct{}
type Result struct {
	Requeue bool
	RequeueAfter int
}

type statusWriter struct{}
func (statusWriter) Update(any, any) error { return nil }
func (statusWriter) Patch(any, any, any) error { return nil }

type FooReconciler struct{}
func (r *FooReconciler) Status() statusWriter { return statusWriter{} }

func (r *FooReconciler) Reconcile(ctx Context, req Request) (Result, error) {
	r.Status().Patch(nil, nil, nil)
	return Result{}, nil
}
`,
		},
	)

	facts := ExtractControllers([]*packages.Package{controllerPkg}, repoRoot)
	if len(facts) != 1 {
		t.Fatalf("expected 1 controller fact, got %d", len(facts))
	}

	data, ok := facts[0].Data.(ControllerData)
	if !ok {
		t.Fatalf("expected ControllerData, got %T", facts[0].Data)
	}
	if len(data.StatusUpdateSites) != 1 {
		t.Fatalf("expected 1 status update site for patch call, got %d", len(data.StatusUpdateSites))
	}
	if data.StatusUpdateSites[0].IsGuarded {
		t.Error("expected status patch call to be unguarded")
	}
}

func TestExtractControllers_IgnoresRetryLikeCallsOutsideRetryPackage(t *testing.T) {
	repoRoot := t.TempDir()

	controllerPkg := newSyntheticPackage(
		t,
		repoRoot,
		"example.com/project/controllers",
		map[string]string{
			"controllers/reconcile.go": `package controllers
type Context struct{}
type Request struct{}
type Result struct {
	Requeue bool
	RequeueAfter int
}

type statusWriter struct{}
func (statusWriter) Update(any, any) error { return nil }

type helperRetryPkg struct{}
var helperRetry helperRetryPkg
func (helperRetryPkg) RetryOnConflict(any, func() error) error { return nil }

type FooReconciler struct{}
func (r *FooReconciler) Status() statusWriter { return statusWriter{} }

func (r *FooReconciler) Reconcile(ctx Context, req Request) (Result, error) {
	helperRetry.RetryOnConflict(nil, func() error {
		return r.Status().Update(nil, nil)
	})
	return Result{}, nil
}
`,
		},
	)

	facts := ExtractControllers([]*packages.Package{controllerPkg}, repoRoot)
	if len(facts) != 1 {
		t.Fatalf("expected 1 controller fact, got %d", len(facts))
	}

	data, ok := facts[0].Data.(ControllerData)
	if !ok {
		t.Fatalf("expected ControllerData, got %T", facts[0].Data)
	}
	if len(data.RetryOps) != 0 {
		t.Fatalf("expected no retry ops for non-retry package calls, got %+v", data.RetryOps)
	}
	if len(data.StatusUpdateSites) != 1 {
		t.Fatalf("expected 1 status update site, got %d", len(data.StatusUpdateSites))
	}
	if data.StatusUpdateSites[0].IsGuarded {
		t.Fatalf("expected status update inside non-retry helper call to be unguarded")
	}
}

func TestExtractControllers_LibraryInvocationInReconcile(t *testing.T) {
	repoRoot := t.TempDir()

	controllerPkg := newSyntheticPackage(
		t,
		repoRoot,
		"example.com/project/controllers",
		map[string]string{
			"controllers/reconcile.go": `package controllers
import "helm.sh/helm/v3/pkg/action"

type Context struct{}
type Request struct{}
type Result struct{}

type FooReconciler struct{}

func (r *FooReconciler) Reconcile(ctx Context, req Request) (Result, error) {
	_ = action.NewInstall(nil)
	return Result{}, nil
}
`,
		},
	)

	facts := ExtractControllers([]*packages.Package{controllerPkg}, repoRoot)
	if len(facts) != 1 {
		t.Fatalf("expected 1 controller fact, got %d", len(facts))
	}

	data, ok := facts[0].Data.(ControllerData)
	if !ok {
		t.Fatalf("expected ControllerData, got %T", facts[0].Data)
	}
	if len(data.LibraryInvocations) != 1 {
		t.Fatalf("expected 1 library invocation, got %+v", data.LibraryInvocations)
	}
	got := data.LibraryInvocations[0]
	if got.Family != "helm" || got.Call != "action.NewInstall" || got.Method != "Reconcile" {
		t.Fatalf("unexpected library invocation: %+v", got)
	}
	if !got.InvokedInReconcileLoop {
		t.Fatalf("expected invocation to be marked as in reconcile loop: %+v", got)
	}
}

func TestExtractControllers_LibraryInvocationInReachableHelper(t *testing.T) {
	repoRoot := t.TempDir()

	controllerPkg := newSyntheticPackage(
		t,
		repoRoot,
		"example.com/project/controllers",
		map[string]string{
			"controllers/reconcile.go": `package controllers
import "sigs.k8s.io/kustomize/api/krusty"

type Context struct{}
type Request struct{}
type Result struct{}

type FooReconciler struct{}

func (r *FooReconciler) Reconcile(ctx Context, req Request) (Result, error) {
	return Result{}, r.render()
}

func (r *FooReconciler) render() error {
	_ = krusty.MakeKustomizer(nil)
	return nil
}
`,
		},
	)

	facts := ExtractControllers([]*packages.Package{controllerPkg}, repoRoot)
	if len(facts) != 1 {
		t.Fatalf("expected 1 controller fact, got %d", len(facts))
	}

	data, ok := facts[0].Data.(ControllerData)
	if !ok {
		t.Fatalf("expected ControllerData, got %T", facts[0].Data)
	}
	if len(data.LibraryInvocations) != 1 {
		t.Fatalf("expected 1 library invocation, got %+v", data.LibraryInvocations)
	}
	got := data.LibraryInvocations[0]
	if got.Family != "kustomize" || got.Call != "krusty.MakeKustomizer" || got.Method != "render" {
		t.Fatalf("unexpected library invocation: %+v", got)
	}
	if !got.InvokedInReconcileLoop {
		t.Fatalf("expected reachable helper invocation to be marked in reconcile loop: %+v", got)
	}
}

func TestExtractControllers_LibraryImportedButNotInvoked(t *testing.T) {
	repoRoot := t.TempDir()

	controllerPkg := newSyntheticPackage(
		t,
		repoRoot,
		"example.com/project/controllers",
		map[string]string{
			"controllers/reconcile.go": `package controllers
import "helm.sh/helm/v3/pkg/action"

type Context struct{}
type Request struct{}
type Result struct{}

type FooReconciler struct{}

func (r *FooReconciler) Reconcile(ctx Context, req Request) (Result, error) {
	return Result{}, nil
}

func (r *FooReconciler) SetupWithManager(mgr any) error {
	_ = action.Configuration{}
	return nil
}
`,
		},
	)

	facts := ExtractControllers([]*packages.Package{controllerPkg}, repoRoot)
	if len(facts) != 1 {
		t.Fatalf("expected 1 controller fact, got %d", len(facts))
	}

	data, ok := facts[0].Data.(ControllerData)
	if !ok {
		t.Fatalf("expected ControllerData, got %T", facts[0].Data)
	}
	if len(data.LibraryInvocations) != 0 {
		t.Fatalf("expected no library invocation signal, got %+v", data.LibraryInvocations)
	}
}

func TestExtractControllers_LibraryInvocationOutsideReconcileLoop(t *testing.T) {
	repoRoot := t.TempDir()

	controllerPkg := newSyntheticPackage(
		t,
		repoRoot,
		"example.com/project/controllers",
		map[string]string{
			"controllers/reconcile.go": `package controllers
import "helm.sh/helm/v3/pkg/action"

type Context struct{}
type Request struct{}
type Result struct{}

type FooReconciler struct{}

func (r *FooReconciler) Reconcile(ctx Context, req Request) (Result, error) {
	return Result{}, nil
}

func (r *FooReconciler) warmCache() error {
	_ = action.NewInstall(nil)
	return nil
}
`,
		},
	)

	facts := ExtractControllers([]*packages.Package{controllerPkg}, repoRoot)
	if len(facts) != 1 {
		t.Fatalf("expected 1 controller fact, got %d", len(facts))
	}

	data, ok := facts[0].Data.(ControllerData)
	if !ok {
		t.Fatalf("expected ControllerData, got %T", facts[0].Data)
	}
	if len(data.LibraryInvocations) != 1 {
		t.Fatalf("expected 1 library invocation, got %+v", data.LibraryInvocations)
	}
	got := data.LibraryInvocations[0]
	if got.InvokedInReconcileLoop {
		t.Fatalf("expected unrelated method invocation to be marked outside reconcile loop: %+v", got)
	}
}

func TestExtractControllers_LibraryInvocationThroughCrossPackageWrapper(t *testing.T) {
	repoRoot := t.TempDir()

	writeTestFile(t, repoRoot, "go.mod", `module example.com/project

go 1.22

require helm.sh/helm/v3 v3.0.0

replace helm.sh/helm/v3 => ./third_party/helm
`)
	writeTestFile(t, repoRoot, "controllers/reconcile.go", `package controllers

import "example.com/project/renderer"

type Context struct{}
type Request struct{}
type Result struct{}

type FooReconciler struct{}

func (r *FooReconciler) Reconcile(ctx Context, req Request) (Result, error) {
	helmRenderer := renderer.NewHelmRenderer("charts/mlflow")
	_, err := helmRenderer.RenderChart()
	return Result{}, err
}
`)
	writeTestFile(t, repoRoot, "renderer/renderer.go", `package renderer

import (
	"helm.sh/helm/v3/pkg/engine"
	"helm.sh/helm/v3/pkg/loader"
)

type HelmRenderer struct {
	chartPath string
}

func NewHelmRenderer(chartPath string) *HelmRenderer {
	return &HelmRenderer{chartPath: chartPath}
}

func (h *HelmRenderer) RenderChart() (map[string]string, error) {
	chart, err := loader.Load(h.chartPath)
	if err != nil {
		return nil, err
	}
	return engine.Render(chart, map[string]any{}), nil
}
`)
	writeTestFile(t, repoRoot, "third_party/helm/go.mod", `module helm.sh/helm/v3

go 1.22
`)
	writeTestFile(t, repoRoot, "third_party/helm/pkg/loader/loader.go", `package loader

type Chart struct{}

func Load(path string) (*Chart, error) {
	return &Chart{}, nil
}
`)
	writeTestFile(t, repoRoot, "third_party/helm/pkg/engine/engine.go", `package engine

import "helm.sh/helm/v3/pkg/loader"

func Render(chart *loader.Chart, values map[string]any) map[string]string {
	return map[string]string{}
}
`)

	pkgs := loadPackagesFromDir(t, repoRoot)
	facts := ExtractControllers(pkgs, repoRoot)
	if len(facts) != 1 {
		t.Fatalf("expected 1 controller fact, got %d", len(facts))
	}

	data, ok := facts[0].Data.(ControllerData)
	if !ok {
		t.Fatalf("expected ControllerData, got %T", facts[0].Data)
	}
	if len(data.LibraryInvocations) != 2 {
		t.Fatalf("expected 2 cross-package library invocations, got %+v", data.LibraryInvocations)
	}

	gotCalls := map[string]LibraryInvocation{}
	for _, invocation := range data.LibraryInvocations {
		gotCalls[invocation.Call] = invocation
	}

	loadCall, ok := gotCalls["loader.Load"]
	if !ok {
		t.Fatalf("expected loader.Load invocation, got %+v", data.LibraryInvocations)
	}
	if loadCall.Method != "RenderChart" || !loadCall.InvokedInReconcileLoop || loadCall.Family != "helm" {
		t.Fatalf("unexpected loader.Load invocation: %+v", loadCall)
	}

	renderCall, ok := gotCalls["engine.Render"]
	if !ok {
		t.Fatalf("expected engine.Render invocation, got %+v", data.LibraryInvocations)
	}
	if renderCall.Method != "RenderChart" || !renderCall.InvokedInReconcileLoop || renderCall.Family != "helm" {
		t.Fatalf("unexpected engine.Render invocation: %+v", renderCall)
	}
}

func TestExtractControllers_MixedGuardedAndUnguarded(t *testing.T) {
	repoRoot := t.TempDir()

	controllerPkg := newSyntheticPackage(
		t,
		repoRoot,
		"example.com/project/controllers",
		map[string]string{
			"controllers/reconcile.go": `package controllers
import retry "k8s.io/client-go/util/retry"

type Context struct{}
type Request struct{}
type Result struct {
	Requeue bool
	RequeueAfter int
}

type statusWriter struct{}
func (statusWriter) Update(any, any) error { return nil }

type FooReconciler struct{}
func (r *FooReconciler) Status() statusWriter { return statusWriter{} }

func (r *FooReconciler) Reconcile(ctx Context, req Request) (Result, error) {
	// Bare status update - not guarded
	r.Status().Update(nil, nil)
	return Result{}, nil
}

func (r *FooReconciler) updateStatus() error {
	// Guarded status update
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return r.Status().Update(nil, nil)
	})
}
`,
		},
	)

	facts := ExtractControllers([]*packages.Package{controllerPkg}, repoRoot)
	if len(facts) != 1 {
		t.Fatalf("expected 1 controller fact, got %d", len(facts))
	}

	data, ok := facts[0].Data.(ControllerData)
	if !ok {
		t.Fatalf("expected ControllerData, got %T", facts[0].Data)
	}

	if len(data.StatusUpdateSites) != 2 {
		t.Fatalf("expected 2 status update sites, got %d: %+v", len(data.StatusUpdateSites), data.StatusUpdateSites)
	}

	// Sites are sorted by line. The bare call in Reconcile comes first.
	if data.StatusUpdateSites[0].IsGuarded {
		t.Error("expected first status update (bare) NOT to be guarded")
	}
	if data.StatusUpdateSites[0].Method != "Reconcile" {
		t.Errorf("expected first site method Reconcile, got %s", data.StatusUpdateSites[0].Method)
	}
	if !data.StatusUpdateSites[1].IsGuarded {
		t.Error("expected second status update (in helper) to be guarded")
	}
	if data.StatusUpdateSites[1].Method != "updateStatus" {
		t.Errorf("expected second site method updateStatus, got %s", data.StatusUpdateSites[1].Method)
	}

	if len(data.RetryOps) != 1 {
		t.Fatalf("expected 1 retry op, got %d", len(data.RetryOps))
	}
}

func TestExtractControllers_RetryOnError(t *testing.T) {
	repoRoot := t.TempDir()

	controllerPkg := newSyntheticPackage(
		t,
		repoRoot,
		"example.com/project/controllers",
		map[string]string{
			"controllers/reconcile.go": `package controllers
import retry "k8s.io/client-go/util/retry"

type Context struct{}
type Request struct{}
type Result struct {
	Requeue bool
	RequeueAfter int
}

type statusWriter struct{}
func (statusWriter) Update(any, any) error { return nil }

type FooReconciler struct{}
func (r *FooReconciler) Status() statusWriter { return statusWriter{} }

func isRetryable(err error) bool { return true }

func (r *FooReconciler) Reconcile(ctx Context, req Request) (Result, error) {
	retry.OnError(retry.DefaultBackoff, isRetryable, func() error {
		return r.Status().Update(nil, nil)
	})
	return Result{}, nil
}
`,
		},
	)

	facts := ExtractControllers([]*packages.Package{controllerPkg}, repoRoot)
	if len(facts) != 1 {
		t.Fatalf("expected 1 controller fact, got %d", len(facts))
	}

	data, ok := facts[0].Data.(ControllerData)
	if !ok {
		t.Fatalf("expected ControllerData, got %T", facts[0].Data)
	}

	if len(data.RetryOps) != 1 {
		t.Fatalf("expected 1 retry op, got %d", len(data.RetryOps))
	}
	if data.RetryOps[0].Function != "OnError" {
		t.Errorf("expected OnError, got %s", data.RetryOps[0].Function)
	}
	if data.RetryOps[0].BackoffKind != "DefaultBackoff" {
		t.Errorf("expected DefaultBackoff, got %s", data.RetryOps[0].BackoffKind)
	}
	if !data.RetryOps[0].WrapsStatusUpdate {
		t.Error("expected wraps_status_update=true")
	}

	if len(data.StatusUpdateSites) != 1 {
		t.Fatalf("expected 1 status update site, got %d", len(data.StatusUpdateSites))
	}
	if !data.StatusUpdateSites[0].IsGuarded {
		t.Error("expected status update to be guarded")
	}
	if data.StatusUpdateSites[0].GuardKind != "OnError" {
		t.Errorf("expected guard kind OnError, got %s", data.StatusUpdateSites[0].GuardKind)
	}
}

func TestExtractControllers_CustomBackoff(t *testing.T) {
	repoRoot := t.TempDir()

	controllerPkg := newSyntheticPackage(
		t,
		repoRoot,
		"example.com/project/controllers",
		map[string]string{
			"controllers/reconcile.go": `package controllers
import retry "k8s.io/client-go/util/retry"

type Context struct{}
type Request struct{}
type Result struct {
	Requeue bool
	RequeueAfter int
}

type Backoff struct {
	Steps int
}

type FooReconciler struct{}

func (r *FooReconciler) Reconcile(ctx Context, req Request) (Result, error) {
	retry.RetryOnConflict(Backoff{Steps: 5}, func() error {
		return nil
	})
	return Result{}, nil
}
`,
		},
	)

	facts := ExtractControllers([]*packages.Package{controllerPkg}, repoRoot)
	if len(facts) != 1 {
		t.Fatalf("expected 1 controller fact, got %d", len(facts))
	}

	data, ok := facts[0].Data.(ControllerData)
	if !ok {
		t.Fatalf("expected ControllerData, got %T", facts[0].Data)
	}

	if len(data.RetryOps) != 1 {
		t.Fatalf("expected 1 retry op, got %d", len(data.RetryOps))
	}
	if data.RetryOps[0].BackoffKind != "custom" {
		t.Errorf("expected custom backoff, got %s", data.RetryOps[0].BackoffKind)
	}
}
