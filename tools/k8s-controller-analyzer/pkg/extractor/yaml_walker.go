package extractor

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"sigs.k8s.io/yaml"
)

// RepoWalkResult holds discovered YAML documents and test files from a repo walk.
type RepoWalkResult struct {
	YAMLDocs  []YAMLDoc
	TestFiles []string
}

// YAMLDoc represents a single parsed YAML document from the repo.
type YAMLDoc struct {
	RelPath string
	Kind    string
	Data    map[string]any
}

var skipDirs = map[string]bool{
	".git":         true,
	"vendor":       true,
	"node_modules": true,
	"testdata":     true,
	".github":      true,
}

// WalkRepo walks the repository directory tree and returns discovered YAML
// documents and test files. Results are sorted for determinism.
func WalkRepo(repoPath string) (*RepoWalkResult, error) {
	result := &RepoWalkResult{}

	err := filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}

		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, _ := filepath.Rel(repoPath, path)

		// Collect test files
		if strings.HasSuffix(relPath, "_test.go") {
			result.TestFiles = append(result.TestFiles, relPath)
			return nil
		}

		// Collect YAML files
		ext := strings.ToLower(filepath.Ext(relPath))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		docs, err := parseYAMLFile(path, relPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping %s: %v\n", relPath, err)
			return nil
		}

		result.YAMLDocs = append(result.YAMLDocs, docs...)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking repo: %w", err)
	}

	sort.Strings(result.TestFiles)
	sort.Slice(result.YAMLDocs, func(i int, j int) bool {
		return result.YAMLDocs[i].RelPath < result.YAMLDocs[j].RelPath
	})

	return result, nil
}

func parseYAMLFile(
	absPath string,
	relPath string,
) ([]YAMLDoc, error) {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}

	// Skip files that look like templates (Helm etc.)
	if bytes.Contains(data, []byte("{{")) {
		return nil, nil
	}

	var docs []YAMLDoc
	for _, raw := range splitYAMLDocuments(data) {
		if len(bytes.TrimSpace(raw)) == 0 {
			continue
		}

		var parsed map[string]any
		if err := yaml.Unmarshal(raw, &parsed); err != nil {
			continue // skip unparseable documents
		}

		kind, _ := parsed["kind"].(string)
		if kind == "" {
			continue // skip documents without a kind
		}

		docs = append(docs, YAMLDoc{
			RelPath: relPath,
			Kind:    kind,
			Data:    parsed,
		})
	}

	return docs, nil
}

func splitYAMLDocuments(data []byte) [][]byte {
	return bytes.Split(data, []byte("\n---"))
}
