package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func TestGetGitStateReturnsBranchAndCommit(t *testing.T) {
	repoRoot := initTestRepo(t)
	wantCommit := commitTestFile(t, repoRoot)
	wantBranch := gitOutput(t, repoRoot, "rev-parse", "--abbrev-ref", "HEAD")

	got, err := getGitState(repoRoot)
	if err != nil {
		t.Fatalf("getGitState returned error: %v", err)
	}

	if got.Branch != wantBranch {
		t.Fatalf("branch = %q, want %q", got.Branch, wantBranch)
	}
	if got.Commit != wantCommit {
		t.Fatalf("commit = %q, want %q", got.Commit, wantCommit)
	}
}

func TestGetGitStateRequiresCommit(t *testing.T) {
	repoRoot := initTestRepo(t)

	if got, err := getGitState(repoRoot); err == nil {
		t.Fatalf("getGitState() = %+v, nil; want error", got)
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

func TestNewActiveSessionUsesGoalAndGitState(t *testing.T) {
	state := gitState{Branch: "main", Commit: "abc123"}

	session, err := newActiveSession("  build auth flow  ", state)
	if err != nil {
		t.Fatalf("newActiveSession returned error: %v", err)
	}

	if session.ID == "" {
		t.Fatalf("session ID is empty")
	}
	if session.Goal != "build auth flow" {
		t.Fatalf("goal = %q, want %q", session.Goal, "build auth flow")
	}
	if session.Branch != state.Branch {
		t.Fatalf("branch = %q, want %q", session.Branch, state.Branch)
	}
	if session.BaseCommit != state.Commit {
		t.Fatalf("baseCommit = %q, want %q", session.BaseCommit, state.Commit)
	}
	if session.Status != sessionStatusActive {
		t.Fatalf("status = %q, want %q", session.Status, sessionStatusActive)
	}
	if _, err := time.Parse(time.RFC3339, session.StartedAt); err != nil {
		t.Fatalf("startedAt = %q, want RFC3339 timestamp: %v", session.StartedAt, err)
	}
}

func TestNewActiveSessionRequiresGoal(t *testing.T) {
	state := gitState{Branch: "main", Commit: "abc123"}

	if session, err := newActiveSession("   ", state); err == nil {
		t.Fatalf("newActiveSession() = %+v, nil; want error", session)
	}
}

func TestWriteActiveSessionCreatesFile(t *testing.T) {
	recallDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(recallDir, "sessions"), 0o755); err != nil {
		t.Fatalf("failed to create sessions directory: %v", err)
	}

	session := recallSession{
		ID:         "20260511T120000Z",
		Goal:       "build auth flow",
		StartedAt:  "2026-05-11T12:00:00Z",
		Branch:     "main",
		BaseCommit: "abc123",
		Status:     sessionStatusActive,
	}

	got, err := writeActiveSession(recallDir, session)
	if err != nil {
		t.Fatalf("writeActiveSession returned error: %v", err)
	}

	wantPath := filepath.Join(recallDir, "sessions", activeSessionFileName)
	if got != wantPath {
		t.Fatalf("writeActiveSession() = %q, want %q", got, wantPath)
	}

	data, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("failed to read active session: %v", err)
	}

	var gotSession recallSession
	if err := json.Unmarshal(data, &gotSession); err != nil {
		t.Fatalf("failed to decode active session: %v", err)
	}
	if gotSession != session {
		t.Fatalf("active session = %+v, want %+v", gotSession, session)
	}
}

func TestWriteActiveSessionRejectsExistingFile(t *testing.T) {
	recallDir := t.TempDir()
	sessionsDir := filepath.Join(recallDir, "sessions")
	if err := os.Mkdir(sessionsDir, 0o755); err != nil {
		t.Fatalf("failed to create sessions directory: %v", err)
	}

	activePath := filepath.Join(sessionsDir, activeSessionFileName)
	original := []byte("existing session")
	if err := os.WriteFile(activePath, original, 0o644); err != nil {
		t.Fatalf("failed to create existing active session: %v", err)
	}

	if got, err := writeActiveSession(recallDir, recallSession{}); err == nil {
		t.Fatalf("writeActiveSession() = %q, nil; want error", got)
	}

	data, err := os.ReadFile(activePath)
	if err != nil {
		t.Fatalf("failed to read active session: %v", err)
	}
	if string(data) != string(original) {
		t.Fatalf("active session was overwritten: got %q, want %q", string(data), string(original))
	}
}

func TestWriteActiveSessionRejectsDirectory(t *testing.T) {
	recallDir := t.TempDir()
	activePath := filepath.Join(recallDir, "sessions", activeSessionFileName)
	if err := os.MkdirAll(activePath, 0o755); err != nil {
		t.Fatalf("failed to create active session directory: %v", err)
	}

	if got, err := writeActiveSession(recallDir, recallSession{}); err == nil {
		t.Fatalf("writeActiveSession() = %q, nil; want error", got)
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

func commitTestFile(t *testing.T, repoRoot string) string {
	t.Helper()

	filePath := filepath.Join(repoRoot, "README.md")
	if err := os.WriteFile(filePath, []byte("# Test Repo\n"), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	runGit(t, repoRoot, "add", "README.md")
	runGit(t, repoRoot, "-c", "user.name=Recall Test", "-c", "user.email=recall@example.com", "commit", "-m", "initial commit")

	return gitOutput(t, repoRoot, "rev-parse", "HEAD")
}

func runGit(t *testing.T, repoRoot string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", append([]string{"-C", repoRoot}, args...)...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
	}
}

func gitOutput(t *testing.T, repoRoot string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", append([]string{"-C", repoRoot}, args...)...)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v failed: %v", args, err)
	}

	return strings.TrimSpace(string(output))
}
