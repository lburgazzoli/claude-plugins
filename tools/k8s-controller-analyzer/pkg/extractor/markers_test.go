package extractor

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestExtractMarkersFromDoc(t *testing.T) {
	src := `package test

// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get
// This is a normal comment.
func Reconcile() {}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// Get the comment group above the function
	if len(file.Comments) == 0 {
		t.Fatal("expected comments")
	}

	markers := ExtractMarkersFromDoc(file.Comments[0], fset)
	if len(markers) != 2 {
		t.Fatalf("expected 2 markers, got %d", len(markers))
	}

	if markers[0].Name != "kubebuilder:rbac" {
		t.Errorf("expected kubebuilder:rbac, got %s", markers[0].Name)
	}
	if markers[0].Args["groups"] != "apps" {
		t.Errorf("expected groups=apps, got %s", markers[0].Args["groups"])
	}
	if markers[0].Args["resources"] != "deployments" {
		t.Errorf("expected resources=deployments, got %s", markers[0].Args["resources"])
	}
}

func TestMarkerName(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{"kubebuilder:rbac:groups=apps,resources=deployments,verbs=get", "kubebuilder:rbac"},
		{"kubebuilder:storageversion", "kubebuilder:storageversion"},
		{"kubebuilder:object:root=true", "kubebuilder:object:root"},
		{"groupName=example.com", "groupName"},
		{"kubebuilder:subresource:status", "kubebuilder:subresource:status"},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got := markerName(tt.raw)
			if got != tt.want {
				t.Errorf("markerName(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestMarkerArgs(t *testing.T) {
	raw := "kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch"
	args := markerArgs(raw)

	if args["groups"] != "apps" {
		t.Errorf("expected groups=apps, got %s", args["groups"])
	}
	if args["resources"] != "deployments" {
		t.Errorf("expected resources=deployments, got %s", args["resources"])
	}
	if args["verbs"] != "get;list;watch" {
		t.Errorf("expected verbs=get;list;watch, got %s", args["verbs"])
	}
}
