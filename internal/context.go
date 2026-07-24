package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ContextManager struct {
	MaxTokens  int
	MaxFiles   int
	RootDir    string
	IgnorePatterns []string
	fileIndex  map[string]FileEntry
}

type FileEntry struct {
	Path      string
	Size      int64
	Extension string
	IsDir     bool
}

func NewContextManager(rootDir string) *ContextManager {
	if rootDir == "" {
		rootDir, _ = os.Getwd()
	}

	return &ContextManager{
		MaxTokens: 8000,
		MaxFiles:  50,
		RootDir:   rootDir,
		IgnorePatterns: []string{
			".git", "node_modules", "vendor", ".cache",
			"__pycache__", ".next", "dist", "build",
			".vscode", ".idea", "*.min.js", "*.min.css",
		},
		fileIndex: make(map[string]FileEntry),
	}
}

func (cm *ContextManager) BuildIndex() error {
	cm.fileIndex = make(map[string]FileEntry)

	return filepath.Walk(cm.RootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		relPath, _ := filepath.Rel(cm.RootDir, path)

		if cm.shouldIgnore(relPath) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)

		cm.fileIndex[relPath] = FileEntry{
			Path:      relPath,
			Size:      info.Size(),
			Extension: ext,
			IsDir:     info.IsDir(),
		}

		return nil
	})
}

func (cm *ContextManager) shouldIgnore(path string) bool {
	parts := strings.Split(path, string(os.PathSeparator))

	for _, part := range parts {
		for _, pattern := range cm.IgnorePatterns {
			if matched, _ := filepath.Match(pattern, part); matched {
				return true
			}
			if strings.Contains(part, pattern) {
				return true
			}
		}
	}

	return false
}

func (cm *ContextManager) SearchCodebase(query string) []SearchResult {
	var results []SearchResult

	query = strings.ToLower(query)

	for path, entry := range cm.fileIndex {
		if entry.IsDir {
			continue
		}

		if strings.Contains(strings.ToLower(path), query) {
			results = append(results, SearchResult{
				File:    path,
				Content: fmt.Sprintf("[%s]", entry.Extension),
			})

			if len(results) >= 20 {
				break
			}
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return len(results[i].File) < len(results[j].File)
	})

	return results
}

func (cm *ContextManager) ReadFileForContext(path string, maxLines int) (string, int, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", 0, err
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", 0, err
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	totalLines := len(lines)

	if maxLines <= 0 {
		maxLines = 100
	}

	if totalLines > maxLines {
		half := maxLines / 2
	头部 := lines[:half]
		尾部 := lines[totalLines-half:]

		var builder strings.Builder
		for _, line := range 头部 {
			builder.WriteString(line)
			builder.WriteString("\n")
		}
		builder.WriteString(fmt.Sprintf("\n... (%d lines omitted) ...\n\n", totalLines-maxLines))
		for _, line := range 尾部 {
			builder.WriteString(line)
			builder.WriteString("\n")
		}

		return builder.String(), totalLines, nil
	}

	return content, totalLines, nil
}

func (cm *ContextManager) GetProjectStructure() string {
	var dirs []string
	var files []string

	for path, entry := range cm.fileIndex {
		if path == "." {
			continue
		}

		parts := strings.Split(path, string(os.PathSeparator))
		if len(parts) > 3 {
			path = strings.Join(parts[:3], string(os.PathSeparator)) + "/..."
		}

		if entry.IsDir {
			dirs = append(dirs, path)
		} else {
			files = append(files, path)
		}
	}

	sort.Strings(dirs)
	sort.Strings(files)

	var out strings.Builder
	out.WriteString("## Project Structure\n\n")

	if len(dirs) > 0 {
		out.WriteString("### Directories\n")
		for _, d := range dirs[:min(20, len(dirs))] {
			out.WriteString(fmt.Sprintf("- %s/\n", d))
		}
		out.WriteString("\n")
	}

	if len(files) > 0 {
		out.WriteString("### Key Files\n")
		for _, f := range files[:min(30, len(files))] {
			out.WriteString(fmt.Sprintf("- %s\n", f))
		}
	}

	return out.String()
}

func (cm *ContextManager) GetImportantFiles() []string {
	var important []string

	priorities := map[string]int{
		".go":       1,
		".rs":       1,
		".py":       1,
		".js":       2,
		".ts":       2,
		".tsx":      2,
		".jsx":      2,
		".yaml":     3,
		".yml":      3,
		".json":     3,
		".toml":     3,
		"Makefile":  1,
		"Dockerfile": 2,
		"README.md": 3,
	}

	for path, entry := range cm.fileIndex {
		if entry.IsDir {
			continue
		}

		name := filepath.Base(path)
		ext := entry.Extension

		if prio, ok := priorities[ext]; ok {
			if prio <= 2 {
				important = append(important, path)
			}
		}

		if prio, ok := priorities[name]; ok {
			if prio <= 2 {
				important = append(important, path)
			}
		}
	}

	sort.Strings(important)

	if len(important) > 20 {
		important = important[:20]
	}

	return important
}

func (cm *ContextManager) EstimateTokens(text string) int {
	return len(text) / 4
}

func (cm *ContextManager) FitToContext(texts []string, maxTokens int) []string {
	var result []string
	used := 0

	for _, text := range texts {
		tokens := cm.EstimateTokens(text)
		if used+tokens > maxTokens {
			break
		}
		result = append(result, text)
		used += tokens
	}

	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
