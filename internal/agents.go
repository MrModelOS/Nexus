package internal

import (
	"fmt"
	"strings"
)

type AgentProfile int

const (
	AgentBuild AgentProfile = iota
	AgentPlan
)

type AgentInfo struct {
	Name        string
	Description string
	CanWrite    bool
	CanExecute  bool
}

var agents = map[AgentProfile]AgentInfo{
	AgentBuild: {
		Name:        "build",
		Description: "Full access agent for development",
		CanWrite:    true,
		CanExecute:  true,
	},
	AgentPlan: {
		Name:        "plan",
		Description: "Read-only agent for analysis and planning",
		CanWrite:    false,
		CanExecute:  false,
	},
}

func GetAgentInfo(profile AgentProfile) AgentInfo {
	return agents[profile]
}

func (p AgentProfile) String() string {
	return agents[p].Name
}

func (p AgentProfile) Toggle() AgentProfile {
	if p == AgentBuild {
		return AgentPlan
	}
	return AgentBuild
}

func RenderAgentBadge(profile AgentProfile) string {
	info := agents[profile]

	color := "12"
	if profile == AgentPlan {
		color = "4"
	}

	return fmt.Sprintf("\033[1;%sm%s\033[0m", color, info.Name)
}

func ParseAgentProfile(s string) AgentProfile {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "plan" {
		return AgentPlan
	}
	return AgentBuild
}

func (p AgentProfile) CanEdit() bool {
	return agents[p].CanWrite
}

func (p AgentProfile) CanRunCommands() bool {
	return agents[p].CanExecute
}
