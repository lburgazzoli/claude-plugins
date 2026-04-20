package extractor

// Fact represents a single extracted fact about the operator codebase.
type Fact struct {
	Rules []string `json:"rules"`
	Kind  string   `json:"kind"`
	File  string   `json:"file"`
	Line  int      `json:"line"`
	Data  any      `json:"data"`
}

// PermissionTuple is a normalized RBAC-relevant permission signal.
type PermissionTuple struct {
	Group         string   `json:"group,omitempty"`
	Version       string   `json:"version,omitempty"`
	Kind          string   `json:"kind,omitempty"`
	Resource      string   `json:"resource,omitempty"`
	Subresource   string   `json:"subresource,omitempty"`
	Verbs         []string `json:"verbs,omitempty"`
	Scope         string   `json:"scope,omitempty"`
	ResourceNames []string `json:"resource_names,omitempty"`
	SourcePath    string   `json:"source_path,omitempty"`
	SourceLine    int      `json:"source_line,omitempty"`
}

// AmbiguitySignal explains why a controller signal could not be resolved fully.
type AmbiguitySignal struct {
	Kind   string `json:"kind"`
	Detail string `json:"detail,omitempty"`
	File   string `json:"file,omitempty"`
	Line   int    `json:"line"`
}

// NewFact creates a Fact with a single rule string.
func NewFact(
	rule string,
	kind string,
	file string,
	line int,
	data any,
) Fact {
	return Fact{
		Rules: []string{rule},
		Kind:  kind,
		File:  file,
		Line:  line,
		Data:  data,
	}
}

// NewMultiRuleFact creates a Fact with multiple rule strings.
func NewMultiRuleFact(
	rules []string,
	kind string,
	file string,
	line int,
	data any,
) Fact {
	copiedRules := append([]string(nil), rules...)
	return Fact{
		Rules: copiedRules,
		Kind:  kind,
		File:  file,
		Line:  line,
		Data:  data,
	}
}

// RBACMarker represents a parsed kubebuilder:rbac marker.
type RBACMarker struct {
	Verbs         string            `json:"verbs"`
	Resource      string            `json:"resource"`
	Group         string            `json:"group"`
	ResourceNames string            `json:"resource_names,omitempty"`
	Namespace     string            `json:"namespace,omitempty"`
	Line          int               `json:"line"`
	Permissions   []PermissionTuple `json:"permissions,omitempty"`
}

// FinalizerOp represents a finalizer add/remove/contains operation.
type FinalizerOp struct {
	Op    string `json:"op"`
	Value string `json:"value"`
	Line  int    `json:"line"`
}

// APICall represents a candidate Kubernetes client API call.
type APICall struct {
	Method              string            `json:"method"`
	Receiver            string            `json:"receiver"`
	ObjType             string            `json:"obj_type"`
	File                string            `json:"file,omitempty"`
	MethodContext       string            `json:"method_context,omitempty"`
	ResolvedType        string            `json:"resolved_type,omitempty"`
	ResolvedGroup       string            `json:"resolved_group,omitempty"`
	ResolvedVersion     string            `json:"resolved_version,omitempty"`
	ResolvedKind        string            `json:"resolved_kind,omitempty"`
	OperationClass      string            `json:"operation_class,omitempty"`
	ReceiverResolution  string            `json:"receiver_resolution,omitempty"`
	ObjectResolution    string            `json:"object_resolution,omitempty"`
	RequiredPermissions []PermissionTuple `json:"required_permissions,omitempty"`
	Line                int               `json:"line"`
}

// WriteOp represents an external write operation (Create/Update/Patch/Delete).
type WriteOp struct {
	Call string `json:"call"`
	Line int    `json:"line"`
}

// StatusConditionSet represents a status condition being set.
type StatusConditionSet struct {
	Condition      string `json:"condition"`
	HasObservedGen bool   `json:"has_observed_generation"`
	Line           int    `json:"line"`
}

// StatusUpdateSite represents a status update call and whether it's guarded.
type StatusUpdateSite struct {
	IsGuarded bool   `json:"is_guarded"`
	GuardKind string `json:"guard_kind,omitempty"` // "RetryOnConflict", "OnError", or ""
	Method    string `json:"method"`               // reconciler method containing this call
	Text      string `json:"text"`
	Line      int    `json:"line"`
}

