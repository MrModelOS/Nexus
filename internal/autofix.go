package internal

import (
	"fmt"
	"os/exec"
	"strings"
)

type TestRunner struct {
	Commands []RunCommand
}

type RunCommand struct {
	Name    string
	Command string
	Args    []string
}

type RunResult struct {
	Command string
	Success bool
	Output  string
	Error   error
}

func DetectTestCommands() []RunCommand {
	var cmds []RunCommand

	if _, err := exec.LookPath("go"); err == nil {
		if fileExists("go.mod") {
			cmds = append(cmds, RunCommand{
				Name:    "go test",
				Command: "go",
				Args:    []string{"test", "./..."},
			})
		}
	}

	if _, err := exec.LookPath("npm"); err == nil {
		if fileExists("package.json") {
			data, _ := readFileString("package.json")
			if strings.Contains(data, "\"test\"") {
				cmds = append(cmds, RunCommand{
					Name:    "npm test",
					Command: "npm",
					Args:    []string{"test"},
				})
			}
		}
	}

	if _, err := exec.LookPath("pytest"); err == nil {
		if fileExists("pytest.ini") || fileExists("pyproject.toml") || fileExists("setup.cfg") {
			cmds = append(cmds, RunCommand{
				Name:    "pytest",
				Command: "pytest",
				Args:    []string{},
			})
		}
	}

	linters := []struct {
		bin  string
		name string
		args []string
	}{
		{"golangci-lint", "golangci-lint", []string{"run"}},
		{"eslint", "eslint", []string{"."}},
		{"ruff", "ruff", []string{"check", "."}},
	}

	for _, l := range linters {
		if _, err := exec.LookPath(l.bin); err == nil {
			cmds = append(cmds, RunCommand{
				Name:    l.name,
				Command: l.bin,
				Args:    l.args,
			})
		}
	}

	return cmds
}

func ExecuteCommand(cmd RunCommand) RunResult {
	result := RunResult{
		Command: cmd.Name,
		Success: true,
	}

	execCmd := exec.Command(cmd.Command, cmd.Args...)
	output, err := execCmd.CombinedOutput()

	result.Output = string(output)

	if err != nil {
		result.Success = false
		result.Error = err

		if exitErr, ok := err.(*exec.ExitError); ok {
			result.Output = string(output) + fmt.Sprintf("\nExit code: %d", exitErr.ExitCode())
		}
	}

	return result
}

func RunAllChecks() []RunResult {
	cmds := DetectTestCommands()
	if len(cmds) == 0 {
		return nil
	}

	var results []RunResult
	for _, cmd := range cmds {
		results = append(results, ExecuteCommand(cmd))
	}

	return results
}

func FormatAutoFixResults(results []RunResult) string {
	if len(results) == 0 {
		return "No test/lint commands detected."
	}

	var out strings.Builder
	out.WriteString("\033[1;35mAuto-check results:\033[0m\n\n")

	allPassed := true
	for _, r := range results {
		if r.Success {
			out.WriteString(fmt.Sprintf("  \033[1;32m✓\033[0m %s\n", r.Command))
		} else {
			allPassed = false
			out.WriteString(fmt.Sprintf("  \033[1;31m✗\033[0m %s\n", r.Command))
			if r.Output != "" {
				lines := strings.Split(r.Output, "\n")
				maxLines := 15
				if len(lines) > maxLines {
					lines = lines[:maxLines]
					lines = append(lines, fmt.Sprintf("  ... (%d more lines)", len(lines)-maxLines))
				}
				for _, line := range lines {
					if strings.TrimSpace(line) != "" {
						out.WriteString(fmt.Sprintf("    %s\n", line))
					}
				}
			}
		}
	}

	if allPassed {
		out.WriteString("\n\033[1;32mAll checks passed!\033[0m")
	} else {
		out.WriteString("\n\033[1;33mSome checks failed. Model will attempt to fix.\033[0m")
	}

	return out.String()
}

func GetFailedOutput(results []RunResult) string {
	var failed []string
	for _, r := range results {
		if !r.Success {
			failed = append(failed, fmt.Sprintf("[%s]\n%s", r.Command, r.Output))
		}
	}
	return strings.Join(failed, "\n\n")
}

func fileExists(path string) bool {
	_, err := exec.Command("test", "-f", path).Output()
	return err == nil
}

func readFileString(path string) (string, error) {
	data, err := exec.Command("cat", path).Output()
	return string(data), err
}
