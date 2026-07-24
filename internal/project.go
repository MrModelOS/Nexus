package internal

import (
	"os"
	"path/filepath"
	"strings"
)

type ProjectContext struct {
	Instructions string
	MemoryPath   string
	Memory       string
	RootDir      string
}

func FindProjectRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}

	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "NEXUS.md")); err == nil {
			return dir
		}
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return cwd
}

func LoadProjectContext() *ProjectContext {
	root := FindProjectRoot()
	if root == "" {
		return nil
	}

	ctx := &ProjectContext{
		RootDir:  root,
		MemoryPath: filepath.Join(root, ".nexus", "memory.md"),
	}

	nexusPath := filepath.Join(root, "NEXUS.md")
	if data, err := os.ReadFile(nexusPath); err == nil {
		ctx.Instructions = string(data)
	}

	os.MkdirAll(filepath.Join(root, ".nexus"), 0755)

	if data, err := os.ReadFile(ctx.MemoryPath); err == nil {
		ctx.Memory = string(data)
	}

	return ctx
}

func (ctx *ProjectContext) SaveMemory(memory string) error {
	if ctx == nil || ctx.MemoryPath == "" {
		return nil
	}
	return os.WriteFile(ctx.MemoryPath, []byte(memory), 0644)
}

func (ctx *ProjectContext) AddMemoryEntry(entry string) error {
	if ctx == nil {
		return nil
	}

	existing := ctx.Memory
	if existing != "" {
		existing = strings.TrimRight(existing, "\n") + "\n"
	}

	existing += "- " + entry + "\n"
	ctx.Memory = existing

	return ctx.SaveMemory(existing)
}

func (ctx *ProjectContext) GetSystemPrompt() string {
	if ctx == nil {
		return ""
	}

	var parts []string

	if ctx.Instructions != "" {
		parts = append(parts, "## Project Instructions\n\n"+ctx.Instructions)
	}

	if ctx.Memory != "" {
		parts = append(parts, "## Project Memory\n\n"+ctx.Memory)
	}

	return strings.Join(parts, "\n\n")
}
