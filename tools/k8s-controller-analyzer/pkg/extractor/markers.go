package extractor

import (
	"go/ast"
	"go/token"
	"strings"
)

const markerPrefix = "// +"

// Marker represents a single parsed kubebuilder marker.
type Marker struct {
	Raw  string            // full text after "// +", e.g. "kubebuilder:rbac:groups=apps,..."
	Name string            // marker name, e.g. "kubebuilder:rbac"
	Args map[string]string // parsed key=value pairs
	Line int               // source line number
}

// ExtractMarkersFromDoc extracts all kubebuilder markers from a comment group.
func ExtractMarkersFromDoc(
	cg *ast.CommentGroup,
	fset *token.FileSet,
) []Marker {
	if cg == nil {
		return nil
	}

	var markers []Marker
	for _, c := range cg.List {
		if !strings.HasPrefix(c.Text, markerPrefix) {
			continue
		}
		raw := strings.TrimPrefix(c.Text, "// +")
		markers = append(markers, Marker{
			Raw:  raw,
			Name: markerName(raw),
			Args: markerArgs(raw),
			Line: fset.Position(c.Slash).Line,
		})
	}

	return markers
}

// ExtractMarkersFromGroups extracts kubebuilder markers from multiple comment groups.
func ExtractMarkersFromGroups(
	groups []*ast.CommentGroup,
	fset *token.FileSet,
) []Marker {
	var markers []Marker
	for _, cg := range groups {
		markers = append(markers, ExtractMarkersFromDoc(cg, fset)...)
	}

	return markers
}

// DocOrNearby returns the Doc comment group if present, otherwise scans
// the file's comments for one within 3 lines above the declaration.
func DocOrNearby(
	file *ast.File,
	fset *token.FileSet,
	pos token.Pos,
	doc *ast.CommentGroup,
) *ast.CommentGroup {
	if doc != nil {
		return doc
	}

	declLine := fset.Position(pos).Line
	for _, cg := range file.Comments {
		lastLine := fset.Position(cg.End()).Line
		if lastLine >= declLine-3 && lastLine < declLine {
			return cg
		}
	}

	return nil
}

// CollectNearbyCommentGroups returns all comment groups within maxGap lines
// above the declaration, starting from doc (if present) and scanning upward.
// This handles the common kubebuilder pattern where markers are in a separate
// comment group above the doc comment, separated by a blank line.
func CollectNearbyCommentGroups(
	file *ast.File,
	fset *token.FileSet,
	pos token.Pos,
	doc *ast.CommentGroup,
	maxGap int,
) []*ast.CommentGroup {
	declLine := fset.Position(pos).Line

	// Collect all comment groups that end before the declaration.
	var candidates []*ast.CommentGroup
	for _, cg := range file.Comments {
		lastLine := fset.Position(cg.End()).Line
		if lastLine < declLine {
			candidates = append(candidates, cg)
		}
	}

	if len(candidates) == 0 {
		if doc != nil {
			return []*ast.CommentGroup{doc}
		}
		return nil
	}

	// Walk backwards from the declaration, collecting groups within maxGap
	// of each other (or within maxGap of the declaration line).
	var result []*ast.CommentGroup
	prevStart := declLine
	for i := len(candidates) - 1; i >= 0; i-- {
		cg := candidates[i]
		lastLine := fset.Position(cg.End()).Line
		if prevStart-lastLine > maxGap {
			break
		}
		result = append(result, cg)
		prevStart = fset.Position(cg.Pos()).Line
	}

	// Reverse to maintain source order.
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result
}

// markerName returns the marker identifier before the first key=value segment.
// For "kubebuilder:rbac:groups=apps,..." → "kubebuilder:rbac"
// For "kubebuilder:object:root=true" → "kubebuilder:object:root"
// For "groupName=example.com" → "groupName"
func markerName(raw string) string {
	parts := strings.Split(raw, ":")
	for i, p := range parts {
		if strings.Contains(p, "=") {
			// Extract the key name before '=' from this segment
			key, _, _ := strings.Cut(p, "=")
			// If this segment has comma-separated key=value pairs, it's purely args
			if strings.Contains(p, ",") {
				return strings.Join(parts[:i], ":")
			}
			// Single key=value: include the key as part of the name
			prefix := strings.Join(parts[:i], ":")
			if prefix == "" {
				return key
			}
			return prefix + ":" + key
		}
	}

	return raw
}

// markerArgs parses the key=value pairs from a marker string.
func markerArgs(raw string) map[string]string {
	args := map[string]string{}

	parts := strings.Split(raw, ":")
	argIdx := -1
	for i, p := range parts {
		if strings.Contains(p, "=") {
			argIdx = i
			break
		}
	}

	if argIdx < 0 {
		return args
	}

	argStr := strings.Join(parts[argIdx:], ":")
	for _, kv := range strings.Split(argStr, ",") {
		if key, val, ok := strings.Cut(kv, "="); ok {
			args[key] = val
		}
	}

	return args
}
