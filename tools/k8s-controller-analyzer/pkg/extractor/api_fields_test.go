package extractor

import (
	"testing"

	"golang.org/x/tools/go/packages"
)

func TestExtractAPIFields_RootAndFieldMarkers(t *testing.T) {
	repoRoot := t.TempDir()

	apiPkg := newSyntheticPackage(
		t,
		repoRoot,
		"example.com/project/api/v1alpha1",
		map[string]string{
			"api/v1alpha1/foo_types.go": `package v1alpha1

type FooSpec struct {
	// +kubebuilder:validation:Optional
	Replicas int32 ` + "`json:\"replicas,omitempty\"`" + `
}

type FooStatus struct {
	State string ` + "`json:\"state,omitempty\"`" + `
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type Foo struct {
	Spec    FooSpec   ` + "`json:\"spec,omitempty\"`" + `
	Status  FooStatus ` + "`json:\"status,omitempty\"`" + `
	Counter uint32    ` + "`json:\"counter,omitempty\"`" + `
}
`,
		},
	)

	facts := ExtractAPIFields([]*packages.Package{apiPkg}, repoRoot)
	if len(facts) == 0 {
		t.Fatal("expected API field facts")
	}

	foundType := false
	foundReplicas := false

	for _, fact := range facts {
		switch fact.Kind {
		case KindCRDType:
			data, ok := fact.Data.(CRDTypeData)
			if !ok || data.Kind != "Foo" {
				continue
			}
			foundType = true
			if !data.HasRootMarker || !data.HasStatusSub || !data.HasStatusField {
				t.Fatalf("unexpected crd_type markers: %+v", data)
			}
			if len(data.UnsignedFields) != 1 || data.UnsignedFields[0].FieldName != "Counter" {
				t.Fatalf("expected unsigned Counter field, got %+v", data.UnsignedFields)
			}
		case KindCRDField:
			data, ok := fact.Data.(CRDFieldData)
			if !ok || data.TypeName != "FooSpec" || data.FieldName != "Replicas" {
				continue
			}
			foundReplicas = true
			if !data.IsOptional || !data.HasOmitempty {
				t.Fatalf("expected optional+omitempty on Replicas, got %+v", data)
			}
		}
	}

	if !foundType {
		t.Fatal("expected Foo crd_type fact")
	}
	if !foundReplicas {
		t.Fatal("expected FooSpec.Replicas field fact")
	}
}

func TestExtractAPIFields_StatusFieldType(t *testing.T) {
	repoRoot := t.TempDir()

	apiPkg := newSyntheticPackage(
		t,
		repoRoot,
		"example.com/project/api/v1alpha1",
		map[string]string{
			"api/v1alpha1/foo_types.go": `package v1alpha1

type FooStatus struct {
	Conditions []Condition ` + "`json:\"conditions,omitempty\"`" + `
}

type Condition struct {
	Type string
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type Foo struct {
	Spec   FooSpec   ` + "`json:\"spec,omitempty\"`" + `
	Status FooStatus ` + "`json:\"status,omitempty\"`" + `
}

type FooSpec struct {
	Replicas int32 ` + "`json:\"replicas,omitempty\"`" + `
}
`,
		},
	)

	facts := ExtractAPIFields([]*packages.Package{apiPkg}, repoRoot)

	var typeData *CRDTypeData
	for _, fact := range facts {
		if fact.Kind == KindCRDType {
			d, ok := fact.Data.(CRDTypeData)
			if ok && d.Kind == "Foo" {
				typeData = &d
				break
			}
		}
	}

	if typeData == nil {
		t.Fatal("expected Foo crd_type fact")
	}

	if typeData.StatusFieldType != "FooStatus" {
		t.Errorf("expected StatusFieldType=FooStatus, got %s", typeData.StatusFieldType)
	}
}
