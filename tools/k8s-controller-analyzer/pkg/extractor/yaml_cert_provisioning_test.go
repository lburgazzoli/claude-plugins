package extractor

import (
	"testing"
)

func TestExtractCertProvisioning_CertManagerCertificate(t *testing.T) {
	docs := []YAMLDoc{
		{
			RelPath: "config/certmanager/certificate.yaml",
			Kind:    "Certificate",
			Data: map[string]any{
				"metadata": map[string]any{
					"name":      "serving-cert",
					"namespace": "system",
				},
				"spec": map[string]any{
					"issuerRef": map[string]any{
						"kind": "Issuer",
						"name": "selfsigned-issuer",
					},
					"secretName": "webhook-server-cert",
				},
			},
		},
	}

	facts := ExtractCertProvisioning(docs)
	if len(facts) != 1 {
		t.Fatalf("expected 1 fact, got %d", len(facts))
	}

	data := facts[0].Data.(CertProvisioningData)
	if data.Mechanism != "cert-manager" {
		t.Errorf("expected mechanism=cert-manager, got %s", data.Mechanism)
	}
	if data.Source != "yaml" {
		t.Errorf("expected source=yaml, got %s", data.Source)
	}
	if data.Detail != "selfsigned-issuer" {
		t.Errorf("expected detail=selfsigned-issuer, got %s", data.Detail)
	}
}

func TestExtractCertProvisioning_InjectCAAnnotation(t *testing.T) {
	docs := []YAMLDoc{
		{
			RelPath: "config/webhook/manifests.yaml",
			Kind:    "ValidatingWebhookConfiguration",
			Data: map[string]any{
				"metadata": map[string]any{
					"name": "validating-webhook",
					"annotations": map[string]any{
						"cert-manager.io/inject-ca-from": "system/serving-cert",
					},
				},
			},
		},
	}

	facts := ExtractCertProvisioning(docs)
	if len(facts) != 1 {
		t.Fatalf("expected 1 fact, got %d", len(facts))
	}

	data := facts[0].Data.(CertProvisioningData)
	if data.Mechanism != "cert-manager" {
		t.Errorf("expected mechanism=cert-manager, got %s", data.Mechanism)
	}
	if data.Detail != "system/serving-cert" {
		t.Errorf("expected detail=system/serving-cert, got %s", data.Detail)
	}
}

func TestExtractCertProvisioning_OpenShiftServingCert(t *testing.T) {
	docs := []YAMLDoc{
		{
			RelPath: "config/default/metrics_service.yaml",
			Kind:    "Service",
			Data: map[string]any{
				"metadata": map[string]any{
					"name":      "metrics-service",
					"namespace": "system",
					"annotations": map[string]any{
						"service.beta.openshift.io/serving-cert-secret-name": "metrics-certs",
					},
				},
			},
		},
	}

	facts := ExtractCertProvisioning(docs)
	if len(facts) != 1 {
		t.Fatalf("expected 1 fact, got %d", len(facts))
	}

	data := facts[0].Data.(CertProvisioningData)
	if data.Mechanism != "openshift-service-ca" {
		t.Errorf("expected mechanism=openshift-service-ca, got %s", data.Mechanism)
	}
	if data.Detail != "metrics-certs" {
		t.Errorf("expected detail=metrics-certs, got %s", data.Detail)
	}
}

func TestExtractCertProvisioning_ServiceWithoutAnnotation(t *testing.T) {
	docs := []YAMLDoc{
		{
			RelPath: "config/default/service.yaml",
			Kind:    "Service",
			Data: map[string]any{
				"metadata": map[string]any{
					"name": "plain-service",
				},
			},
		},
	}

	facts := ExtractCertProvisioning(docs)
	if len(facts) != 0 {
		t.Errorf("expected 0 facts for unannotated Service, got %d", len(facts))
	}
}

func TestExtractCertProvisioning_CertificateWithoutIssuerRef(t *testing.T) {
	docs := []YAMLDoc{
		{
			RelPath: "config/certmanager/bad.yaml",
			Kind:    "Certificate",
			Data: map[string]any{
				"metadata": map[string]any{"name": "no-issuer"},
				"spec":     map[string]any{},
			},
		},
	}

	facts := ExtractCertProvisioning(docs)
	if len(facts) != 0 {
		t.Errorf("expected 0 facts for Certificate without issuerRef, got %d", len(facts))
	}
}
