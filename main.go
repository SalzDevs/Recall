package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	configFileName        = "config.json"
	activeSessionFileName = "active.json"
	sessionStatusActive   = "active"
	checkpointTimeFormat  = "20060102T150405.000000000"
)

var (
	errNoActiveSession = errors.New("no active Recall session")
	recallSubdirs      = []string{"sessions", "checkpoints", "handoffs"}
)

type recallConfig struct {
	SchemaVersion int    `json:"schemaVersion"`
	ProjectName   string `json:"projectName"`
	CreatedAt     string `json:"createdAt"`
}

type gitState struct {
	Branch string
	Commit string
}

type recallSession struct {
	ID         string `json:"id"`
	Goal       string `json:"goal"`
	StartedAt  string `json:"startedAt"`
	Branch     string `json:"branch"`
	BaseCommit string `json:"baseCommit"`
	Status     string `json:"status"`
}

type recallStatus struct {
	Session      recallSession
	ChangedFiles []string
	DiffStats    string
}

func findGitRoot(startDir string) (string, error) {
	if startDir == "" {
		return "", fmt.Errorf("start directory is required")
	}

	cmd := exec.Command("git", "-C", startDir, "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not inside a git repository")
	}

	gitRoot := strings.TrimSpace(string(output))
	if gitRoot == "" {
		return "", fmt.Errorf("git root not found")
	}

	return filepath.Clean(gitRoot), nil
}

func getGitState(gitRoot string) (gitState, error) {
	if gitRoot == "" {
		return gitState{}, fmt.Errorf("git root is required")
	}

	branchOutput, err := exec.Command("git", "-C", gitRoot, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return gitState{}, fmt.Errorf("failed to read current git branch")
	}

	commitOutput, err := exec.Command("git", "-C", gitRoot, "rev-parse", "HEAD").Output()
	if err != nil {
		return gitState{}, fmt.Errorf("failed to read current git commit")
	}

	state := gitState{
		Branch: strings.TrimSpace(string(branchOutput)),
		Commit: strings.TrimSpace(string(commitOutput)),
	}
	if state.Branch == "" {
		return gitState{}, fmt.Errorf("current git branch not found")
	}
	if state.Commit == "" {
		return gitState{}, fmt.Errorf("current git commit not found")
	}

	return state, nil
}

func getChangedFiles(gitRoot string) ([]string, error) {
	if gitRoot == "" {
		return nil, fmt.Errorf("git root is required")
	}

	output, err := exec.Command("git", "-C", gitRoot, "status", "--short").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to read changed files")
	}

	text := strings.TrimRight(string(output), "\r\n")
	if text == "" {
		return []string{}, nil
	}

	lines := strings.Split(text, "\n")
	changedFiles := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		changedFiles = append(changedFiles, line)
	}

	return changedFiles, nil
}

func getDiffStats(gitRoot, baseCommit string) (string, error) {
	if gitRoot == "" {
		return "", fmt.Errorf("git root is required")
	}
	if baseCommit == "" {
		return "", fmt.Errorf("base commit is required")
	}

	output, err := exec.Command("git", "-C", gitRoot, "diff", "--shortstat", baseCommit).Output()
	if err != nil {
		return "", fmt.Errorf("failed to read diff stats")
	}

	return strings.TrimSpace(string(output)), nil
}

func ensureRecallDir(gitRoot string) (string, error) {
	if gitRoot == "" {
		return "", fmt.Errorf("git root is required")
	}

	recallDir := filepath.Join(gitRoot, ".recall")
	info, err := os.Stat(recallDir)
	if err == nil {
		if !info.IsDir() {
			return "", fmt.Errorf(".recall exists but is not a directory")
		}

		return recallDir, nil
	}

	if !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to inspect .recall directory: %w", err)
	}

	if err := os.Mkdir(recallDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create .recall directory: %w", err)
	}

	return recallDir, nil
}

