package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type PermissionLevel int

const (
	PermRead PermissionLevel = iota
	PermWrite
	PermExecute
	PermAdmin
)

type Permission struct {
	Path      string
	Level     PermissionLevel
	Recursive bool
}

type PermissionManager struct {
	ProjectDir    string
	GlobalPerms   []Permission
	ProjectPerms  []Permission
	AutoApprove   bool
	AskEveryTime  bool
}

func NewPermissionManager(projectDir string) *PermissionManager {
	pm := &PermissionManager{
		ProjectDir:   projectDir,
		AutoApprove:  false,
		AskEveryTime: true,
	}

	pm.loadPermissions()

	return pm
}

func (pm *PermissionManager) loadPermissions() {
	home, _ := os.UserHomeDir()
	globalPath := filepath.Join(home, ".config", "nexus", "permissions.json")

	if data, err := os.ReadFile(globalPath); err == nil {
		json.Unmarshal(data, &pm.GlobalPerms)
	}

	if pm.ProjectDir != "" {
		projectPath := filepath.Join(pm.ProjectDir, ".nexus", "permissions.json")
		if data, err := os.ReadFile(projectPath); err == nil {
			json.Unmarshal(data, &pm.ProjectPerms)
		}
	}
}

func (pm *PermissionManager) SavePermissions() error {
	home, _ := os.UserHomeDir()
	globalPath := filepath.Join(home, ".config", "nexus", "permissions.json")

	dir := filepath.Dir(globalPath)
	os.MkdirAll(dir, 0755)

	data, err := json.MarshalIndent(pm.GlobalPerms, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(globalPath, data, 0644); err != nil {
		return err
	}

	if pm.ProjectDir != "" {
		projectPath := filepath.Join(pm.ProjectDir, ".nexus", "permissions.json")
		pdir := filepath.Dir(projectPath)
		os.MkdirAll(pdir, 0755)

		pdata, err := json.MarshalIndent(pm.ProjectPerms, "", "  ")
		if err != nil {
			return err
		}

		if err := os.WriteFile(projectPath, pdata, 0644); err != nil {
			return err
		}
	}

	return nil
}

func (pm *PermissionManager) CheckPermission(path string, level PermissionLevel) bool {
	absPath, _ := filepath.Abs(path)

	for _, perm := range pm.ProjectPerms {
		if pm.matchesPermission(absPath, perm) && level <= perm.Level {
			return true
		}
	}

	for _, perm := range pm.GlobalPerms {
		if pm.matchesPermission(absPath, perm) && level <= perm.Level {
			return true
		}
	}

	return false
}

func (pm *PermissionManager) matchesPermission(path string, perm Permission) bool {
	permPath, _ := filepath.Abs(perm.Path)

	if perm.Recursive {
		return strings.HasPrefix(path, permPath)
	}

	return path == permPath || filepath.Dir(path) == permPath
}

func (pm *PermissionManager) GrantPermission(path string, level PermissionLevel, recursive bool) {
	absPath, _ := filepath.Abs(path)

	for i, perm := range pm.ProjectPerms {
		if perm.Path == absPath {
			pm.ProjectPerms[i].Level = level
			pm.ProjectPerms[i].Recursive = recursive
			return
		}
	}

	pm.ProjectPerms = append(pm.ProjectPerms, Permission{
		Path:      absPath,
		Level:     level,
		Recursive: recursive,
	})
}

func (pm *PermissionManager) RevokePermission(path string) {
	absPath, _ := filepath.Abs(path)

	var filtered []Permission
	for _, perm := range pm.ProjectPerms {
		if perm.Path != absPath {
			filtered = append(filtered, perm)
		}
	}
	pm.ProjectPerms = filtered
}

func (pm *PermissionManager) GetPermissionSummary() string {
	var lines []string
	lines = append(lines, "\033[1;35mPermissions:\033[0m")

	if len(pm.ProjectPerms) > 0 {
		lines = append(lines, "\n  Project permissions:")
		for _, perm := range pm.ProjectPerms {
			lines = append(lines, fmt.Sprintf("    %s [%s] %s",
				perm.Path,
				pm.levelName(perm.Level),
				pm.recursiveStr(perm.Recursive),
			))
		}
	}

	if len(pm.GlobalPerms) > 0 {
		lines = append(lines, "\n  Global permissions:")
		for _, perm := range pm.GlobalPerms {
			lines = append(lines, fmt.Sprintf("    %s [%s] %s",
				perm.Path,
				pm.levelName(perm.Level),
				pm.recursiveStr(perm.Recursive),
			))
		}
	}

	if len(pm.ProjectPerms) == 0 && len(pm.GlobalPerms) == 0 {
		lines = append(lines, "  No permissions configured")
		lines = append(lines, "  Use /perm grant <path> <level> to add")
	}

	return strings.Join(lines, "\n")
}

func (pm *PermissionManager) levelName(level PermissionLevel) string {
	switch level {
	case PermRead:
		return "read"
	case PermWrite:
		return "write"
	case PermExecute:
		return "execute"
	case PermAdmin:
		return "admin"
	default:
		return "unknown"
	}
}

func (pm *PermissionManager) recursiveStr(recursive bool) string {
	if recursive {
		return "(recursive)"
	}
	return ""
}

func (pm *PermissionManager) ParseLevel(s string) PermissionLevel {
	switch strings.ToLower(s) {
	case "read", "r":
		return PermRead
	case "write", "w":
		return PermWrite
	case "execute", "exec", "x":
		return PermExecute
	case "admin", "a":
		return PermAdmin
	default:
		return PermRead
	}
}
