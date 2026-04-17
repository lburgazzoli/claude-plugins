package extractor

import "fmt"

type k8sMetadata struct {
	name      string
	namespace string
}

func metadataFrom(data map[string]any) k8sMetadata {
	meta, _ := data["metadata"].(map[string]any)
	if meta == nil {
		return k8sMetadata{}
	}

	name, _ := meta["name"].(string)
	namespace, _ := meta["namespace"].(string)

	return k8sMetadata{name: name, namespace: namespace}
}

func stringField(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

func toStringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}

	var result []string
	for _, item := range arr {
		result = append(result, fmt.Sprintf("%v", item))
	}

	return result
}