// RetryOp represents a retry wrapper call (e.g., retry.RetryOnConflict).
type RetryOp struct {
	Function          string   `json:"function"`      // "RetryOnConflict" or "OnError"
	BackoffKind       string   `json:"backoff_kind"`  // "DefaultRetry", "DefaultBackoff", or "custom"
	Method            string   `json:"method"`        // reconciler method containing this call
	WrappedCalls      []string `json:"wrapped_calls"` // API calls inside closure, e.g. ["Status.Update"]
	WrapsStatusUpdate bool     `json:"wraps_status_update"`
	Line              int      `json:"line"`
}

// LibraryInvocation represents a Helm/Kustomize call site on a controller method.
type LibraryInvocation struct {
	Family                 string `json:"family"` // "helm" or "kustomize"
	Call                   string `json:"call"`
	Method                 string `json:"method"`
	Line                   int    `json:"line"`
	InvokedInReconcileLoop bool   `json:"invoked_in_reconcile_loop"`
}

// EventUsage represents an event recorder call.
type EventUsage struct {
	Receiver            string            `json:"receiver"`
	Method              string            `json:"method"`
	File                string            `json:"file,omitempty"`
	OperationClass      string            `json:"operation_class,omitempty"`
	RequiredPermissions []PermissionTuple `json:"required_permissions,omitempty"`
	Line                int               `json:"line"`
}

// OwnerRefOp represents owner reference or finalizer operations.
type OwnerRefOp struct {
	Type string `json:"type"` // "owner-reference" or "finalizer"
	Call string `json:"call"`
	Line int    `json:"line"`
}

// NotFoundHandling represents a not-found error handling site.
type NotFoundHandling struct {
	Pattern string `json:"pattern"` // "IsNotFound" or "IgnoreNotFound"
	Line    int    `json:"line"`
}

// PredicateUsage represents a predicate in a watch setup.
type PredicateUsage struct {
	Name string `json:"name"`
	Line int    `json:"line"`
}

// RequeueOp represents a requeue operation.
type RequeueOp struct {
	Kind string `json:"kind"`
	Line int    `json:"line"`
}

// ErrorReturn represents an error return statement.
type ErrorReturn struct {
	Line       int  `json:"line"`
	HasRequeue bool `json:"has_requeue"`
}

// ReconcilesTarget identifies the GVK inferred for a controller.
type ReconcilesTarget struct {
	Group   string `json:"group"`
	Version string `json:"version"`
	Kind    string `json:"kind"`
}

// ControllerData holds all extracted data for a controller/reconciler.
type ControllerData struct {
	Name                   string               `json:"name"`
	Reconciles             ReconcilesTarget     `json:"reconciles"`
	Owns                   []string             `json:"owns"`
	Watches                []string             `json:"watches"`
	RBACMarkers            []RBACMarker         `json:"rbac_markers"`
	FinalizerOps           []FinalizerOp        `json:"finalizer_ops"`
	OwnerRefOps            []OwnerRefOp         `json:"owner_ref_ops"`
	ExternalWriteOps       []WriteOp            `json:"external_write_ops"`
	APICalls               []APICall            `json:"api_calls"`
	StatusConditionSets    []StatusConditionSet `json:"status_condition_sets"`
	StatusUpdateSites      []StatusUpdateSite   `json:"status_update_sites"`
	RetryOps               []RetryOp            `json:"retry_ops"`
	LibraryInvocations     []LibraryInvocation  `json:"library_invocations"`
	EventUsages            []EventUsage         `json:"event_usages"`
	NotFoundHandlers       []NotFoundHandling   `json:"not_found_handlers"`
	PredicateUsages        []PredicateUsage     `json:"predicate_usages"`
	RequeueOps             []RequeueOp          `json:"requeue_ops"`
	ErrorReturns           []ErrorReturn        `json:"error_returns"`
	AmbiguitySignals       []AmbiguitySignal    `json:"ambiguity_signals,omitempty"`
	MaxConcurrentReconciles int                  `json:"max_concurrent_reconciles,omitempty"`
}

// CRDVersionData holds extracted data for a CRD version.
type CRDVersionData struct {
	Kind    string `json:"kind"`
	Group   string `json:"group"`
	Version string `json:"version"`
	Storage bool   `json:"storage"`
	Served  bool   `json:"served"`
	Hub     bool   `json:"hub"`
	Spoke   bool   `json:"spoke"`
}

