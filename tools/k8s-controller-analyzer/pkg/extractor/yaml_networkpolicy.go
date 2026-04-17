package extractor

// ExtractNetworkPolicyManifests extracts facts from NetworkPolicy YAML manifests.
func ExtractNetworkPolicyManifests(docs []YAMLDoc) []Fact {
	var facts []Fact

	for _, doc := range docs {
		if doc.Kind != "NetworkPolicy" {
			continue
		}

		meta := metadataFrom(doc.Data)
		data := NetworkPolicyManifestData{
			Name:      meta.name,
			Namespace: meta.namespace,
		}

		spec, _ := doc.Data["spec"].(map[string]any)
		if spec != nil {
			data.PolicyTypes = toStringSlice(spec["policyTypes"])
		}

		facts = append(facts, NewFact(
			RuleNetworkPolicyManifest,
			KindNetworkPolicyManifest,
			doc.RelPath,
			0,
			data,
		))
	}

	return facts
}
