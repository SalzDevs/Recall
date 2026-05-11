package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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

func main() {
	currentDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get current directory: %v\n", err)
		os.Exit(1)
	}

	gitRoot, err := findGitRoot(currentDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Recall requires a Git repository. Run `git init` first.\n")
		os.Exit(1)
	}

	recallDir, err := ensureRecallDir(gitRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize Recall: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Initialized Recall in %s\n", recallDir)
}
