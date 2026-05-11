package main

import (
	"encoding/json"
	"errors"
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

func TestGetChangedFilesReturnsShortStatusLines(t *testing.T) {
	repoRoot := initTestRepo(t)
	commitTestFile(t, repoRoot)

	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("# Changed\n"), 0o644); err != nil {
		t.Fatalf("failed to modify tracked file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "new.txt"), []byte("new file\n"), 0o644); err != nil {
		t.Fatalf("failed to write untracked file: %v", err)
	}

	changedFiles, err := getChangedFiles(repoRoot)
	if err != nil {
		t.Fatalf("getChangedFiles returned error: %v", err)
	}

	if !containsLine(changedFiles, " M README.md") {
		t.Fatalf("changed files = %v, want modified README.md", changedFiles)
	}
	if !containsLine(changedFiles, "?? new.txt") {
		t.Fatalf("changed files = %v, want untracked new.txt", changedFiles)
	}
}

func TestGetChangedFilesReturnsEmptySliceForCleanRepo(t *testing.T) {
	repoRoot := initTestRepo(t)
	commitTestFile(t, repoRoot)

	changedFiles, err := getChangedFiles(repoRoot)
	if err != nil {
		t.Fatalf("getChangedFiles returned error: %v", err)
	}
	if len(changedFiles) != 0 {
		t.Fatalf("changed files = %v, want empty slice", changedFiles)
	}
}

func TestGetDiffStatsReturnsShortStatSinceBaseCommit(t *testing.T) {
	repoRoot := initTestRepo(t)
	baseCommit := commitTestFile(t, repoRoot)

	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("# Changed\n"), 0o644); err != nil {
		t.Fatalf("failed to modify tracked file: %v", err)
	}

	diffStats, err := getDiffStats(repoRoot, baseCommit)
	if err != nil {
		t.Fatalf("getDiffStats returned error: %v", err)
	}
	if !strings.Contains(diffStats, "1 file changed") {
		t.Fatalf("diff stats = %q, want file change count", diffStats)
	}
}

func TestGetDiffStatsReturnsEmptyStringForNoTrackedChanges(t *testing.T) {
	repoRoot := initTestRepo(t)
	baseCommit := commitTestFile(t, repoRoot)

	diffStats, err := getDiffStats(repoRoot, baseCommit)
	if err != nil {
		t.Fatalf("getDiffStats returned error: %v", err)
	}
	if diffStats != "" {
		t.Fatalf("diff stats = %q, want empty string", diffStats)
	}
}

func TestEnsureRecallIgnoredCreatesGitignore(t *testing.T) {
	repoRoot := t.TempDir()

	gitignorePath, err := ensureRecallIgnored(repoRoot)
	if err != nil {
		t.Fatalf("ensureRecallIgnored returned error: %v", err)
	}
	if gitignorePath != filepath.Join(repoRoot, ".gitignore") {
		t.Fatalf("gitignorePath = %q, want .gitignore path", gitignorePath)
	}

	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("failed to read .gitignore: %v", err)
	}
	if string(data) != ".recall/\n" {
		t.Fatalf(".gitignore = %q, want .recall entry", string(data))
	}
}

func TestEnsureRecallIgnoredAppendsEntry(t *testing.T) {
	repoRoot := t.TempDir()
	gitignorePath := filepath.Join(repoRoot, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte("dist/"), 0o644); err != nil {
		t.Fatalf("failed to write .gitignore: %v", err)
	}

	if _, err := ensureRecallIgnored(repoRoot); err != nil {
		t.Fatalf("ensureRecallIgnored returned error: %v", err)
	}

	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("failed to read .gitignore: %v", err)
	}
	if string(data) != "dist/\n.recall/\n" {
		t.Fatalf(".gitignore = %q, want appended .recall entry", string(data))
	}
}

func TestEnsureRecallIgnoredDoesNotDuplicateEntry(t *testing.T) {
	repoRoot := t.TempDir()
	gitignorePath := filepath.Join(repoRoot, ".gitignore")
	original := []byte("dist/\n.recall/\n")
	if err := os.WriteFile(gitignorePath, original, 0o644); err != nil {
		t.Fatalf("failed to write .gitignore: %v", err)
	}

	if _, err := ensureRecallIgnored(repoRoot); err != nil {
		t.Fatalf("ensureRecallIgnored returned error: %v", err)
	}

	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("failed to read .gitignore: %v", err)
	}
	if string(data) != string(original) {
		t.Fatalf(".gitignore = %q, want unchanged %q", string(data), string(original))
	}
}