// CRDFieldData holds extracted data for a CRD field.
type CRDFieldData struct {
	TypeName     string        `json:"type_name"`
	FieldName    string        `json:"field_name"`
	FieldType    string        `json:"field_type"`
	JSONTag      string        `json:"json_tag,omitempty"`
	HasOmitempty bool          `json:"has_omitempty"`
	IsOptional   bool          `json:"is_optional"`
	IsRequired   bool          `json:"is_required"`
	ListType       string        `json:"list_type,omitempty"`        // "atomic", "set", or "map"
	ListMapKeys    []string      `json:"list_map_keys,omitempty"`    // keys for listType=map
	CELRules       []CELRule     `json:"cel_rules,omitempty"`        // x-kubernetes-validations
	HasMaxItems    bool          `json:"has_max_items,omitempty"`    // +kubebuilder:validation:MaxItems present
	HasMaxProperties bool       `json:"has_max_properties,omitempty"` // +kubebuilder:validation:MaxProperties present
	Markers        []FieldMarker `json:"markers,omitempty"`
}

// CELRule represents a parsed +kubebuilder:validation:XValidation rule.
type CELRule struct {
	Rule       string `json:"rule"`
	Message    string `json:"message,omitempty"`
	UsesOldSel bool   `json:"uses_old_self"` // transition rule using oldSelf
	Line       int    `json:"line"`
}

// FieldMarker represents a validation/default marker on a struct field.
type FieldMarker struct {
	Name  string `json:"name"`
	Value string `json:"value,omitempty"`
	Line  int    `json:"line"`
}

// CRDTypeData holds extracted data about a root CRD type's markers.
type CRDTypeData struct {
	Kind           string         `json:"kind"`
	HasRootMarker  bool           `json:"has_root_marker"`
	HasStatusSub   bool           `json:"has_status_subresource"`
	HasStatusField bool           `json:"has_status_field"`
	ResourceScope  string         `json:"resource_scope,omitempty"`
	PrintColumns   []FieldMarker  `json:"print_columns,omitempty"`
	Fields         []CRDFieldData `json:"fields,omitempty"`
	UnsignedFields []CRDFieldData `json:"unsigned_fields,omitempty"`

	// Status struct analysis
	StatusFieldType string `json:"status_field_type,omitempty"` // e.g., "FooStatus"

	// CEL validation rules on the type
	CELRules []CELRule `json:"cel_rules,omitempty"`
}

// WebhookData holds extracted data for a webhook.
type WebhookData struct {
	Kind              string `json:"kind"`
	Type              string `json:"type"`
	Path              string `json:"path"`
	FailurePolicy     string `json:"failure_policy,omitempty"`
	SideEffects       string `json:"side_effects,omitempty"`
	TimeoutSeconds    string `json:"timeout_seconds,omitempty"`
	HasAuthAnnotation bool   `json:"has_auth_annotation"`
}

// SchemeRegistrationData holds extracted data for a scheme registration.
type SchemeRegistrationData struct {
	Package string `json:"package"`
	Call    string `json:"call"`
}

// ImportData holds extracted import analysis for a file.
type ImportData struct {
	VendorImports       []VendorImport  `json:"vendor_imports,omitempty"`
	LibraryImports      []LibraryImport `json:"library_imports,omitempty"`
	UnstructuredLogging []LoggingCall   `json:"unstructured_logging,omitempty"`
	HasMetrics          bool            `json:"has_metrics"`
	MetricsPackage      string          `json:"metrics_package,omitempty"`
}

// VendorImport represents a vendor-specific import.
type VendorImport struct {
	Path   string `json:"path"`
	Vendor string `json:"vendor"`
	Line   int    `json:"line"`
}

// LibraryImport represents usage of Helm/Kustomize libraries.
type LibraryImport struct {
	Family string `json:"family"` // "helm" or "kustomize"
	Path   string `json:"path"`
	Line   int    `json:"line"`
}

// LoggingCall represents an unstructured logging call.
type LoggingCall struct {
	Call string `json:"call"`
	Line int    `json:"line"`
}

// --- YAML manifest types ---

// RBACManifestData holds facts from a Role/ClusterRole YAML file.
type RBACManifestData struct {
	Name                string            `json:"name"`
	Kind                string            `json:"kind"`
	Namespace           string            `json:"namespace,omitempty"`
	Rules               []RBACRuleData    `json:"rules"`
	Permissions         []PermissionTuple `json:"permissions,omitempty"`
	HasWildcard         bool              `json:"has_wildcard"`
	HasWildcardGroup    bool              `json:"has_wildcard_group,omitempty"`
	HasWildcardResource bool              `json:"has_wildcard_resource,omitempty"`
	HasWildcardVerb     bool              `json:"has_wildcard_verb,omitempty"`
	HasEvents           bool              `json:"has_events"`
}

