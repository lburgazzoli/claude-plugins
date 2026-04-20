package extractor

// ExtractCertProvisioning detects certificate provisioning signals from YAML manifests.
// One fact is emitted per detected signal. The lifecycle skill checks for the presence
// of any cert_provisioning fact when webhook_manifest facts also exist.
func ExtractCertProvisioning(docs []YAMLDoc) []Fact {
	var facts []Fact

	for _, doc := range docs {
		// cert-manager Certificate resource
		if doc.Kind == "Certificate" {
			spec, _ := doc.Data["spec"].(map[string]any)
			if spec != nil {
				if issuerRef, _ := spec["issuerRef"].(map[string]any); issuerRef != nil {
					facts = append(facts, NewFact(
						RuleCertProvisioning,
						KindCertProvisioning,
						doc.RelPath,
						0,
						CertProvisioningData{
							Mechanism: "cert-manager",
							Source:    "yaml",
							Detail:    stringField(issuerRef, "name"),
						},
					))
				}
			}
			continue
		}

		// inject-ca-from annotation on webhook configs
		if doc.Kind == "ValidatingWebhookConfiguration" || doc.Kind == "MutatingWebhookConfiguration" {
			if v := annotationValue(doc.Data, "cert-manager.io/inject-ca-from"); v != "" {
				facts = append(facts, NewFact(
					RuleCertProvisioning,
					KindCertProvisioning,
					doc.RelPath,
					0,
					CertProvisioningData{
						Mechanism: "cert-manager",
						Source:    "yaml",
						Detail:    v,
					},
				))
			}
			continue
		}

		// OpenShift serving-cert annotation on Service
		if doc.Kind == "Service" {
			if v := annotationValue(doc.Data, "service.beta.openshift.io/serving-cert-secret-name"); v != "" {
				facts = append(facts, NewFact(
					RuleCertProvisioning,
					KindCertProvisioning,
					doc.RelPath,
					0,
					CertProvisioningData{
						Mechanism: "openshift-service-ca",
						Source:    "yaml",
						Detail:    v,
					},
				))
			}
		}
	}

	return facts
}

func annotationValue(data map[string]any, key string) string {
	meta, _ := data["metadata"].(map[string]any)
	if meta == nil {
		return ""
	}
	annots, _ := meta["annotations"].(map[string]any)
	if annots == nil {
		return ""
	}

	return stringField(annots, key)
}