func TestEnsureRecallIgnoredRejectsDirectory(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(repoRoot, ".gitignore"), 0o755); err != nil {
		t.Fatalf("failed to create .gitignore directory: %v", err)
	}

	if gitignorePath, err := ensureRecallIgnored(repoRoot); err == nil {
		t.Fatalf("ensureRecallIgnored() = %q, nil; want error", gitignorePath)
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

func TestGetRecallDirReturnsInitializedRecallDir(t *testing.T) {
	repoRoot := t.TempDir()
	recallDir, err := ensureRecallDir(repoRoot)
	if err != nil {
		t.Fatalf("ensureRecallDir returned error: %v", err)
	}
	if err := ensureRecallSubdirs(recallDir); err != nil {
		t.Fatalf("ensureRecallSubdirs returned error: %v", err)
	}
	if _, err := writeDefaultConfig(recallDir, "TestProject"); err != nil {
		t.Fatalf("writeDefaultConfig returned error: %v", err)
	}

	got, err := getRecallDir(repoRoot)
	if err != nil {
		t.Fatalf("getRecallDir returned error: %v", err)
	}
	if got != recallDir {
		t.Fatalf("getRecallDir() = %q, want %q", got, recallDir)
	}
}

func TestGetRecallDirRejectsUninitializedRepo(t *testing.T) {
	repoRoot := t.TempDir()

	if got, err := getRecallDir(repoRoot); err == nil {
		t.Fatalf("getRecallDir() = %q, nil; want error", got)
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

func TestReadActiveSessionReturnsSession(t *testing.T) {
	recallDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(recallDir, "sessions"), 0o755); err != nil {
		t.Fatalf("failed to create sessions directory: %v", err)
	}

	want := recallSession{
		ID:         "20260511T120000Z",
		Goal:       "build auth flow",
		StartedAt:  "2026-05-11T12:00:00Z",
		Branch:     "main",
		BaseCommit: "abc123",
		Status:     sessionStatusActive,
	}
	if _, err := writeActiveSession(recallDir, want); err != nil {
		t.Fatalf("writeActiveSession returned error: %v", err)
	}

	got, err := readActiveSession(recallDir)
	if err != nil {
		t.Fatalf("readActiveSession returned error: %v", err)
	}
	if got != want {
		t.Fatalf("readActiveSession() = %+v, want %+v", got, want)
	}
}

func TestReadActiveSessionReturnsNoActiveSession(t *testing.T) {
	recallDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(recallDir, "sessions"), 0o755); err != nil {
		t.Fatalf("failed to create sessions directory: %v", err)
	}

	if session, err := readActiveSession(recallDir); !errors.Is(err, errNoActiveSession) {
		t.Fatalf("readActiveSession() = %+v, %v; want errNoActiveSession", session, err)
	}
}

func TestReadActiveSessionRejectsInvalidJSON(t *testing.T) {
	recallDir := t.TempDir()
	sessionsDir := filepath.Join(recallDir, "sessions")
	if err := os.Mkdir(sessionsDir, 0o755); err != nil {
		t.Fatalf("failed to create sessions directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sessionsDir, activeSessionFileName), []byte("not json"), 0o644); err != nil {
		t.Fatalf("failed to write invalid active session: %v", err)
	}

	if session, err := readActiveSession(recallDir); err == nil {
		t.Fatalf("readActiveSession() = %+v, nil; want error", session)
	}
}

func TestReadActiveSessionRejectsDirectory(t *testing.T) {
	recallDir := t.TempDir()
	activePath := filepath.Join(recallDir, "sessions", activeSessionFileName)
	if err := os.MkdirAll(activePath, 0o755); err != nil {
		t.Fatalf("failed to create active session directory: %v", err)
	}

	if session, err := readActiveSession(recallDir); err == nil {
		t.Fatalf("readActiveSession() = %+v, nil; want error", session)
	}
}

func TestStopActiveSessionArchivesAndRemovesActiveSession(t *testing.T) {
	recallDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(recallDir, "sessions"), 0o755); err != nil {
		t.Fatalf("failed to create sessions directory: %v", err)
	}

	activeSession := recallSession{
		ID:         "20260511T120000Z",
		Goal:       "build auth flow",
		StartedAt:  "2026-05-11T12:00:00Z",
		Branch:     "main",
		BaseCommit: "abc123",
		Status:     sessionStatusActive,
	}
	if _, err := writeActiveSession(recallDir, activeSession); err != nil {
		t.Fatalf("writeActiveSession returned error: %v", err)
	}

	stoppedSession, archivePath, err := stopActiveSession(recallDir)
	if err != nil {
		t.Fatalf("stopActiveSession returned error: %v", err)
	}

	if stoppedSession.Status != sessionStatusStopped {
		t.Fatalf("status = %q, want %q", stoppedSession.Status, sessionStatusStopped)
	}
	if stoppedSession.EndedAt == "" {
		t.Fatalf("endedAt is empty")
	}
	if _, err := time.Parse(time.RFC3339, stoppedSession.EndedAt); err != nil {
		t.Fatalf("endedAt = %q, want RFC3339 timestamp: %v", stoppedSession.EndedAt, err)
	}

	if archivePath != filepath.Join(recallDir, "sessions", activeSession.ID+".json") {
		t.Fatalf("archivePath = %q, want session archive path", archivePath)
	}
	if _, err := os.Stat(filepath.Join(recallDir, "sessions", activeSessionFileName)); !os.IsNotExist(err) {
		t.Fatalf("active session still exists or stat failed unexpectedly: %v", err)
	}

	data, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("failed to read archived session: %v", err)
	}
	var archivedSession recallSession
	if err := json.Unmarshal(data, &archivedSession); err != nil {
		t.Fatalf("failed to decode archived session: %v", err)
	}
	if archivedSession != stoppedSession {
		t.Fatalf("archived session = %+v, want %+v", archivedSession, stoppedSession)
	}
}