// RBACRuleData represents a single RBAC rule entry.
type RBACRuleData struct {
	APIGroups     []string          `json:"api_groups"`
	Resources     []string          `json:"resources"`
	Verbs         []string          `json:"verbs"`
	ResourceNames []string          `json:"resource_names,omitempty"`
	Permissions   []PermissionTuple `json:"permissions,omitempty"`
}

// CRDManifestData holds facts from a CustomResourceDefinition YAML file.
type CRDManifestData struct {
	Name               string               `json:"name"`
	Group              string               `json:"group"`
	Kind               string               `json:"kind"`
	Scope              string               `json:"scope"`
	Versions           []CRDManifestVersion `json:"versions"`
	ConversionStrategy string               `json:"conversion_strategy,omitempty"`
	ServedVersionCount int                  `json:"served_version_count"`
	HasMultipleServed  bool                 `json:"has_multiple_served"`
}

// CRDManifestVersion represents a single CRD version entry.
type CRDManifestVersion struct {
	Name       string `json:"name"`
	Storage    bool   `json:"storage"`
	Served     bool   `json:"served"`
	Deprecated bool   `json:"deprecated"`
}

// WebhookManifestData holds facts from a webhook configuration YAML file.
type WebhookManifestData struct {
	Name     string                 `json:"name"`
	Kind     string                 `json:"kind"`
	Webhooks []WebhookManifestEntry `json:"webhooks"`
}

// WebhookManifestEntry represents a single webhook entry.
type WebhookManifestEntry struct {
	Name               string   `json:"name"`
	FailurePolicy      string   `json:"failure_policy"`
	SideEffects        string   `json:"side_effects"`
	TimeoutSeconds     int      `json:"timeout_seconds"`
	ReinvocationPolicy string   `json:"reinvocation_policy,omitempty"`
	Scopes             []string `json:"scopes,omitempty"`
}

// DeploymentManifestData holds facts from a Deployment/StatefulSet YAML file.
type DeploymentManifestData struct {
	Name            string                  `json:"name"`
	Kind            string                  `json:"kind"`
	Namespace       string                  `json:"namespace,omitempty"`
	Containers      []ContainerManifestData `json:"containers"`
	SecurityContext map[string]any          `json:"security_context,omitempty"`
}

// ContainerManifestData holds extracted container-level deployment facts.
type ContainerManifestData struct {
	Name            string            `json:"name"`
	Requests        map[string]string `json:"requests,omitempty"`
	Limits          map[string]string `json:"limits,omitempty"`
	SecurityContext map[string]any    `json:"security_context,omitempty"`
	HasLiveness     bool              `json:"has_liveness"`
	HasReadiness    bool              `json:"has_readiness"`
}

// NetworkPolicyManifestData holds facts from a NetworkPolicy YAML file.
type NetworkPolicyManifestData struct {
	Name        string   `json:"name"`
	Namespace   string   `json:"namespace,omitempty"`
	PolicyTypes []string `json:"policy_types"`
}

// TestDiscoveryData holds discovered test file paths.
type TestDiscoveryData struct {
	Files []string `json:"files"`
	Count int      `json:"count"`
}

// ManifestData holds the file manifest (replacing evidence_manifest.py).
type ManifestData struct {
	Skill   string          `json:"skill"`
	Count   int             `json:"count"`
	Hash    string          `json:"hash"`
	Entries []ManifestEntry `json:"entries"`
}

// ManifestEntry represents a categorized file in the manifest.
type ManifestEntry struct {
	Category string `json:"category"`
	Path     string `json:"path"`
}

// CertProvisioningData holds a detected certificate provisioning signal.
// One fact is emitted per signal; the lifecycle skill checks for the presence of any.
type CertProvisioningData struct {
	Mechanism string `json:"mechanism"`        // "cert-manager", "openshift-service-ca", "certdir"
	Source    string `json:"source"`            // "yaml" or "go"
	Detail    string `json:"detail,omitempty"` // e.g., annotation value, CertDir path
}

// ManagerConfigData holds extracted manager configuration from main/cmd.
type ManagerConfigData struct {
	LeaderElection             bool   `json:"leader_election"`
	LeaderElectionID           string `json:"leader_election_id,omitempty"`
	LeaderElectionResourceLock string `json:"leader_election_resource_lock,omitempty"`
	LeaderElectionReleaseOnCancel bool `json:"leader_election_release_on_cancel,omitempty"`
	HasSignalHandler           bool   `json:"has_signal_handler"`
	GracefulShutdownTimeout    string `json:"graceful_shutdown_timeout,omitempty"`
}
