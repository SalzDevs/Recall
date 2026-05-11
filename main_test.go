package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestFindGitRootFromRepoRoot(t *testing.T) {
	repoRoot := initTestRepo(t)

	got, err := findGitRoot(repoRoot)
	if err != nil {
		t.Fatalf("findGitRoot returned error: %v", err)
	}

	if got != repoRoot {
		t.Fatalf("findGitRoot() = %q, want %q", got, repoRoot)
	}
}

func TestFindGitRootFromSubdirectory(t *testing.T) {
	repoRoot := initTestRepo(t)
	subdir := filepath.Join(repoRoot, "internal", "auth")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}

	got, err := findGitRoot(subdir)
	if err != nil {
		t.Fatalf("findGitRoot returned error: %v", err)
	}

	if got != repoRoot {
		t.Fatalf("findGitRoot() = %q, want %q", got, repoRoot)
	}
}

func TestFindGitRootOutsideRepo(t *testing.T) {
	nonRepoDir := t.TempDir()

	if got, err := findGitRoot(nonRepoDir); err == nil {
		t.Fatalf("findGitRoot() = %q, nil; want error", got)
	}
}

func initTestRepo(t *testing.T) string {
	t.Helper()

	repoRoot := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = repoRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to initialize test git repo: %v\n%s", err, string(output))
	}

	cleanRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		t.Fatalf("failed to resolve repo root: %v", err)
	}

	return filepath.Clean(cleanRoot)
}