func TestStopActiveSessionReturnsNoActiveSession(t *testing.T) {
	recallDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(recallDir, "sessions"), 0o755); err != nil {
		t.Fatalf("failed to create sessions directory: %v", err)
	}

	if session, archivePath, err := stopActiveSession(recallDir); !errors.Is(err, errNoActiveSession) {
		t.Fatalf("stopActiveSession() = %+v, %q, %v; want errNoActiveSession", session, archivePath, err)
	}
}

func TestWriteCheckpointCreatesMarkdown(t *testing.T) {
	recallDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(recallDir, "checkpoints"), 0o755); err != nil {
		t.Fatalf("failed to create checkpoints directory: %v", err)
	}

	status := recallStatus{
		Session: recallSession{
			ID:         "20260511T120000Z",
			Goal:       "build auth flow",
			StartedAt:  "2026-05-11T12:00:00Z",
			Branch:     "main",
			BaseCommit: "abc123",
			Status:     sessionStatusActive,
		},
		ChangedFiles: []string{" M README.md", "?? new.txt"},
		DiffStats:    "1 file changed, 1 insertion(+)",
	}

	checkpointPath, err := writeCheckpoint(recallDir, status, "OAuth flow implemented")
	if err != nil {
		t.Fatalf("writeCheckpoint returned error: %v", err)
	}
	if filepath.Dir(checkpointPath) != filepath.Join(recallDir, "checkpoints") {
		t.Fatalf("checkpoint path = %q, want file in checkpoints directory", checkpointPath)
	}

	data, err := os.ReadFile(checkpointPath)
	if err != nil {
		t.Fatalf("failed to read checkpoint: %v", err)
	}
	content := string(data)
	for _, want := range []string{
		"# Recall Checkpoint",
		"Message: OAuth flow implemented",
		"Goal: build auth flow",
		"-  M README.md",
		"- ?? new.txt",
		"1 file changed, 1 insertion(+)",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("checkpoint content missing %q:\n%s", want, content)
		}
	}
}

func TestWriteCheckpointRequiresMessage(t *testing.T) {
	recallDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(recallDir, "checkpoints"), 0o755); err != nil {
		t.Fatalf("failed to create checkpoints directory: %v", err)
	}

	if checkpointPath, err := writeCheckpoint(recallDir, recallStatus{}, "   "); err == nil {
		t.Fatalf("writeCheckpoint() = %q, nil; want error", checkpointPath)
	}
}

func TestWriteCheckpointRejectsMissingCheckpointDir(t *testing.T) {
	recallDir := t.TempDir()

	if checkpointPath, err := writeCheckpoint(recallDir, recallStatus{}, "message"); err == nil {
		t.Fatalf("writeCheckpoint() = %q, nil; want error", checkpointPath)
	}
}

