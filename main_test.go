package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
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

func TestEnsureRecallSubdirsCreatesDirectories(t *testing.T) {
	recallDir := t.TempDir()

	if err := ensureRecallSubdirs(recallDir); err != nil {
		t.Fatalf("ensureRecallSubdirs returned error: %v", err)
	}

	for _, subdir := range recallSubdirs {
		path := filepath.Join(recallDir, subdir)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("failed to stat %s: %v", path, err)
		}
		if !info.IsDir() {
			t.Fatalf("%s exists but is not a directory", path)
		}
	}
}

func TestEnsureRecallSubdirsAcceptsExistingDirectories(t *testing.T) {
	recallDir := t.TempDir()
	for _, subdir := range recallSubdirs {
		if err := os.Mkdir(filepath.Join(recallDir, subdir), 0o755); err != nil {
			t.Fatalf("failed to create existing %s directory: %v", subdir, err)
		}
	}

	if err := ensureRecallSubdirs(recallDir); err != nil {
		t.Fatalf("ensureRecallSubdirs returned error: %v", err)
	}
}

func TestEnsureRecallSubdirsRejectsFile(t *testing.T) {
	recallDir := t.TempDir()
	path := filepath.Join(recallDir, recallSubdirs[0])
	if err := os.WriteFile(path, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("failed to create subdir file: %v", err)
	}

	if err := ensureRecallSubdirs(recallDir); err == nil {
		t.Fatalf("ensureRecallSubdirs returned nil; want error")
	}
}

func TestWriteDefaultConfigCreatesConfig(t *testing.T) {
	recallDir := t.TempDir()
	configPath := filepath.Join(recallDir, configFileName)

	got, err := writeDefaultConfig(recallDir, "TestProject")
	if err != nil {
		t.Fatalf("writeDefaultConfig returned error: %v", err)
	}

	if got != configPath {
		t.Fatalf("writeDefaultConfig() = %q, want %q", got, configPath)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config.json: %v", err)
	}

	var config recallConfig
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("failed to decode config.json: %v", err)
	}

	if config.SchemaVersion != 1 {
		t.Fatalf("schemaVersion = %d, want 1", config.SchemaVersion)
	}
	if config.ProjectName != "TestProject" {
		t.Fatalf("projectName = %q, want %q", config.ProjectName, "TestProject")
	}
	if _, err := time.Parse(time.RFC3339, config.CreatedAt); err != nil {
		t.Fatalf("createdAt = %q, want RFC3339 timestamp: %v", config.CreatedAt, err)
	}
}

func TestWriteDefaultConfigDoesNotOverwriteExistingConfig(t *testing.T) {
	recallDir := t.TempDir()
	configPath := filepath.Join(recallDir, configFileName)
	original := []byte("existing config")
	if err := os.WriteFile(configPath, original, 0o644); err != nil {
		t.Fatalf("failed to create existing config.json: %v", err)
	}

	got, err := writeDefaultConfig(recallDir, "TestProject")
	if err != nil {
		t.Fatalf("writeDefaultConfig returned error: %v", err)
	}
	if got != configPath {
		t.Fatalf("writeDefaultConfig() = %q, want %q", got, configPath)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config.json: %v", err)
	}
	if string(data) != string(original) {
		t.Fatalf("config.json was overwritten: got %q, want %q", string(data), string(original))
	}
}

func TestWriteDefaultConfigRejectsDirectory(t *testing.T) {
	recallDir := t.TempDir()
	configPath := filepath.Join(recallDir, configFileName)
	if err := os.Mkdir(configPath, 0o755); err != nil {
		t.Fatalf("failed to create config.json directory: %v", err)
	}

	if got, err := writeDefaultConfig(recallDir, "TestProject"); err == nil {
		t.Fatalf("writeDefaultConfig() = %q, nil; want error", got)
	}
}

func TestRunInitInitializesRecallInGitRoot(t *testing.T) {
	repoRoot := initTestRepo(t)
	subdir := filepath.Join(repoRoot, "internal", "auth")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Fatalf("failed to restore current directory: %v", err)
		}
	}()

	if err := os.Chdir(subdir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	recallDir, configPath, err := runInit()
	if err != nil {
		t.Fatalf("runInit returned error: %v", err)
	}

	wantRecallDir := filepath.Join(repoRoot, ".recall")
	if recallDir != wantRecallDir {
		t.Fatalf("recallDir = %q, want %q", recallDir, wantRecallDir)
	}
	if configPath != filepath.Join(wantRecallDir, configFileName) {
		t.Fatalf("configPath = %q, want %q", configPath, filepath.Join(wantRecallDir, configFileName))
	}

	for _, path := range append([]string{configPath}, recallSubdirsPaths(wantRecallDir)...) {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}
}

func recallSubdirsPaths(recallDir string) []string {
	paths := make([]string, 0, len(recallSubdirs))
	for _, subdir := range recallSubdirs {
		paths = append(paths, filepath.Join(recallDir, subdir))
	}
	return paths
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
