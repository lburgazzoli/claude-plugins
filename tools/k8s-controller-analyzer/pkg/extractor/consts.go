package extractor

// Kubebuilder marker names.
const (
	MarkerObjectRoot        = "kubebuilder:object:root"
	MarkerStorageVersion    = "kubebuilder:storageversion"
	MarkerSubresourceStatus = "kubebuilder:subresource:status"
	MarkerResource          = "kubebuilder:resource"
	MarkerRBAC              = "kubebuilder:rbac"
	MarkerWebhook           = "kubebuilder:webhook"
	MarkerPrintColumn       = "kubebuilder:printcolumn"
	MarkerValidationOpt     = "kubebuilder:validation:Optional"
	MarkerValidationReq     = "kubebuilder:validation:Required"
	MarkerGroupName         = "groupName"
)

// Kubebuilder marker argument keys.
const (
	ArgGroups         = "groups"
	ArgResources      = "resources"
	ArgVerbs          = "verbs"
	ArgResourceNames  = "resourceNames"
	ArgNamespace      = "namespace"
	ArgScope          = "scope"
	ArgPath           = "path"
	ArgMutating       = "mutating"
	ArgFailurePolicy  = "failurePolicy"
	ArgSideEffects    = "sideEffects"
	ArgTimeoutSeconds = "timeoutSeconds"
	ArgName           = "name"
)

// Kubernetes client method names.
const (
	MethodGet    = "Get"
	MethodList   = "List"
	MethodCreate = "Create"
	MethodUpdate = "Update"
	MethodDelete = "Delete"
	MethodPatch  = "Patch"
	MethodStatus = "Status"
)

// Controller-runtime function names.
const (
	FuncReconcile        = "Reconcile"
	FuncSetupWithManager = "SetupWithManager"
	FuncOwns             = "Owns"
	FuncWatches          = "Watches"
	FuncWithEventFilter  = "WithEventFilter"
)

// controllerutil function names.
const (
	FuncAddFinalizer      = "controllerutil.AddFinalizer"
	FuncRemoveFinalizer   = "controllerutil.RemoveFinalizer"
	FuncContainsFinalizer = "controllerutil.ContainsFinalizer"
	FuncSetCtrlRef        = "controllerutil.SetControllerReference"
	FuncSetOwnerRef       = "controllerutil.SetOwnerReference"
	FuncSetCtrlRefAlt     = "ctrl.SetControllerReference"
)

// Event recorder methods.
const (
	MethodEvent  = "Event"
	MethodEventf = "Eventf"
)

// Not-found handling functions.
const (
	FuncIsNotFound     = "IsNotFound"
	FuncIgnoreNotFound = "IgnoreNotFound"
)

// Retry function names (k8s.io/client-go/util/retry).
const (
	FuncRetryOnConflict = "RetryOnConflict"
	FuncOnError         = "OnError"
)

// Retry backoff identifiers.
const (
	BackoffDefaultRetry   = "DefaultRetry"
	BackoffDefaultBackoff = "DefaultBackoff"
	BackoffCustom         = "custom"
)

// Condition fields.
const (
	FieldType               = "Type"
	FieldObservedGeneration = "ObservedGeneration"
	FieldRequeue            = "Requeue"
	FieldRequeueAfter       = "RequeueAfter"
)

// Rule IDs.
const (
	RuleRBACCoverage      = "rbac-coverage"
	RuleRequeueSafety     = "requeue-safety"
	RuleFinalizerSafety   = "finalizer-safety"
	RuleStatusConditions  = "status-conditions"
	RuleWatchOwns         = "watch-owns-alignment"
	RuleLibraryRendering  = "library-rendering"
	RuleCRDVersion        = "crd-version-coverage"
	RuleWebhookAuth       = "webhook-auth"
	RuleSchemeReg         = "scheme-registration"
	RuleCRDStructure      = "crd-structure"
	RuleFieldConventions  = "field-conventions"
	RuleMarkerCorrectness = "marker-correctness"
	RuleVendorIsolation   = "vendor-isolation"
	RuleLibraryImports    = "library-imports"
	RuleStructuredLogging = "structured-logging"
	RuleMetricsCoverage   = "metrics-coverage"
)

// Rule IDs for YAML extractors.
const (
	RuleRBACManifest          = "rbac-manifest"
	RuleCRDManifest           = "crd-manifest"
	RuleWebhookManifest       = "webhook-manifest"
	RuleDeploymentManifest    = "deployment-manifest"
	RuleNetworkPolicyManifest = "networkpolicy-manifest"
	RuleTestDiscovery         = "test-discovery"
	RuleManifest              = "manifest"
)

// Fact kinds.
const (
	KindController         = "controller"
	KindCRDVersion         = "crd_version"
	KindCRDType            = "crd_type"
	KindCRDField           = "crd_field"
	KindWebhook            = "webhook"
	KindSchemeRegistration = "scheme_registration"
	KindImportAnalysis     = "import_analysis"

	KindRBACManifest          = "rbac_manifest"
	KindCRDManifest           = "crd_manifest"
	KindWebhookManifest       = "webhook_manifest"
	KindDeploymentManifest    = "deployment_manifest"
	KindNetworkPolicyManifest = "networkpolicy_manifest"
	KindTestDiscovery         = "test_discovery"
	KindManifest              = "manifest"
)
