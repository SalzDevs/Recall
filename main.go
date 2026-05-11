package main

import (
	"encoding/json"
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
)

var recallSubdirs = []string{"sessions", "checkpoints", "handoffs"}

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

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  recall init")
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
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", args[0])
		printUsage(os.Stderr)
		os.Exit(1)
	}
}
