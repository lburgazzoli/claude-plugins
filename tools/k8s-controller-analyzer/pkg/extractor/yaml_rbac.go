package extractor

// ExtractRBACManifests extracts facts from Role/ClusterRole YAML manifests.
func ExtractRBACManifests(docs []YAMLDoc) []Fact {
	var facts []Fact

	for _, doc := range docs {
		if doc.Kind != "Role" && doc.Kind != "ClusterRole" {
			continue
		}

		meta := metadataFrom(doc.Data)
		data := RBACManifestData{
			Name:      meta.name,
			Kind:      doc.Kind,
			Namespace: meta.namespace,
		}

		rules, _ := doc.Data["rules"].([]any)
		for _, r := range rules {
			rule, ok := r.(map[string]any)
			if !ok {
				continue
			}

			rd := RBACRuleData{
				APIGroups:     toStringSlice(rule["apiGroups"]),
				Resources:     toStringSlice(rule["resources"]),
				Verbs:         toStringSlice(rule["verbs"]),
				ResourceNames: toStringSlice(rule["resourceNames"]),
			}

			for _, v := range rd.Verbs {
				if v == "*" {
					data.HasWildcard = true
				}
			}
			for _, res := range rd.Resources {
				if res == "*" {
					data.HasWildcard = true
				}
				if res == "events" {
					data.HasEvents = true
				}
			}

			data.Rules = append(data.Rules, rd)
		}

		facts = append(facts, NewFact(
			RuleRBACManifest,
			KindRBACManifest,
			doc.RelPath,
			0,
			data,
		))
	}

	return facts
}