func getRecallDir(gitRoot string) (string, error) {
	if gitRoot == "" {
		return "", fmt.Errorf("git root is required")
	}

	recallDir := filepath.Join(gitRoot, ".recall")
	info, err := os.Stat(recallDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("Recall is not initialized. Run `recall init` first")
		}
		return "", fmt.Errorf("failed to inspect .recall directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf(".recall exists but is not a directory")
	}

	configPath := filepath.Join(recallDir, configFileName)
	info, err = os.Stat(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("Recall is not initialized. Run `recall init` first")
		}
		return "", fmt.Errorf("failed to inspect config.json: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("config.json exists but is a directory")
	}

	sessionsDir := filepath.Join(recallDir, "sessions")
	info, err = os.Stat(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("Recall is not initialized. Run `recall init` first")
		}
		return "", fmt.Errorf("failed to inspect sessions directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("sessions path exists but is not a directory")
	}

	return recallDir, nil
}

func ensureRecallSubdirs(recallDir string) error {
	if recallDir == "" {
		return fmt.Errorf("recall directory is required")
	}

	for _, subdir := range recallSubdirs {
		path := filepath.Join(recallDir, subdir)
		info, err := os.Stat(path)
		if err == nil {
			if !info.IsDir() {
				return fmt.Errorf("%s exists but is not a directory", path)
			}

			continue
		}

		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to inspect %s: %w", path, err)
		}

		if err := os.Mkdir(path, 0o755); err != nil {
			return fmt.Errorf("failed to create %s: %w", path, err)
		}
	}

	return nil
}

func newActiveSession(goal string, state gitState) (recallSession, error) {
	goal = strings.TrimSpace(goal)
	if goal == "" {
		return recallSession{}, fmt.Errorf("session goal is required")
	}
	if state.Branch == "" {
		return recallSession{}, fmt.Errorf("git branch is required")
	}
	if state.Commit == "" {
		return recallSession{}, fmt.Errorf("git commit is required")
	}

	now := time.Now().UTC()
	return recallSession{
		ID:         now.Format("20060102T150405Z"),
		Goal:       goal,
		StartedAt:  now.Format(time.RFC3339),
		Branch:     state.Branch,
		BaseCommit: state.Commit,
		Status:     sessionStatusActive,
	}, nil
}

func writeActiveSession(recallDir string, session recallSession) (string, error) {
	if recallDir == "" {
		return "", fmt.Errorf("recall directory is required")
	}

	activePath := filepath.Join(recallDir, "sessions", activeSessionFileName)
	info, err := os.Stat(activePath)
	if err == nil {
		if info.IsDir() {
			return "", fmt.Errorf("active session path exists but is a directory")
		}

		return "", fmt.Errorf("active session already exists")
	}

	if !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to inspect active session: %w", err)
	}

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to encode active session: %w", err)
	}
	data = append(data, '\n')

	file, err := os.OpenFile(activePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", fmt.Errorf("failed to create active session: %w", err)
	}
	defer file.Close()

	if _, err := file.Write(data); err != nil {
		return "", fmt.Errorf("failed to write active session: %w", err)
	}

	return activePath, nil
}

func readActiveSession(recallDir string) (recallSession, error) {
	if recallDir == "" {
		return recallSession{}, fmt.Errorf("recall directory is required")
	}

	activePath := filepath.Join(recallDir, "sessions", activeSessionFileName)
	info, err := os.Stat(activePath)
	if err != nil {
		if os.IsNotExist(err) {
			return recallSession{}, errNoActiveSession
		}
		return recallSession{}, fmt.Errorf("failed to inspect active session: %w", err)
	}
	if info.IsDir() {
		return recallSession{}, fmt.Errorf("active session path exists but is a directory")
	}

	data, err := os.ReadFile(activePath)
	if err != nil {
		return recallSession{}, fmt.Errorf("failed to read active session: %w", err)
	}

	var session recallSession
	if err := json.Unmarshal(data, &session); err != nil {
		return recallSession{}, fmt.Errorf("failed to decode active session: %w", err)
	}

	return session, nil
}

func ensureSubdir(recallDir, name string) (string, error) {
	if recallDir == "" {
		return "", fmt.Errorf("recall directory is required")
	}
	if name == "" {
		return "", fmt.Errorf("subdirectory name is required")
	}

	path := filepath.Join(recallDir, name)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%s directory not found", name)
		}
		return "", fmt.Errorf("failed to inspect %s directory: %w", name, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s path exists but is not a directory", name)
	}

	return path, nil
}

