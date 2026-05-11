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

func TestEnsureRecallDirCreatesDirectory(t *testing.T) {
	repoRoot := t.TempDir()
	recallDir := filepath.Join(repoRoot, ".recall")

	got, err := ensureRecallDir(repoRoot)
	if err != nil {
		t.Fatalf("ensureRecallDir returned error: %v", err)
	}

	if got != recallDir {
		t.Fatalf("ensureRecallDir() = %q, want %q", got, recallDir)
	}

	info, err := os.Stat(recallDir)
	if err != nil {
		t.Fatalf("failed to stat .recall directory: %v", err)
	}

	if !info.IsDir() {
		t.Fatalf(".recall exists but is not a directory")
	}
}

func TestEnsureRecallDirAcceptsExistingDirectory(t *testing.T) {
	repoRoot := t.TempDir()
	recallDir := filepath.Join(repoRoot, ".recall")
	if err := os.Mkdir(recallDir, 0o755); err != nil {
		t.Fatalf("failed to create .recall directory: %v", err)
	}

	got, err := ensureRecallDir(repoRoot)
	if err != nil {
		t.Fatalf("ensureRecallDir returned error: %v", err)
	}

	if got != recallDir {
		t.Fatalf("ensureRecallDir() = %q, want %q", got, recallDir)
	}
}

func TestEnsureRecallDirRejectsFile(t *testing.T) {
	repoRoot := t.TempDir()
	recallPath := filepath.Join(repoRoot, ".recall")
	if err := os.WriteFile(recallPath, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("failed to create .recall file: %v", err)
	}

	if got, err := ensureRecallDir(repoRoot); err == nil {
		t.Fatalf("ensureRecallDir() = %q, nil; want error", got)
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
