package internal

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type SearchResult struct {
	File    string
	Line    int
	Content string
}

type FileRef struct {
	Path    string
	Content string
}

func SearchFiles(query string, maxResults int) ([]SearchResult, error) {
	if maxResults <= 0 {
		maxResults = 20
	}

	rgPath, err := exec.LookPath("rg")
	if err != nil {
		return searchFallback(query, maxResults)
	}

	args := []string{
		"--no-heading",
		"--line-number",
		"--color=never",
		"-i",
		"--max-count=3",
		fmt.Sprintf("--max-columns=%d", 200),
		query,
	}

	cmd := exec.Command(rgPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("ripgrep failed: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var results []SearchResult

	for _, line := range lines {
		if len(results) >= maxResults {
			break
		}

		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 3 {
			continue
		}

		linenum := 0
		fmt.Sscanf(parts[1], "%d", &linenum)

		results = append(results, SearchResult{
			File:    parts[0],
			Line:    linenum,
			Content: strings.TrimSpace(parts[2]),
		})
	}

	return results, nil
}

func searchFallback(query string, maxResults int) ([]SearchResult, error) {
	var results []SearchResult

	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		if strings.Contains(path, ".git") || strings.Contains(path, "node_modules") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		content := string(data)
		queryLower := strings.ToLower(query)

		if !strings.Contains(strings.ToLower(content), queryLower) {
			return nil
		}

		lines := strings.Split(content, "\n")
		for i, line := range lines {
			if len(results) >= maxResults {
				return filepath.SkipDir
			}
			if strings.Contains(strings.ToLower(line), queryLower) {
				results = append(results, SearchResult{
					File:    path,
					Line:    i + 1,
					Content: strings.TrimSpace(line),
				})
			}
		}

		return nil
	})

	return results, err
}

func ResolveFileRefs(text string) string {
	re := regexp.MustCompile(`@(\S+)`)

	return re.ReplaceAllStringFunc(text, func(match string) string {
		path := match[1:]
		if path == "" {
			return match
		}

		if _, err := os.Stat(path); os.IsNotExist(err) {
			return match
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return match
		}

		content := string(data)
		if len(content) > 4000 {
			content = content[:4000] + "\n\n... (truncated at 4000 chars)"
		}

		return fmt.Sprintf("@%s\n```\n%s\n```", path, content)
	})
}

func FormatSearchResults(results []SearchResult) string {
	if len(results) == 0 {
		return "No results found."
	}

	fileResults := make(map[string][]SearchResult)
	for _, r := range results {
		fileResults[r.File] = append(fileResults[r.File], r)
	}

	var files []string
	for f := range fileResults {
		files = append(files, f)
	}
	sort.Strings(files)

	var out strings.Builder
	out.WriteString(fmt.Sprintf("\033[1;35mSearch results:\033[0m %d matches in %d files\n\n", len(results), len(files)))

	for _, file := range files {
		fileMatches := fileResults[file]
		out.WriteString(fmt.Sprintf("\033[1;36m%s\033[0m\n", file))
		for _, m := range fileMatches {
			out.WriteString(fmt.Sprintf("  \033[1;33m%d\033[0m: %s\n", m.Line, m.Content))
		}
		out.WriteString("\n")
	}

	return strings.TrimSpace(out.String())
}
