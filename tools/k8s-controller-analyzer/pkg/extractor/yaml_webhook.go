package extractor

// ExtractWebhookManifests extracts facts from webhook configuration YAML manifests.
func ExtractWebhookManifests(docs []YAMLDoc) []Fact {
	var facts []Fact

	for _, doc := range docs {
		if doc.Kind != "ValidatingWebhookConfiguration" && doc.Kind != "MutatingWebhookConfiguration" {
			continue
		}

		meta := metadataFrom(doc.Data)
		data := WebhookManifestData{
			Name: meta.name,
			Kind: doc.Kind,
		}

		webhooks, _ := doc.Data["webhooks"].([]any)
		for _, w := range webhooks {
			wh, ok := w.(map[string]any)
			if !ok {
				continue
			}

			entry := WebhookManifestEntry{
				Name: stringField(wh, "name"),
			}

			entry.FailurePolicy = stringField(wh, "failurePolicy")
			entry.SideEffects = stringField(wh, "sideEffects")

			if ts, ok := wh["timeoutSeconds"].(float64); ok {
				entry.TimeoutSeconds = int(ts)
			} else if ts, ok := wh["timeoutSeconds"].(int64); ok {
				entry.TimeoutSeconds = int(ts)
			}

			// Extract scopes from rules
			rules, _ := wh["rules"].([]any)
			for _, r := range rules {
				rule, ok := r.(map[string]any)
				if !ok {
					continue
				}
				if scope := stringField(rule, "scope"); scope != "" {
					entry.Scopes = append(entry.Scopes, scope)
				}
			}

			data.Webhooks = append(data.Webhooks, entry)
		}

		facts = append(facts, NewFact(
			RuleWebhookManifest,
			KindWebhookManifest,
			doc.RelPath,
			0,
			data,
		))
	}

	return facts
}