func writeCheckpoint(recallDir string, status recallStatus, message string) (string, error) {
	if recallDir == "" {
		return "", fmt.Errorf("recall directory is required")
	}

	message = strings.TrimSpace(message)
	if message == "" {
		return "", fmt.Errorf("checkpoint message is required")
	}

	checkpointsDir, err := ensureSubdir(recallDir, "checkpoints")
	if err != nil {
		return "", err
	}

	now := time.Now().UTC()
	createdAt := now.Format(time.RFC3339)
	fileName := now.Format(checkpointTimeFormat) + "Z.md"
	checkpointPath := filepath.Join(checkpointsDir, fileName)

	var builder strings.Builder
	builder.WriteString("# Recall Checkpoint\n\n")
	builder.WriteString(fmt.Sprintf("Message: %s\n", message))
	builder.WriteString(fmt.Sprintf("Created: %s\n\n", createdAt))
	builder.WriteString("## Session\n\n")
	builder.WriteString(fmt.Sprintf("Goal: %s\n", status.Session.Goal))
	builder.WriteString(fmt.Sprintf("Started: %s\n", status.Session.StartedAt))
	builder.WriteString(fmt.Sprintf("Branch: %s\n", status.Session.Branch))
	builder.WriteString(fmt.Sprintf("Base commit: %s\n\n", status.Session.BaseCommit))
	builder.WriteString("## Changed files\n\n")
	if len(status.ChangedFiles) == 0 {
		builder.WriteString("none\n\n")
	} else {
		for _, file := range status.ChangedFiles {
			builder.WriteString(fmt.Sprintf("- %s\n", file))
		}
		builder.WriteString("\n")
	}
	builder.WriteString("## Diff\n\n")
	if status.DiffStats == "" {
		builder.WriteString("no tracked changes\n")
	} else {
		builder.WriteString(status.DiffStats)
		builder.WriteString("\n")
	}

	file, err := os.OpenFile(checkpointPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", fmt.Errorf("failed to create checkpoint: %w", err)
	}
	defer file.Close()

	if _, err := file.WriteString(builder.String()); err != nil {
		return "", fmt.Errorf("failed to write checkpoint: %w", err)
	}

	return checkpointPath, nil
}

func writeHandoff(recallDir string, status recallStatus) (string, error) {
	if recallDir == "" {
		return "", fmt.Errorf("recall directory is required")
	}

	handoffsDir, err := ensureSubdir(recallDir, "handoffs")
	if err != nil {
		return "", err
	}

	now := time.Now().UTC()
	handoffPath := filepath.Join(handoffsDir, now.Format(checkpointTimeFormat)+"Z.md")

	var builder strings.Builder
	builder.WriteString("# Recall Agent Handoff\n\n")
	builder.WriteString(fmt.Sprintf("Generated: %s\n\n", now.Format(time.RFC3339)))
	builder.WriteString("## Current session\n\n")
	builder.WriteString(fmt.Sprintf("Goal: %s\n", status.Session.Goal))
	builder.WriteString(fmt.Sprintf("Started: %s\n", status.Session.StartedAt))
	builder.WriteString(fmt.Sprintf("Status: %s\n\n", status.Session.Status))
	builder.WriteString("## Git context\n\n")
	builder.WriteString(fmt.Sprintf("Branch: %s\n", status.Session.Branch))
	builder.WriteString(fmt.Sprintf("Base commit: %s\n\n", status.Session.BaseCommit))
	builder.WriteString("## Changed files\n\n")
	if len(status.ChangedFiles) == 0 {
		builder.WriteString("none\n\n")
	} else {
		for _, file := range status.ChangedFiles {
			builder.WriteString(fmt.Sprintf("- %s\n", file))
		}
		builder.WriteString("\n")
	}
	builder.WriteString("## Diff stats\n\n")
	if status.DiffStats == "" {
		builder.WriteString("no tracked changes\n\n")
	} else {
		builder.WriteString(status.DiffStats)
		builder.WriteString("\n\n")
	}
	builder.WriteString("## Suggested next-agent prompt\n\n")
	builder.WriteString(fmt.Sprintf("Continue this Recall session: %s\n\n", status.Session.Goal))
	builder.WriteString("Before changing code, review the changed files and diff stats above. Preserve the user's context and explain any risky changes.\n")

	file, err := os.OpenFile(handoffPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", fmt.Errorf("failed to create handoff: %w", err)
	}
	defer file.Close()

	if _, err := file.WriteString(builder.String()); err != nil {
		return "", fmt.Errorf("failed to write handoff: %w", err)
	}

	return handoffPath, nil
}

