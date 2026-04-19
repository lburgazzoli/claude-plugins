package cli

import (
	"testing"
)

func TestNewRootCmdHasSubcommands(t *testing.T) {
	cmd := NewRootCmd()

	names := map[string]bool{}
	for _, sub := range cmd.Commands() {
		names[sub.Name()] = true
	}

	for _, want := range []string{"analyze", "mcp"} {
		if !names[want] {
			t.Errorf("missing subcommand %q", want)
		}
	}
}

func TestAnalyzeCmdRequiresRepoPath(t *testing.T) {
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"analyze"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when repo path is missing")
	}
}
