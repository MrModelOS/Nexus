package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type PromptTemplate struct {
	Name        string    `json:"name"`
	Content     string    `json:"content"`
	Description string    `json:"description"`
	Tags        []string  `json:"tags"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	UsageCount  int       `json:"usage_count"`
}

type PromptManager struct {
	Prompts  map[string]PromptTemplate
	FilePath string
}

func NewPromptManager() *PromptManager {
	home, _ := os.UserHomeDir()
	filePath := filepath.Join(home, ".config", "nexus", "prompts.json")

	pm := &PromptManager{
		Prompts:  make(map[string]PromptTemplate),
		FilePath: filePath,
	}

	pm.Load()
	return pm
}

func (pm *PromptManager) Load() error {
	if _, err := os.Stat(pm.FilePath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(pm.FilePath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &pm.Prompts)
}

func (pm *PromptManager) Save() error {
	data, err := json.MarshalIndent(pm.Prompts, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(pm.FilePath)
	os.MkdirAll(dir, 0755)

	return os.WriteFile(pm.FilePath, data, 0644)
}

func (pm *PromptManager) Add(name, content, description string, tags []string) error {
	pm.Prompts[name] = PromptTemplate{
		Name:        name,
		Content:     content,
		Description: description,
		Tags:        tags,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	return pm.Save()
}

func (pm *PromptManager) Get(name string) (string, bool) {
	prompt, ok := pm.Prompts[name]
	if !ok {
		return "", false
	}

	prompt.UsageCount++
	prompt.UpdatedAt = time.Now()
	pm.Prompts[name] = prompt
	pm.Save()

	return prompt.Content, true
}

func (pm *PromptManager) Delete(name string) error {
	delete(pm.Prompts, name)
	return pm.Save()
}

func (pm *PromptManager) List() []PromptTemplate {
	var prompts []PromptTemplate
	for _, p := range pm.Prompts {
		prompts = append(prompts, p)
	}

	sort.Slice(prompts, func(i, j int) bool {
		return prompts[i].UsageCount > prompts[j].UsageCount
	})

	return prompts
}

func (pm *PromptManager) Search(query string) []PromptTemplate {
	var results []PromptTemplate
	query = strings.ToLower(query)

	for _, p := range pm.Prompts {
		if strings.Contains(strings.ToLower(p.Name), query) ||
			strings.Contains(strings.ToLower(p.Description), query) ||
			strings.Contains(strings.ToLower(p.Content), query) {
			results = append(results, p)
		}

		for _, tag := range p.Tags {
			if strings.Contains(strings.ToLower(tag), query) {
				results = append(results, p)
				break
			}
		}
	}

	return results
}

func (pm *PromptManager) RenderList() string {
	prompts := pm.List()
	if len(prompts) == 0 {
		return "No prompts saved.\n\nUse /prompt save <name> to save a prompt."
	}

	var out strings.Builder
	out.WriteString("\033[1;35mPrompt Templates:\033[0m\n\n")

	for _, p := range prompts {
		desc := p.Description
		if desc == "" {
			desc = p.Content[:min(50, len(p.Content))]
			if len(p.Content) > 50 {
				desc += "..."
			}
		}

		out.WriteString(fmt.Sprintf("  \033[1;36m%s\033[0m", p.Name))
		if p.UsageCount > 0 {
			out.WriteString(fmt.Sprintf(" (used %d times)", p.UsageCount))
		}
		out.WriteString("\n")
		out.WriteString(fmt.Sprintf("    %s\n", desc))

		if len(p.Tags) > 0 {
			out.WriteString(fmt.Sprintf("    Tags: %s\n", strings.Join(p.Tags, ", ")))
		}
		out.WriteString("\n")
	}

	return out.String()
}

func (pm *PromptManager) UseTemplate(name string, vars map[string]string) (string, bool) {
	content, ok := pm.Get(name)
	if !ok {
		return "", false
	}

	for key, value := range vars {
		content = strings.ReplaceAll(content, "{{"+key+"}}", value)
	}

	return content, true
}
