package extractor

import "testing"

func TestExtractRBACManifests_NormalizesPermissions(t *testing.T) {
	docs := []YAMLDoc{
		{
			RelPath: "config/rbac/role.yaml",
			Kind:    "ClusterRole",
			Data: map[string]any{
				"metadata": map[string]any{
					"name": "manager-role",
				},
				"rules": []any{
					map[string]any{
						"apiGroups": []any{"example.com"},
						"resources": []any{"foos", "foos/status", "*"},
						"verbs":     []any{"get", "update", "*"},
					},
				},
			},
		},
	}

	facts := ExtractRBACManifests(docs)
	if len(facts) != 1 {
		t.Fatalf("expected 1 RBAC manifest fact, got %d", len(facts))
	}

	data, ok := facts[0].Data.(RBACManifestData)
	if !ok {
		t.Fatalf("expected RBACManifestData, got %T", facts[0].Data)
	}

	if !data.HasWildcard || !data.HasWildcardVerb || !data.HasWildcardResource {
		t.Fatalf("expected split wildcard metadata, got %+v", data)
	}
	if len(data.Rules) != 1 || len(data.Rules[0].Permissions) != 3 {
		t.Fatalf("expected per-rule normalized permissions, got %+v", data.Rules)
	}
	if len(data.Permissions) != 3 {
		t.Fatalf("expected manifest-level normalized permissions, got %+v", data.Permissions)
	}

	foundStatusPermission := false
	for _, permission := range data.Permissions {
		if permission.Resource == "foos" && permission.Subresource == "status" {
			foundStatusPermission = true
		}
		if permission.Scope != "Cluster" {
			t.Fatalf("expected Cluster scope, got %+v", permission)
		}
	}
	if !foundStatusPermission {
		t.Fatalf("expected foos/status permission, got %+v", data.Permissions)
	}
}