func writeDefaultConfig(recallDir, projectName string) (string, error) {
	if recallDir == "" {
		return "", fmt.Errorf("recall directory is required")
	}
	if projectName == "" {
		return "", fmt.Errorf("project name is required")
	}

	configPath := filepath.Join(recallDir, configFileName)
	info, err := os.Stat(configPath)
	if err == nil {
		if info.IsDir() {
			return "", fmt.Errorf("config.json exists but is a directory")
		}

		return configPath, nil
	}

	if !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to inspect config.json: %w", err)
	}

	config := recallConfig{
		SchemaVersion: 1,
		ProjectName:   projectName,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to encode config.json: %w", err)
	}
	data = append(data, '\n')

	file, err := os.OpenFile(configPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", fmt.Errorf("failed to create config.json: %w", err)
	}
	defer file.Close()

	if _, err := file.Write(data); err != nil {
		return "", fmt.Errorf("failed to write config.json: %w", err)
	}

	return configPath, nil
}

func runInit() (string, string, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return "", "", fmt.Errorf("failed to get current directory: %w", err)
	}

	gitRoot, err := findGitRoot(currentDir)
	if err != nil {
		return "", "", fmt.Errorf("Recall requires a Git repository. Run `git init` first")
	}

	recallDir, err := ensureRecallDir(gitRoot)
	if err != nil {
		return "", "", err
	}

	if err := ensureRecallSubdirs(recallDir); err != nil {
		return "", "", err
	}

	projectName := filepath.Base(gitRoot)
	configPath, err := writeDefaultConfig(recallDir, projectName)
	if err != nil {
		return "", "", err
	}

	return recallDir, configPath, nil
}

func runStart(goal string) (recallSession, string, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return recallSession{}, "", fmt.Errorf("failed to get current directory: %w", err)
	}

	gitRoot, err := findGitRoot(currentDir)
	if err != nil {
		return recallSession{}, "", fmt.Errorf("Recall requires a Git repository. Run `git init` first")
	}

	recallDir, err := getRecallDir(gitRoot)
	if err != nil {
		return recallSession{}, "", err
	}

	state, err := getGitState(gitRoot)
	if err != nil {
		return recallSession{}, "", err
	}

	session, err := newActiveSession(goal, state)
	if err != nil {
		return recallSession{}, "", err
	}

	activePath, err := writeActiveSession(recallDir, session)
	if err != nil {
		return recallSession{}, "", err
	}

	return session, activePath, nil
}

func runStatus() (recallStatus, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return recallStatus{}, fmt.Errorf("failed to get current directory: %w", err)
	}

	gitRoot, err := findGitRoot(currentDir)
	if err != nil {
		return recallStatus{}, fmt.Errorf("Recall requires a Git repository. Run `git init` first")
	}

	recallDir, err := getRecallDir(gitRoot)
	if err != nil {
		return recallStatus{}, err
	}

	session, err := readActiveSession(recallDir)
	if err != nil {
		return recallStatus{}, err
	}

	changedFiles, err := getChangedFiles(gitRoot)
	if err != nil {
		return recallStatus{}, err
	}

	diffStats, err := getDiffStats(gitRoot, session.BaseCommit)
	if err != nil {
		return recallStatus{}, err
	}

	return recallStatus{Session: session, ChangedFiles: changedFiles, DiffStats: diffStats}, nil
}

func currentRecallContext() (string, recallStatus, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return "", recallStatus{}, fmt.Errorf("failed to get current directory: %w", err)
	}

	gitRoot, err := findGitRoot(currentDir)
	if err != nil {
		return "", recallStatus{}, fmt.Errorf("Recall requires a Git repository. Run `git init` first")
	}

	recallDir, err := getRecallDir(gitRoot)
	if err != nil {
		return "", recallStatus{}, err
	}

	session, err := readActiveSession(recallDir)
	if err != nil {
		return "", recallStatus{}, err
	}

	changedFiles, err := getChangedFiles(gitRoot)
	if err != nil {
		return "", recallStatus{}, err
	}

	diffStats, err := getDiffStats(gitRoot, session.BaseCommit)
	if err != nil {
		return "", recallStatus{}, err
	}

	return recallDir, recallStatus{Session: session, ChangedFiles: changedFiles, DiffStats: diffStats}, nil
}

