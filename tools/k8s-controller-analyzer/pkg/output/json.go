package output

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/lburgazzoli/claude-plugins/tools/k8s-controller-analyzer/pkg/extractor"
)

// Report is the top-level output structure.
type Report struct {
	SchemaVersion string                  `json:"schema_version"`
	RepoPath      string                  `json:"repo_path"`
	ExtractedAt   string                  `json:"extracted_at"`
	Manifest      *extractor.ManifestData `json:"manifest,omitempty"`
	Facts         []extractor.Fact        `json:"facts"`
}

// WriteJSON serializes the report as JSON to the given writer.
func WriteJSON(
	w io.Writer,
	repoPath string,
	facts []extractor.Fact,
	manifest *extractor.ManifestData,
) error {
	report := Report{
		SchemaVersion: "v3",
		RepoPath:      repoPath,
		ExtractedAt:   time.Now().UTC().Format(time.RFC3339),
		Manifest:      manifest,
		Facts:         facts,
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")

	if err := enc.Encode(report); err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}

	return nil
}

// WritePretty writes a human-readable summary to the given writer.
func WritePretty(
	w io.Writer,
	repoPath string,
	facts []extractor.Fact,
) error {
	fmt.Fprintf(w, "Operator Analysis: %s\n", repoPath)
	fmt.Fprintf(w, "Extracted at: %s\n", time.Now().UTC().Format(time.RFC3339))
	fmt.Fprintf(w, "Total facts: %d\n\n", len(facts))

	// Group by kind
	byKind := map[string][]extractor.Fact{}
	for _, f := range facts {
		byKind[f.Kind] = append(byKind[f.Kind], f)
	}

	for kind, kindFacts := range byKind {
		fmt.Fprintf(w, "=== %s (%d) ===\n", kind, len(kindFacts))
		for _, f := range kindFacts {
			data, _ := json.MarshalIndent(f.Data, "  ", "  ")
			fmt.Fprintf(w, "  %s:%d\n  %s\n\n", f.File, f.Line, data)
		}
	}

	return nil
}
