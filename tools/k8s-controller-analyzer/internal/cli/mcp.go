package cli

import (
	"bytes"
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

type analyzeInput struct {
	RepoPath string `json:"repo_path" jsonschema:"absolute path to the Kubernetes controller repository to analyze"`
	Skill    string `json:"skill,omitempty" jsonschema:"skill name for manifest generation: architecture, api, lifecycle, or production-readiness"`
	Rules    string `json:"rules,omitempty" jsonschema:"comma-separated list of rules to extract (default: all)"`
}

func analyze(
	ctx context.Context,
	req *mcp.CallToolRequest,
	args analyzeInput,
) (*mcp.CallToolResult, any, error) {
	var buf bytes.Buffer

	opts := Options{
		RepoPath: args.RepoPath,
		Skill:    args.Skill,
		Rules:    args.Rules,
		Format:   "json",
	}

	if err := RunTo(&buf, opts); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "error: " + err.Error()},
			},
			IsError: true,
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: buf.String()},
		},
	}, nil, nil
}

// NewMCPCmd creates the "mcp" subcommand that runs the analyzer as an MCP server.
func NewMCPCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Run as an MCP (Model Context Protocol) server over stdio",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			server := mcp.NewServer(
				&mcp.Implementation{
					Name:    "k8s-controller-analyzer",
					Version: "0.1.0",
				},
				nil,
			)

			mcp.AddTool(server, &mcp.Tool{
				Name:        "analyze_controller",
				Description: "Extract structured facts from a Kubernetes controller repository for architecture, API conventions, lifecycle, and production readiness assessment",
			}, analyze)

			return server.Run(cmd.Context(), &mcp.StdioTransport{})
		},
	}
}
