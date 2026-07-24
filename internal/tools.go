package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type Tool interface {
	Name() string
	Description() string
	Schema() map[string]interface{}
	Execute(args map[string]interface{}) (string, error)
}

type ToolRegistry struct {
	tools   map[string]Tool
	results chan ToolResult
}

type ToolResult struct {
	ToolName string
	Output   string
	Error    error
}

func NewToolRegistry() *ToolRegistry {
	r := &ToolRegistry{
		tools:   make(map[string]Tool),
		results: make(chan ToolResult, 10),
	}

	r.Register(&ReadFileTool{})
	r.Register(&WriteFileTool{})
	r.Register(&EditFileTool{})
	r.Register(&DeleteFileTool{})
	r.Register(&ListDirTool{})
	r.Register(&RunCommandTool{})
	r.Register(&GrepTool{})
	r.Register(&GlobTool{})

	return r
}

func (r *ToolRegistry) Register(tool Tool) {
	r.tools[tool.Name()] = tool
}

func (r *ToolRegistry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

func (r *ToolRegistry) List() []ToolInfo {
	var list []ToolInfo
	for _, t := range r.tools {
		list = append(list, ToolInfo{
			Name:        t.Name(),
			Description: t.Description(),
			Schema:      t.Schema(),
		})
	}
	return list
}

type ToolInfo struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Schema      map[string]interface{} `json:"input_schema"`
}

func (r *ToolRegistry) GetSystemPrompt() string {
	tools := r.List()
	if len(tools) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("You have access to the following tools. Use them by responding with a JSON object in this format:\n")
	sb.WriteString("```json\n{\"tool\": \"tool_name\", \"args\": {\"param\": \"value\"}}\n```\n\n")
	sb.WriteString("Available tools:\n\n")

	for _, t := range tools {
		sb.WriteString(fmt.Sprintf("### %s\n%s\n\n", t.Name, t.Description))
		if schemaJSON, err := json.MarshalIndent(t.Schema, "", "  "); err == nil {
			sb.WriteString(fmt.Sprintf("Parameters:\n```json\n%s\n```\n\n", string(schemaJSON)))
		}
	}

	return sb.String()
}

func (r *ToolRegistry) ParseToolCall(text string) (*ToolCall, bool) {
	jsonBlock := regexp.MustCompile("```json\\s*\\n(\\{[^`]+\\})\\s*\\n```")
	matches := jsonBlock.FindStringSubmatch(text)
	if len(matches) < 2 {
		jsonInline := regexp.MustCompile(`\{[^{}]*"tool"[^{}]*\}`)
		inlineMatch := jsonInline.FindString(text)
		if inlineMatch == "" {
			return nil, false
		}
		matches = []string{"", inlineMatch}
	}

	var call ToolCall
	if err := json.Unmarshal([]byte(matches[1]), &call); err != nil {
		return nil, false
	}

	if call.Tool == "" {
		return nil, false
	}

	return &call, true
}

type ToolCall struct {
	Tool string                 `json:"tool"`
	Args map[string]interface{} `json:"args"`
}

func (r *ToolRegistry) Execute(call ToolCall) (string, error) {
	tool, ok := r.tools[call.Tool]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", call.Tool)
	}

	return tool.Execute(call.Args)
}

type ReadFileTool struct{}

func (t *ReadFileTool) Name() string        { return "read_file" }
func (t *ReadFileTool) Description() string { return "Read the contents of a file" }
func (t *ReadFileTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the file to read",
			},
		},
		"required": []string{"path"},
	}
}

func (t *ReadFileTool) Execute(args map[string]interface{}) (string, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", err
	}

	content := string(data)
	if len(content) > 50000 {
		content = content[:50000] + "\n\n... (truncated)"
	}

	return content, nil
}

type WriteFileTool struct{}

func (t *WriteFileTool) Name() string        { return "write_file" }
func (t *WriteFileTool) Description() string { return "Write content to a file (creates or overwrites)" }
func (t *WriteFileTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the file to write",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "Content to write to the file",
			},
		},
		"required": []string{"path", "content"},
	}
}

func (t *WriteFileTool) Execute(args map[string]interface{}) (string, error) {
	path, _ := args["path"].(string)
	content, _ := args["content"].(string)

	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
		return "", err
	}

	return fmt.Sprintf("File written: %s", absPath), nil
}

type EditFileTool struct{}