func TestWriteHandoffCreatesMarkdown(t *testing.T) {
	recallDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(recallDir, "handoffs"), 0o755); err != nil {
		t.Fatalf("failed to create handoffs directory: %v", err)
	}

	status := recallStatus{
		Session: recallSession{
			ID:         "20260511T120000Z",
			Goal:       "build auth flow",
			StartedAt:  "2026-05-11T12:00:00Z",
			Branch:     "main",
			BaseCommit: "abc123",
			Status:     sessionStatusActive,
		},
		ChangedFiles: []string{" M README.md"},
		DiffStats:    "1 file changed, 1 insertion(+)",
	}

	handoffPath, err := writeHandoff(recallDir, status)
	if err != nil {
		t.Fatalf("writeHandoff returned error: %v", err)
	}
	if filepath.Dir(handoffPath) != filepath.Join(recallDir, "handoffs") {
		t.Fatalf("handoff path = %q, want file in handoffs directory", handoffPath)
	}

	data, err := os.ReadFile(handoffPath)
	if err != nil {
		t.Fatalf("failed to read handoff: %v", err)
	}
	content := string(data)
	for _, want := range []string{
		"# Recall Agent Handoff",
		"Goal: build auth flow",
		"Branch: main",
		"-  M README.md",
		"1 file changed, 1 insertion(+)",
		"Continue this Recall session: build auth flow",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("handoff content missing %q:\n%s", want, content)
		}
	}
}

func TestWriteHandoffRejectsMissingHandoffDir(t *testing.T) {
	recallDir := t.TempDir()

	if handoffPath, err := writeHandoff(recallDir, recallStatus{}); err == nil {
		t.Fatalf("writeHandoff() = %q, nil; want error", handoffPath)
	}
}

func TestDiffLineChangeCountParsesInsertionsAndDeletions(t *testing.T) {
	got := diffLineChangeCount("2 files changed, 120 insertions(+), 18 deletions(-)")
	if got != 138 {
		t.Fatalf("diffLineChangeCount() = %d, want 138", got)
	}
}

func TestBuildReviewReturnsLowRiskChecklist(t *testing.T) {
	status := recallStatus{
		Session:      recallSession{Goal: "build auth flow"},
		ChangedFiles: []string{" M README.md"},
		DiffStats:    "1 file changed, 5 insertions(+)",
	}

	review := buildReview(status)
	if review.RiskLevel != "low" {
		t.Fatalf("risk level = %q, want low", review.RiskLevel)
	}
	if len(review.Checklist) == 0 {
		t.Fatalf("checklist is empty")
	}
}

func TestBuildReviewReturnsHighRiskChecklist(t *testing.T) {
	status := recallStatus{
		Session:      recallSession{Goal: "build auth flow"},
		ChangedFiles: []string{" M README.md"},
		DiffStats:    "4 files changed, 500 insertions(+), 1 deletion(-)",
	}

	review := buildReview(status)
	if review.RiskLevel != "high" {
		t.Fatalf("risk level = %q, want high", review.RiskLevel)
	}
	if !strings.Contains(review.Checklist[0], "context-loss risk is high") {
		t.Fatalf("first checklist item = %q, want high-risk warning", review.Checklist[0])
	}
}

