package cli

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeLegacyLongFlags(t *testing.T) {
	args := []string{
		"-rules",
		"crd-version-coverage",
		"-format=pretty",
		"-out",
		"/tmp/out.json",
		"-skill=api",
		"-strict-load",
		"./repo",
	}
	got := normalizeLegacyLongFlags(args)
	want := []string{
		"--rules",
		"crd-version-coverage",
		"--format=pretty",
		"--out",
		"/tmp/out.json",
		"--skill=api",
		"--strict-load",
		"./repo",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeLegacyLongFlags mismatch\n got: %v\nwant: %v", got, want)
	}
}

func TestNormalizeArgsPathFirst(t *testing.T) {
	args := []string{"./repo", "--rules", "crd-version-coverage", "--strict-load"}
	got := normalizeArgs(args)
	want := []string{"./repo", "--rules", "crd-version-coverage", "--strict-load"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeArgs mismatch\n got: %v\nwant: %v", got, want)
	}
}

func TestNormalizeArgsKeepsFlagsFirst(t *testing.T) {
	args := []string{"--rules", "crd-version-coverage", "./repo"}
	got := normalizeArgs(args)
	if !reflect.DeepEqual(got, args) {
		t.Fatalf("expected flags-first args unchanged\n got: %v\nwant: %v", got, args)
	}
}

func TestNormalizeArgsPreservesMultiplePositionals(t *testing.T) {
	args := []string{"./repo", "extra"}
	got := normalizeArgs(args)
	if !reflect.DeepEqual(got, args) {
		t.Fatalf("expected multiple positional args unchanged\n got: %v\nwant: %v", got, args)
	}
}

func TestFormatArgErrorMissingRepoPath(t *testing.T) {
	cmd := NewRootCmd()
	cmd.SetArgs([]string{})
	err := formatArgError(cmd.Execute())
	if err == nil || !strings.Contains(err.Error(), "missing repo path") {
		t.Fatalf("expected missing repo path error, got: %v", err)
	}
}

func TestFormatArgErrorPassThrough(t *testing.T) {
	orig := errors.New("some other error")
	got := formatArgError(orig)
	if !errors.Is(got, orig) {
		t.Fatalf("expected original error to pass through, got: %v", got)
	}
}
