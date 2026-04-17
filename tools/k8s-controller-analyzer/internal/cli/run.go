package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lburgazzoli/claude-plugins/tools/k8s-controller-analyzer/pkg/extractor"
	"github.com/lburgazzoli/claude-plugins/tools/k8s-controller-analyzer/pkg/loader"
	"github.com/lburgazzoli/claude-plugins/tools/k8s-controller-analyzer/pkg/output"
)

// Options captures CLI parameters for the analyzer run.
type Options struct {
	RepoPath   string
	Rules      string
	Format     string
	OutFile    string
	Skill      string
	StrictLoad bool
}

// Run executes the end-to-end analyzer flow.
func Run(opts Options) error {
	repoPath, err := filepath.Abs(opts.RepoPath)
	if err != nil {
		return err
	}

	pkgs, err := loader.Load(repoPath, opts.StrictLoad)
	if err != nil {
		return fmt.Errorf("error loading packages: %w", err)
	}

	if opts.Skill != "" && !extractor.ValidSkills[opts.Skill] {
		return fmt.Errorf("unknown skill %q (valid: architecture, api, production-readiness)", opts.Skill)
	}

	var facts []extractor.Fact
	facts = append(facts, extractor.ExtractControllers(pkgs, repoPath)...)
	facts = append(facts, extractor.ExtractCRDVersions(pkgs, repoPath)...)
	facts = append(facts, extractor.ExtractAPIFields(pkgs, repoPath)...)
	facts = append(facts, extractor.ExtractWebhooks(pkgs, repoPath)...)
	facts = append(facts, extractor.ExtractSchemeRegistrations(pkgs, repoPath)...)
	facts = append(facts, extractor.ExtractImports(pkgs, repoPath)...)

	walk, err := extractor.WalkRepo(repoPath)
	if err != nil {
		return fmt.Errorf("error walking repo: %w", err)
	}

	facts = append(facts, extractor.ExtractRBACManifests(walk.YAMLDocs)...)
	facts = append(facts, extractor.ExtractCRDManifests(walk.YAMLDocs)...)
	facts = append(facts, extractor.ExtractWebhookManifests(walk.YAMLDocs)...)
	facts = append(facts, extractor.ExtractDeploymentManifests(walk.YAMLDocs)...)
	facts = append(facts, extractor.ExtractNetworkPolicyManifests(walk.YAMLDocs)...)

	if len(walk.TestFiles) > 0 {
		facts = append(facts, extractor.NewFact(
			extractor.RuleTestDiscovery,
			extractor.KindTestDiscovery,
			"",
			0,
			extractor.TestDiscoveryData{
				Files: walk.TestFiles,
				Count: len(walk.TestFiles),
			},
		))
	}

	var manifest *extractor.ManifestData
	if opts.Skill != "" {
		m := extractor.BuildManifest(opts.Skill, pkgs, walk, repoPath)
		manifest = &m
	}

	if opts.Rules != "" && opts.Rules != "all" {
		ruleSet := map[string]bool{}
		for _, r := range strings.Split(opts.Rules, ",") {
			ruleSet[strings.TrimSpace(r)] = true
		}
		facts = filterByRules(facts, ruleSet)
	}

	w := os.Stdout
	if opts.OutFile != "" {
		f, err := os.Create(opts.OutFile)
		if err != nil {
			return fmt.Errorf("error creating output file: %w", err)
		}
		defer f.Close()
		w = f
	}

	format := opts.Format
	if format == "" {
		format = "json"
	}

	switch format {
	case "json":
		if err := output.WriteJSON(w, repoPath, facts, manifest); err != nil {
			return err
		}
	case "pretty":
		if err := output.WritePretty(w, repoPath, facts); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown format: %s", format)
	}

	return nil
}

func filterByRules(
	facts []extractor.Fact,
	ruleSet map[string]bool,
) []extractor.Fact {
	var filtered []extractor.Fact

	for _, f := range facts {
		for _, rule := range f.Rules {
			if ruleSet[rule] {
				filtered = append(filtered, f)
				break
			}
		}
	}

	return filtered
}
