package extractor

import "fmt"

// ExtractDeploymentManifests extracts facts from Deployment/StatefulSet YAML manifests.
func ExtractDeploymentManifests(docs []YAMLDoc) []Fact {
	var facts []Fact

	for _, doc := range docs {
		if doc.Kind != "Deployment" && doc.Kind != "StatefulSet" {
			continue
		}

		meta := metadataFrom(doc.Data)
		data := DeploymentManifestData{
			Name:      meta.name,
			Kind:      doc.Kind,
			Namespace: meta.namespace,
		}

		spec, _ := doc.Data["spec"].(map[string]any)
		if spec == nil {
			continue
		}

		template, _ := spec["template"].(map[string]any)
		if template == nil {
			continue
		}

		templateSpec, _ := template["spec"].(map[string]any)
		if templateSpec == nil {
			continue
		}

		// Pod-level security context
		if sc, ok := templateSpec["securityContext"].(map[string]any); ok {
			data.SecurityContext = sc
		}

		// Containers
		containers, _ := templateSpec["containers"].([]any)
		for _, c := range containers {
			container, ok := c.(map[string]any)
			if !ok {
				continue
			}

			cd := ContainerManifestData{
				Name: stringField(container, "name"),
			}

			// Resources
			if res, ok := container["resources"].(map[string]any); ok {
				cd.Requests = resourceMap(res, "requests")
				cd.Limits = resourceMap(res, "limits")
			}

			// Container security context
			if sc, ok := container["securityContext"].(map[string]any); ok {
				cd.SecurityContext = sc
			}

			// Probes
			cd.HasLiveness = container["livenessProbe"] != nil
			cd.HasReadiness = container["readinessProbe"] != nil

			data.Containers = append(data.Containers, cd)
		}

		facts = append(facts, NewFact(
			RuleDeploymentManifest,
			KindDeploymentManifest,
			doc.RelPath,
			0,
			data,
		))
	}

	return facts
}

func resourceMap(
	res map[string]any,
	key string,
) map[string]string {
	section, ok := res[key].(map[string]any)
	if !ok {
		return nil
	}

	result := map[string]string{}
	for k, v := range section {
		result[k] = fmt.Sprintf("%v", v)
	}

	return result
}
