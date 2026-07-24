package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"plugin"
	"strings"
)

type Plugin struct {
	Name        string
	Description string
	Version     string
	Author      string
	Path        string
	Tools       []PluginTool
	IsNative    bool
	NativePlugin plugin.Plugin
}

type PluginTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Schema      map[string]interface{} `json:"schema"`
	Command     string                 `json:"command"`
	Script      string                 `json:"script"`
}

type PluginManager struct {
	PluginDir string
	Plugins   []*Plugin
}

func NewPluginManager() *PluginManager {
	home, _ := os.UserHomeDir()
	pluginDir := filepath.Join(home, ".config", "nexus", "plugins")

	os.MkdirAll(pluginDir, 0755)

	return &PluginManager{
		PluginDir: pluginDir,
		Plugins:   make([]*Plugin, 0),
	}
}

func (pm *PluginManager) LoadAll() error {
	entries, err := os.ReadDir(pm.PluginDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			pluginPath := filepath.Join(pm.PluginDir, entry.Name())
			if err := pm.LoadPlugin(pluginPath); err != nil {
				fmt.Printf("Failed to load plugin %s: %v\n", entry.Name(), err)
			}
		} else if strings.HasSuffix(entry.Name(), ".json") {
			pluginPath := filepath.Join(pm.PluginDir, entry.Name())
			if err := pm.LoadJSONPlugin(pluginPath); err != nil {
				fmt.Printf("Failed to load plugin %s: %v\n", entry.Name(), err)
			}
		}
	}

	return nil
}

func (pm *PluginManager) LoadPlugin(path string) error {
	p := &Plugin{
		Path: path,
	}

	configPath := filepath.Join(path, "plugin.json")
	if _, err := os.Stat(configPath); err == nil {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return err
		}

		var config struct {
			Name        string       `json:"name"`
			Description string       `json:"description"`
			Version     string       `json:"version"`
			Author      string       `json:"author"`
			Tools       []PluginTool `json:"tools"`
		}

		if err := json.Unmarshal(data, &config); err != nil {
			return err
		}

		p.Name = config.Name
		p.Description = config.Description
		p.Version = config.Version
		p.Author = config.Author
		p.Tools = config.Tools
	}

	soPath := filepath.Join(path, "plugin.so")
	if _, err := os.Stat(soPath); err == nil {
		plug, err := plugin.Open(soPath)
		if err == nil {
			p.NativePlugin = *plug
			p.IsNative = true
		}
	}

	pm.Plugins = append(pm.Plugins, p)
	return nil
}

func (pm *PluginManager) LoadJSONPlugin(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var config struct {
		Name        string       `json:"name"`
		Description string       `json:"description"`
		Version     string       `json:"version"`
		Tools       []PluginTool `json:"tools"`
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	p := &Plugin{
		Name:        config.Name,
		Description: config.Description,
		Version:     config.Version,
		Tools:       config.Tools,
		Path:        path,
	}

	pm.Plugins = append(pm.Plugins, p)
	return nil
}

func (pm *PluginManager) GetAllTools() []PluginTool {
	var tools []PluginTool
	for _, p := range pm.Plugins {
		tools = append(tools, p.Tools...)
	}
	return tools
}

func (pm *PluginManager) GetPlugin(name string) *Plugin {
	for _, p := range pm.Plugins {
		if p.Name == name {
			return p
		}
	}
	return nil
}

func (pm *PluginManager) ExecuteTool(tool PluginTool, args map[string]interface{}) (string, error) {
	if tool.Command != "" {
		return pm.executeCommand(tool, args)
	}

	if tool.Script != "" {
		return pm.executeScript(tool.Script, args)
	}

	return "", fmt.Errorf("no execution method for tool %s", tool.Name)
}

func (pm *PluginManager) executeCommand(tool PluginTool, args map[string]interface{}) (string, error) {
	argsJSON, _ := json.Marshal(args)

	cmd := exec.Command("sh", "-c", tool.Command)
	cmd.Env = append(os.Environ(),
		"NEXUS_TOOL_ARGS="+string(argsJSON),
		"NEXUS_TOOL_NAME="+tool.Name,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output) + "\nError: " + err.Error(), nil
	}

	return string(output), nil
}

func (pm *PluginManager) executeScript(script string, args map[string]interface{}) (string, error) {
	tmpFile, err := os.CreateTemp("", "nexus-plugin-*.sh")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(script); err != nil {
		return "", err
	}
	tmpFile.Close()

	os.Chmod(tmpFile.Name(), 0755)

	argsJSON, _ := json.Marshal(args)

	cmd := exec.Command(tmpFile.Name())
	cmd.Env = append(os.Environ(),
		"NEXUS_TOOL_ARGS="+string(argsJSON),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output) + "\nError: " + err.Error(), nil
	}

	return string(output), nil
}

func (pm *PluginManager) CreatePlugin(name string, tools []PluginTool) error {
	pluginDir := filepath.Join(pm.PluginDir, name)
	os.MkdirAll(pluginDir, 0755)

	config := struct {
		Name        string       `json:"name"`
		Description string       `json:"description"`
		Version     string       `json:"version"`
		Tools       []PluginTool `json:"tools"`
	}{
		Name:   name,
		Version: "1.0.0",
		Tools:  tools,
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(pluginDir, "plugin.json"), data, 0644)
}

func (pm *PluginManager) ListPlugins() string {
	if len(pm.Plugins) == 0 {
		return "No plugins installed.\n\nCreate a plugin in ~/.config/nexus/plugins/<name>/plugin.json"
	}

	var out strings.Builder
	out.WriteString("\033[1;35mPlugins:\033[0m\n\n")

	for _, p := range pm.Plugins {
		out.WriteString(fmt.Sprintf("  \033[1;36m%s\033[0m v%s\n", p.Name, p.Version))
		if p.Description != "" {
			out.WriteString(fmt.Sprintf("    %s\n", p.Description))
		}
		out.WriteString(fmt.Sprintf("    Tools: %d\n\n", len(p.Tools)))
	}

	return out.String()
}
