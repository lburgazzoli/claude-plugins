package extractor

// ExtractCRDManifests extracts facts from CustomResourceDefinition YAML manifests.
func ExtractCRDManifests(docs []YAMLDoc) []Fact {
	var facts []Fact

	for _, doc := range docs {
		if doc.Kind != "CustomResourceDefinition" {
			continue
		}

		meta := metadataFrom(doc.Data)
		spec, _ := doc.Data["spec"].(map[string]any)
		if spec == nil {
			continue
		}

		names, _ := spec["names"].(map[string]any)
		kind, _ := names["kind"].(string)
		group, _ := spec["group"].(string)
		scope, _ := spec["scope"].(string)

		data := CRDManifestData{
			Name:  meta.name,
			Group: group,
			Kind:  kind,
			Scope: scope,
		}

		// Parse conversion strategy
		if conv, ok := spec["conversion"].(map[string]any); ok {
			data.ConversionStrategy, _ = conv["strategy"].(string)
		}

		// Parse versions
		versions, _ := spec["versions"].([]any)
		for _, v := range versions {
			ver, ok := v.(map[string]any)
			if !ok {
				continue
			}

			name, _ := ver["name"].(string)
			storage, _ := ver["storage"].(bool)
			served, _ := ver["served"].(bool)
			deprecated, _ := ver["deprecated"].(bool)

			data.Versions = append(data.Versions, CRDManifestVersion{
				Name:       name,
				Storage:    storage,
				Served:     served,
				Deprecated: deprecated,
			})
		}

		// Compute served version count
		servedCount := 0
		for _, v := range data.Versions {
			if v.Served {
				servedCount++
			}
		}
		data.ServedVersionCount = servedCount
		data.HasMultipleServed = servedCount > 1

		facts = append(facts, NewFact(
			RuleCRDManifest,
			KindCRDManifest,
			doc.RelPath,
			0,
			data,
		))
	}

	return facts
}
