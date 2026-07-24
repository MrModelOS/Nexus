package internal

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Skill struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Content     string `yaml:"-"`
	FilePath    string `yaml:"-"`
}

type SkillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func SkillsDirs() []string {
	home, _ := os.UserHomeDir()
	var dirs []string

	dirs = append(dirs, filepath.Join(home, ".config", "nexus", "skills"))

	if wd, err := os.Getwd(); err == nil {
		dirs = append(dirs, filepath.Join(wd, ".nexus", "skills"))
	}

	return dirs
}

func LoadSkills() []Skill {
	var skills []Skill
	loaded := make(map[string]bool)

	for _, dir := range SkillsDirs() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			name := entry.Name()
			if !strings.HasSuffix(name, ".md") {
				continue
			}

			if loaded[name] {
				continue
			}

			path := filepath.Join(dir, entry.Name())
			skill, err := LoadSkill(path)
			if err != nil {
				continue
			}

			skills = append(skills, *skill)
			loaded[name] = true
		}
	}

	return skills
}

func LoadSkill(path string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	content := string(data)
	skill := &Skill{
		FilePath: path,
		Content:  content,
	}

	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) >= 3 {
			var fm SkillFrontmatter
			if err := yaml.Unmarshal([]byte(parts[1]), &fm); err == nil {
				skill.Name = fm.Name
				skill.Description = fm.Description
			}
			skill.Content = strings.TrimSpace(parts[2])
		}
	}

	if skill.Name == "" {
		base := filepath.Base(path)
		skill.Name = strings.TrimSuffix(base, ".md")
	}

	return skill, nil
}

func GetSkillByName(name string) *Skill {
	skills := LoadSkills()
	for _, s := range skills {
		if s.Name == name {
			return &s
		}
	}
	return nil
}

func RenderSkillsList() string {
	skills := LoadSkills()
	if len(skills) == 0 {
		return "No skills found.\n\nCreate a skill in ~/.config/nexus/skills/ or .nexus/skills/"
	}

	var lines []string
	lines = append(lines, "\033[1;35mAvailable Skills:\033[0m")
	lines = append(lines, "")

	for _, s := range skills {
		lines = append(lines, "  \033[1;33m$"+s.Name+"\033[0m")
		if s.Description != "" {
			lines = append(lines, "    "+s.Description)
		}
	}

	lines = append(lines, "")
	lines = append(lines, "\033[90mUse $<skill-name> to activate a skill\033[0m")

	return strings.Join(lines, "\n")
}