func (t *EditFileTool) Name() string { return "edit_file" }
func (t *EditFileTool) Description() string {
	return "Edit a file by replacing text. Use old_text and new_text for precise edits."
}
func (t *EditFileTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the file to edit",
			},
			"old_text": map[string]interface{}{
				"type":        "string",
				"description": "Text to find and replace (must match exactly)",
			},
			"new_text": map[string]interface{}{
				"type":        "string",
				"description": "New text to replace with",
			},
		},
		"required": []string{"path", "old_text", "new_text"},
	}
}

func (t *EditFileTool) Execute(args map[string]interface{}) (string, error) {
	path, _ := args["path"].(string)
	oldText, _ := args["old_text"].(string)
	newText, _ := args["new_text"].(string)

	if path == "" || oldText == "" {
		return "", fmt.Errorf("path and old_text are required")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", err
	}

	content := string(data)
	if !strings.Contains(content, oldText) {
		return "", fmt.Errorf("old_text not found in %s", path)
	}

	newContent := strings.Replace(content, oldText, newText, 1)

	if err := os.WriteFile(absPath, []byte(newContent), 0644); err != nil {
		return "", err
	}

	return fmt.Sprintf("File edited: %s", absPath), nil
}

type DeleteFileTool struct{}

func (t *DeleteFileTool) Name() string        { return "delete_file" }
func (t *DeleteFileTool) Description() string { return "Delete a file" }
func (t *DeleteFileTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the file to delete",
			},
		},
		"required": []string{"path"},
	}
}

func (t *DeleteFileTool) Execute(args map[string]interface{}) (string, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	if err := os.Remove(absPath); err != nil {
		return "", err
	}

	return fmt.Sprintf("File deleted: %s", absPath), nil
}

type ListDirTool struct{}

func (t *ListDirTool) Name() string        { return "list_dir" }
func (t *ListDirTool) Description() string { return "List files and directories" }
func (t *ListDirTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Directory path (default: current directory)",
			},
		},
	}
}

func (t *ListDirTool) Execute(args map[string]interface{}) (string, error) {
	path, _ := args["path"].(string)
	if path == "" {
		path = "."
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return "", err
	}

	var lines []string
	for _, entry := range entries {
		if entry.IsDir() {
			lines = append(lines, fmt.Sprintf("  📁 %s/", entry.Name()))
		} else {
			lines = append(lines, fmt.Sprintf("  📄 %s", entry.Name()))
		}
	}

	return strings.Join(lines, "\n"), nil
}

type RunCommandTool struct{}

func (t *RunCommandTool) Name() string        { return "run_command" }
func (t *RunCommandTool) Description() string { return "Execute a shell command" }
func (t *RunCommandTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "Command to execute",
			},
		},
		"required": []string{"command"},
	}
}

func (t *RunCommandTool) Execute(args map[string]interface{}) (string, error) {
	command, _ := args["command"].(string)
	if command == "" {
		return "", fmt.Errorf("command is required")
	}

	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output) + "\nError: " + err.Error(), nil
	}

	return string(output), nil
}

type GrepTool struct{}

func (t *GrepTool) Name() string { return "grep" }
func (t *GrepTool) Description() string {
	return "Search for pattern in files using ripgrep"
}
func (t *GrepTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pattern": map[string]interface{}{
				"type":        "string",
				"description": "Regex pattern to search for",
			},
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Directory to search in (default: current)",
			},
		},
		"required": []string{"pattern"},
	}
}

func (t *GrepTool) Execute(args map[string]interface{}) (string, error) {
	pattern, _ := args["pattern"].(string)
	path, _ := args["path"].(string)

	if pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}
	if path == "" {
		path = "."
	}

	results, err := SearchFiles(pattern, 30)
	if err != nil {
		return "", err
	}

	return FormatSearchResults(results), nil
}

type GlobTool struct{}

func (t *GlobTool) Name() string { return "glob" }
func (t *GlobTool) Description() string {
	return "Find files matching a glob pattern"
}
func (t *GlobTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pattern": map[string]interface{}{
				"type":        "string",
				"description": "Glob pattern (e.g., **/*.go, src/**/*.ts)",
			},
		},
		"required": []string{"pattern"},
	}
}

func (t *GlobTool) Execute(args map[string]interface{}) (string, error) {
	pattern, _ := args["pattern"].(string)
	if pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}

	var matches []string
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		matched, _ := filepath.Match(pattern, filepath.Base(path))
		if matched {
			matches = append(matches, path)
		}

		if len(matches) >= 100 {
			return filepath.SkipDir
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	if len(matches) == 0 {
		return "No files matched.", nil
	}

	return strings.Join(matches, "\n"), nil
}
