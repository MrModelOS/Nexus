package internal

import (
	"fmt"
	"os/exec"
	"strings"
)

type GitInfo struct {
	Branch     string
	HasChanges bool
	Staged     []string
	Modified   []string
	Untracked  []string
}

func GetGitInfo() *GitInfo {
	info := &GitInfo{}

	if branch, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output(); err == nil {
		info.Branch = strings.TrimSpace(string(branch))
	}

	if output, err := exec.Command("git", "status", "--porcelain").Output(); err == nil {
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, line := range lines {
			if len(line) < 3 {
				continue
			}
			status := line[:2]
			file := strings.TrimSpace(line[3:])

			switch {
			case status[0] != ' ' && status[0] != '?':
				info.Staged = append(info.Staged, file)
			case status[1] != ' ' && status[1] != '?':
				info.Modified = append(info.Modified, file)
			case status == "??":
				info.Untracked = append(info.Untracked, file)
			}
		}
		info.HasChanges = len(info.Staged) > 0 || len(info.Modified) > 0 || len(info.Untracked) > 0
	}

	return info
}

func GetGitDiff(staged bool) (string, error) {
	args := []string{"diff"}
	if staged {
		args = append(args, "--cached")
	}

	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git diff failed: %w", err)
	}

	return string(output), nil
}

func GetGitDiffStats(staged bool) string {
	args := []string{"diff", "--stat"}
	if staged {
		args = append(args, "--cached")
	}

	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(output))
}

func GitAdd(files ...string) error {
	args := append([]string{"add"}, files...)
	cmd := exec.Command("git", args...)
	return cmd.Run()
}

func GitCommit(message string) error {
	cmd := exec.Command("git", "commit", "-m", message)
	return cmd.Run()
}

func GitCreateBranch(name string) error {
	cmd := exec.Command("git", "checkout", "-b", name)
	return cmd.Run()
}

func RenderGitStatus(info *GitInfo) string {
	if info == nil || !info.HasChanges {
		return ""
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("\033[1;35mGit:\033[0m %s", info.Branch))

	if len(info.Staged) > 0 {
		lines = append(lines, fmt.Sprintf("  \033[1;32mStaged:\033[0m %d files", len(info.Staged)))
	}

	if len(info.Modified) > 0 {
		lines = append(lines, fmt.Sprintf("  \033[1;33mModified:\033[0m %d files", len(info.Modified)))
	}

	if len(info.Untracked) > 0 {
		lines = append(lines, fmt.Sprintf("  \033[1;31mUntracked:\033[0m %d files", len(info.Untracked)))
	}

	return strings.Join(lines, "\n")
}