func runCheckpoint(message string) (string, error) {
	recallDir, status, err := currentRecallContext()
	if err != nil {
		return "", err
	}

	return writeCheckpoint(recallDir, status, message)
}

func runHandoff() (string, error) {
	recallDir, status, err := currentRecallContext()
	if err != nil {
		return "", err
	}

	return writeHandoff(recallDir, status)
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  recall init")
	fmt.Fprintln(w, "  recall start <goal>")
	fmt.Fprintln(w, "  recall status")
	fmt.Fprintln(w, "  recall checkpoint <message>")
	fmt.Fprintln(w, "  recall handoff")
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		printUsage(os.Stdout)
		return
	}

	switch args[0] {
	case "help", "-h", "--help":
		printUsage(os.Stdout)
	case "init":
		if len(args) > 1 {
			fmt.Fprintf(os.Stderr, "init does not accept arguments\n\n")
			printUsage(os.Stderr)
			os.Exit(1)
		}

		recallDir, configPath, err := runInit()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to initialize Recall: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Initialized Recall in %s\n", recallDir)
		fmt.Printf("Config: %s\n", configPath)
	case "start":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "start requires a goal\n\n")
			printUsage(os.Stderr)
			os.Exit(1)
		}

		goal := strings.Join(args[1:], " ")
		session, activePath, err := runStart(goal)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to start Recall session: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Started Recall session: %s\n", session.Goal)
		fmt.Printf("Branch: %s\n", session.Branch)
		fmt.Printf("Base commit: %s\n", session.BaseCommit)
		fmt.Printf("Session: %s\n", activePath)
	case "status":
		if len(args) > 1 {
			fmt.Fprintf(os.Stderr, "status does not accept arguments\n\n")
			printUsage(os.Stderr)
			os.Exit(1)
		}

		status, err := runStatus()
		if err != nil {
			if errors.Is(err, errNoActiveSession) {
				fmt.Println("No active Recall session.")
				fmt.Println()
				fmt.Println("Start one with:")
				fmt.Println("  recall start \"describe your goal\"")
				return
			}

			fmt.Fprintf(os.Stderr, "failed to read Recall status: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Active Recall session: %s\n", status.Session.Goal)
		fmt.Printf("Started: %s\n", status.Session.StartedAt)
		fmt.Printf("Branch: %s\n", status.Session.Branch)
		fmt.Printf("Base commit: %s\n", status.Session.BaseCommit)
		fmt.Println()
		fmt.Println("Changed files:")
		if len(status.ChangedFiles) == 0 {
			fmt.Println("  none")
		} else {
			for _, file := range status.ChangedFiles {
				fmt.Printf("  %s\n", file)
			}
		}
		fmt.Println()
		fmt.Println("Diff:")
		if status.DiffStats == "" {
			fmt.Println("  no tracked changes")
		} else {
			fmt.Printf("  %s\n", status.DiffStats)
		}
	case "checkpoint":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "checkpoint requires a message\n\n")
			printUsage(os.Stderr)
			os.Exit(1)
		}

		message := strings.Join(args[1:], " ")
		checkpointPath, err := runCheckpoint(message)
		if err != nil {
			if errors.Is(err, errNoActiveSession) {
				fmt.Fprintf(os.Stderr, "no active Recall session. Start one with `recall start \"describe your goal\"`\n")
				os.Exit(1)
			}

			fmt.Fprintf(os.Stderr, "failed to create checkpoint: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Created checkpoint: %s\n", checkpointPath)
	case "handoff":
		if len(args) > 1 {
			fmt.Fprintf(os.Stderr, "handoff does not accept arguments\n\n")
			printUsage(os.Stderr)
			os.Exit(1)
		}

		handoffPath, err := runHandoff()
		if err != nil {
			if errors.Is(err, errNoActiveSession) {
				fmt.Fprintf(os.Stderr, "no active Recall session. Start one with `recall start \"describe your goal\"`\n")
				os.Exit(1)
			}

			fmt.Fprintf(os.Stderr, "failed to create handoff: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Created handoff: %s\n", handoffPath)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", args[0])
		printUsage(os.Stderr)
		os.Exit(1)
	}
}