func TestBuildReviewFlagsBranchChange(t *testing.T) {
	status := recallStatus{
		Session:         recallSession{Goal: "build auth flow", Branch: "main"},
		CurrentGitState: gitState{Branch: "feature/auth"},
		BranchChanged:   true,
	}

	review := buildReview(status)
	if review.RiskLevel != "medium" {
		t.Fatalf("risk level = %q, want medium", review.RiskLevel)
	}
	if !strings.Contains(review.Checklist[0], "branch change from main to feature/auth") {
		t.Fatalf("first checklist item = %q, want branch-change warning", review.Checklist[0])
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

	restoreWorkingDir := chdir(t, subdir)
	defer restoreWorkingDir()

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

	gitignoreData, err := os.ReadFile(filepath.Join(repoRoot, ".gitignore"))
	if err != nil {
		t.Fatalf("failed to read .gitignore: %v", err)
	}
	if !hasRecallIgnoreEntry(gitignoreData) {
		t.Fatalf(".gitignore missing .recall entry: %q", string(gitignoreData))
	}
}

func TestRunStartCreatesActiveSession(t *testing.T) {
	repoRoot := initTestRepo(t)
	baseCommit := commitTestFile(t, repoRoot)
	restoreWorkingDir := chdir(t, repoRoot)
	defer restoreWorkingDir()

	if _, _, err := runInit(); err != nil {
		t.Fatalf("runInit returned error: %v", err)
	}

	session, activePath, err := runStart("build auth flow")
	if err != nil {
		t.Fatalf("runStart returned error: %v", err)
	}

	if session.Goal != "build auth flow" {
		t.Fatalf("goal = %q, want %q", session.Goal, "build auth flow")
	}
	if session.BaseCommit != baseCommit {
		t.Fatalf("baseCommit = %q, want %q", session.BaseCommit, baseCommit)
	}

	wantActivePath := filepath.Join(repoRoot, ".recall", "sessions", activeSessionFileName)
	if activePath != wantActivePath {
		t.Fatalf("activePath = %q, want %q", activePath, wantActivePath)
	}
	if _, err := os.Stat(wantActivePath); err != nil {
		t.Fatalf("expected active session to exist: %v", err)
	}
}

func TestRunStartRejectsUninitializedRepo(t *testing.T) {
	repoRoot := initTestRepo(t)
	commitTestFile(t, repoRoot)
	restoreWorkingDir := chdir(t, repoRoot)
	defer restoreWorkingDir()

	if session, activePath, err := runStart("build auth flow"); err == nil {
		t.Fatalf("runStart() = %+v, %q, nil; want error", session, activePath)
	}
}

func TestRunStartRejectsExistingActiveSession(t *testing.T) {
	repoRoot := initTestRepo(t)
	commitTestFile(t, repoRoot)
	restoreWorkingDir := chdir(t, repoRoot)
	defer restoreWorkingDir()

	if _, _, err := runInit(); err != nil {
		t.Fatalf("runInit returned error: %v", err)
	}
	if _, _, err := runStart("build auth flow"); err != nil {
		t.Fatalf("first runStart returned error: %v", err)
	}
	if session, activePath, err := runStart("another goal"); err == nil {
		t.Fatalf("second runStart() = %+v, %q, nil; want error", session, activePath)
	}
}

func TestRunStatusReturnsActiveSession(t *testing.T) {
	repoRoot := initTestRepo(t)
	commitTestFile(t, repoRoot)
	restoreWorkingDir := chdir(t, repoRoot)
	defer restoreWorkingDir()

	if _, _, err := runInit(); err != nil {
		t.Fatalf("runInit returned error: %v", err)
	}
	want, _, err := runStart("build auth flow")
	if err != nil {
		t.Fatalf("runStart returned error: %v", err)
	}

	got, err := runStatus()
	if err != nil {
		t.Fatalf("runStatus returned error: %v", err)
	}
	if got.Session != want {
		t.Fatalf("session = %+v, want %+v", got.Session, want)
	}
}

func TestRunStatusIncludesChangedFilesAndDiffStats(t *testing.T) {
	repoRoot := initTestRepo(t)
	commitTestFile(t, repoRoot)
	restoreWorkingDir := chdir(t, repoRoot)
	defer restoreWorkingDir()

	if _, _, err := runInit(); err != nil {
		t.Fatalf("runInit returned error: %v", err)
	}
	if _, _, err := runStart("build auth flow"); err != nil {
		t.Fatalf("runStart returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("# Changed\n"), 0o644); err != nil {
		t.Fatalf("failed to modify tracked file: %v", err)
	}

	got, err := runStatus()
	if err != nil {
		t.Fatalf("runStatus returned error: %v", err)
	}
	if !containsLine(got.ChangedFiles, " M README.md") {
		t.Fatalf("changed files = %v, want modified README.md", got.ChangedFiles)
	}
	if !strings.Contains(got.DiffStats, "1 file changed") {
		t.Fatalf("diff stats = %q, want file change count", got.DiffStats)
	}
}

func TestRunStatusDetectsBranchChange(t *testing.T) {
	repoRoot := initTestRepo(t)
	commitTestFile(t, repoRoot)
	restoreWorkingDir := chdir(t, repoRoot)
	defer restoreWorkingDir()

	if _, _, err := runInit(); err != nil {
		t.Fatalf("runInit returned error: %v", err)
	}
	activeSession, _, err := runStart("build auth flow")
	if err != nil {
		t.Fatalf("runStart returned error: %v", err)
	}
	runGit(t, repoRoot, "checkout", "-b", "feature/auth")

	got, err := runStatus()
	if err != nil {
		t.Fatalf("runStatus returned error: %v", err)
	}
	if !got.BranchChanged {
		t.Fatalf("BranchChanged = false, want true")
	}
	if got.Session.Branch != activeSession.Branch {
		t.Fatalf("session branch = %q, want %q", got.Session.Branch, activeSession.Branch)
	}
	if got.CurrentGitState.Branch != "feature/auth" {
		t.Fatalf("current branch = %q, want feature/auth", got.CurrentGitState.Branch)
	}
}

func TestRunStatusReturnsNoActiveSession(t *testing.T) {
	repoRoot := initTestRepo(t)
	commitTestFile(t, repoRoot)
	restoreWorkingDir := chdir(t, repoRoot)
	defer restoreWorkingDir()

	if _, _, err := runInit(); err != nil {
		t.Fatalf("runInit returned error: %v", err)
	}

	if session, err := runStatus(); !errors.Is(err, errNoActiveSession) {
		t.Fatalf("runStatus() = %+v, %v; want errNoActiveSession", session, err)
	}
}

func TestRunCheckpointCreatesCheckpoint(t *testing.T) {
	repoRoot := initTestRepo(t)
	commitTestFile(t, repoRoot)
	restoreWorkingDir := chdir(t, repoRoot)
	defer restoreWorkingDir()

	if _, _, err := runInit(); err != nil {
		t.Fatalf("runInit returned error: %v", err)
	}
	if _, _, err := runStart("build auth flow"); err != nil {
		t.Fatalf("runStart returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("# Changed\n"), 0o644); err != nil {
		t.Fatalf("failed to modify tracked file: %v", err)
	}

	checkpointPath, err := runCheckpoint("OAuth flow implemented")
	if err != nil {
		t.Fatalf("runCheckpoint returned error: %v", err)
	}
	if filepath.Dir(checkpointPath) != filepath.Join(repoRoot, ".recall", "checkpoints") {
		t.Fatalf("checkpoint path = %q, want file in checkpoints directory", checkpointPath)
	}

	data, err := os.ReadFile(checkpointPath)
	if err != nil {
		t.Fatalf("failed to read checkpoint: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "Message: OAuth flow implemented") {
		t.Fatalf("checkpoint content missing message:\n%s", content)
	}
	if !strings.Contains(content, "-  M README.md") {
		t.Fatalf("checkpoint content missing changed file:\n%s", content)
	}
}

func TestRunCheckpointReturnsNoActiveSession(t *testing.T) {
	repoRoot := initTestRepo(t)
	commitTestFile(t, repoRoot)
	restoreWorkingDir := chdir(t, repoRoot)
	defer restoreWorkingDir()

	if _, _, err := runInit(); err != nil {
		t.Fatalf("runInit returned error: %v", err)
	}

	if checkpointPath, err := runCheckpoint("message"); !errors.Is(err, errNoActiveSession) {
		t.Fatalf("runCheckpoint() = %q, %v; want errNoActiveSession", checkpointPath, err)
	}
}

func TestRunHandoffCreatesHandoff(t *testing.T) {
	repoRoot := initTestRepo(t)
	commitTestFile(t, repoRoot)
	restoreWorkingDir := chdir(t, repoRoot)
	defer restoreWorkingDir()

	if _, _, err := runInit(); err != nil {
		t.Fatalf("runInit returned error: %v", err)
	}
	if _, _, err := runStart("build auth flow"); err != nil {
		t.Fatalf("runStart returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("# Changed\n"), 0o644); err != nil {
		t.Fatalf("failed to modify tracked file: %v", err)
	}

	handoffPath, err := runHandoff()
	if err != nil {
		t.Fatalf("runHandoff returned error: %v", err)
	}
	if filepath.Dir(handoffPath) != filepath.Join(repoRoot, ".recall", "handoffs") {
		t.Fatalf("handoff path = %q, want file in handoffs directory", handoffPath)
	}

	data, err := os.ReadFile(handoffPath)
	if err != nil {
		t.Fatalf("failed to read handoff: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "Continue this Recall session: build auth flow") {
		t.Fatalf("handoff content missing next-agent prompt:\n%s", content)
	}
	if !strings.Contains(content, "-  M README.md") {
		t.Fatalf("handoff content missing changed file:\n%s", content)
	}
}

func TestRunHandoffReturnsNoActiveSession(t *testing.T) {
	repoRoot := initTestRepo(t)
	commitTestFile(t, repoRoot)
	restoreWorkingDir := chdir(t, repoRoot)
	defer restoreWorkingDir()

	if _, _, err := runInit(); err != nil {
		t.Fatalf("runInit returned error: %v", err)
	}

	if handoffPath, err := runHandoff(); !errors.Is(err, errNoActiveSession) {
		t.Fatalf("runHandoff() = %q, %v; want errNoActiveSession", handoffPath, err)
	}
}

func TestRunStopArchivesActiveSession(t *testing.T) {
	repoRoot := initTestRepo(t)
	commitTestFile(t, repoRoot)
	restoreWorkingDir := chdir(t, repoRoot)
	defer restoreWorkingDir()

	if _, _, err := runInit(); err != nil {
		t.Fatalf("runInit returned error: %v", err)
	}
	activeSession, _, err := runStart("build auth flow")
	if err != nil {
		t.Fatalf("runStart returned error: %v", err)
	}

	stoppedSession, archivePath, err := runStop()
	if err != nil {
		t.Fatalf("runStop returned error: %v", err)
	}
	if stoppedSession.ID != activeSession.ID {
		t.Fatalf("stopped session ID = %q, want %q", stoppedSession.ID, activeSession.ID)
	}
	if stoppedSession.Status != sessionStatusStopped {
		t.Fatalf("status = %q, want %q", stoppedSession.Status, sessionStatusStopped)
	}
	if archivePath != filepath.Join(repoRoot, ".recall", "sessions", activeSession.ID+".json") {
		t.Fatalf("archivePath = %q, want stopped session archive", archivePath)
	}

	if _, err := runStatus(); !errors.Is(err, errNoActiveSession) {
		t.Fatalf("runStatus after stop returned %v; want errNoActiveSession", err)
	}
}

func TestRunStopReturnsNoActiveSession(t *testing.T) {
	repoRoot := initTestRepo(t)
	commitTestFile(t, repoRoot)
	restoreWorkingDir := chdir(t, repoRoot)
	defer restoreWorkingDir()

	if _, _, err := runInit(); err != nil {
		t.Fatalf("runInit returned error: %v", err)
	}

	if session, archivePath, err := runStop(); !errors.Is(err, errNoActiveSession) {
		t.Fatalf("runStop() = %+v, %q, %v; want errNoActiveSession", session, archivePath, err)
	}
}

func TestRunReviewReturnsChecklist(t *testing.T) {
	repoRoot := initTestRepo(t)
	commitTestFile(t, repoRoot)
	restoreWorkingDir := chdir(t, repoRoot)
	defer restoreWorkingDir()

	if _, _, err := runInit(); err != nil {
		t.Fatalf("runInit returned error: %v", err)
	}
	if _, _, err := runStart("build auth flow"); err != nil {
		t.Fatalf("runStart returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("# Changed\n"), 0o644); err != nil {
		t.Fatalf("failed to modify tracked file: %v", err)
	}

	review, err := runReview()
	if err != nil {
		t.Fatalf("runReview returned error: %v", err)
	}
	if review.Status.Session.Goal != "build auth flow" {
		t.Fatalf("goal = %q, want build auth flow", review.Status.Session.Goal)
	}
	if len(review.Checklist) == 0 {
		t.Fatalf("checklist is empty")
	}
}

func TestRunReviewReturnsNoActiveSession(t *testing.T) {
	repoRoot := initTestRepo(t)
	commitTestFile(t, repoRoot)
	restoreWorkingDir := chdir(t, repoRoot)
	defer restoreWorkingDir()

	if _, _, err := runInit(); err != nil {
		t.Fatalf("runInit returned error: %v", err)
	}

	if review, err := runReview(); !errors.Is(err, errNoActiveSession) {
		t.Fatalf("runReview() = %+v, %v; want errNoActiveSession", review, err)
	}
}

func recallSubdirsPaths(recallDir string) []string {
	paths := make([]string, 0, len(recallSubdirs))
	for _, subdir := range recallSubdirs {
		paths = append(paths, filepath.Join(recallDir, subdir))
	}
	return paths
}

func chdir(t *testing.T, dir string) func() {
	t.Helper()

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	return func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Fatalf("failed to restore current directory: %v", err)
		}
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

func containsLine(lines []string, want string) bool {
	for _, line := range lines {
		if line == want {
			return true
		}
	}
	return false
}
