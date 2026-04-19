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
				Permissions: normalizePermissionTuples(
					toStringSlice(rule["apiGroups"]),
					toStringSlice(rule["resources"]),
					toStringSlice(rule["verbs"]),
					toStringSlice(rule["resourceNames"]),
					scopeForRBACManifest(doc.Kind),
					doc.RelPath,
					0,
				),
			}

			for _, v := range rd.Verbs {
				if v == "*" {
					data.HasWildcard = true
					data.HasWildcardVerb = true
				}
			}
			for _, group := range rd.APIGroups {
				if group == "*" {
					data.HasWildcard = true
					data.HasWildcardGroup = true
				}
			}
			for _, res := range rd.Resources {
				if res == "*" {
					data.HasWildcard = true
					data.HasWildcardResource = true
				}
				if res == "events" {
					data.HasEvents = true
				}
			}

			data.Rules = append(data.Rules, rd)
			data.Permissions = append(data.Permissions, rd.Permissions...)
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
