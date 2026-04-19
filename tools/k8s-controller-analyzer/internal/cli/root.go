package cli

import (
	"github.com/spf13/cobra"
)

// Execute runs the Cobra root command.
func Execute(args []string) error {
	cmd := NewRootCmd()
	cmd.SetArgs(args)
	return cmd.Execute()
}

// NewRootCmd creates the root command with analyze and mcp subcommands.
func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "k8s-controller-analyzer",
		Short:         "Extract structured facts from Kubernetes controller repos",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.AddCommand(NewAnalyzeCmd())
	cmd.AddCommand(NewMCPCmd())

	return cmd
}

// NewAnalyzeCmd creates the "analyze" subcommand.
func NewAnalyzeCmd() *cobra.Command {
	opts := Options{}

	cmd := &cobra.Command{
		Use:   "analyze [flags] <repo-path>",
		Short: "Analyze a Kubernetes controller repository",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.RepoPath = args[0]
			return Run(opts)
		},
	}

	cmd.Flags().StringVar(
		&opts.Rules,
		"rules",
		"all",
		"comma-separated list of rules to extract for",
	)
	cmd.Flags().StringVar(
		&opts.Format,
		"format",
		"json",
		"output format: json or pretty",
	)
	cmd.Flags().StringVar(
		&opts.OutFile,
		"out",
		"",
		"output file path (default: stdout)",
	)
	cmd.Flags().StringVar(
		&opts.Skill,
		"skill",
		"",
		"skill name for manifest: architecture, api, lifecycle, production-readiness",
	)
	cmd.Flags().BoolVar(
		&opts.StrictLoad,
		"strict-load",
		false,
		"fail if go/packages reports package load/type errors",
	)

	return cmd
}
