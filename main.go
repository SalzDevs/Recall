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

	fmt.Printf("Git root: %s\n", gitRoot)
}
