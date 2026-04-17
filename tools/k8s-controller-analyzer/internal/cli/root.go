package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// Execute runs the Cobra root command.
func Execute(args []string) error {
	cmd := NewRootCmd()
	cmd.SetArgs(normalizeArgs(args))
	return formatArgError(cmd.Execute())
}

// NewRootCmd creates the single-root analyzer CLI command.
func NewRootCmd() *cobra.Command {
	opts := Options{}

	cmd := &cobra.Command{
		Use:           "k8s-controller-analyzer [flags] <repo-path>",
		Short:         "Extract structured facts from Kubernetes controller repos",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.ExactArgs(1),
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
		"skill name for manifest: architecture, api, production-readiness",
	)
	cmd.Flags().BoolVar(
		&opts.StrictLoad,
		"strict-load",
		false,
		"fail if go/packages reports package load/type errors",
	)

	return cmd
}

// normalizeArgs keeps compatibility with "<repo> [flags]" by converting it to
// a flags-first shape that Cobra parses reliably with exact positional args.
func normalizeArgs(args []string) []string {
	if len(args) == 0 {
		return args
	}

	normalized := normalizeLegacyLongFlags(args)
	if strings.HasPrefix(normalized[0], "-") {
		return normalized
	}

	repoArg := normalized[0]
	rest := normalized[1:]
	if len(rest) == 0 {
		return normalized
	}

	// If there is another positional arg in the tail, preserve original order
	// and let Cobra produce the argument error.
	for _, a := range rest {
		if !strings.HasPrefix(a, "-") {
			return normalized
		}
	}

	return append(rest, repoArg)
}

func normalizeLegacyLongFlags(args []string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		switch {
		case a == "-rules":
			out = append(out, "--rules")
		case a == "-format":
			out = append(out, "--format")
		case a == "-out":
			out = append(out, "--out")
		case a == "-skill":
			out = append(out, "--skill")
		case a == "-strict-load":
			out = append(out, "--strict-load")
		case strings.HasPrefix(a, "-rules="):
			out = append(out, "--rules="+strings.TrimPrefix(a, "-rules="))
		case strings.HasPrefix(a, "-format="):
			out = append(out, "--format="+strings.TrimPrefix(a, "-format="))
		case strings.HasPrefix(a, "-out="):
			out = append(out, "--out="+strings.TrimPrefix(a, "-out="))
		case strings.HasPrefix(a, "-skill="):
			out = append(out, "--skill="+strings.TrimPrefix(a, "-skill="))
		case strings.HasPrefix(a, "-strict-load="):
			out = append(out, "--strict-load="+strings.TrimPrefix(a, "-strict-load="))
		default:
			out = append(out, a)
		}
	}

	return out
}

func formatArgError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if strings.Contains(msg, "accepts 1 arg(s), received 0") {
		return fmt.Errorf("missing repo path")
	}
	return err
}
