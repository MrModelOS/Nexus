package internal

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type AgentPhase int

const (
	PhaseThink AgentPhase = iota
	PhasePlan
	PhaseAct
	PhaseVerify
	PhaseReflect
)

var phaseNames = []string{"THINK", "PLAN", "ACT", "VERIFY", "REFLECT"}
var phaseColors = []string{"36", "35", "32", "33", "31"}

type AgentStep struct {
	Phase     AgentPhase
	Thought   string
	Action    string
	Result    string
	Success   bool
	Timestamp time.Time
}

type AgentLoop struct {
	MaxSteps    int
	Steps       []AgentStep
	CurrentStep int
	Plan        string
	Goal        string
	Context     string
	IsRunning   bool
	Error       error
	OnStep      func(step AgentStep)
}

func NewAgentLoop(goal string) *AgentLoop {
	return &AgentLoop{
		MaxSteps: 10,
		Steps:    make([]AgentStep, 0),
		Goal:     goal,
	}
}

func (a *AgentLoop) Start() {
	a.IsRunning = true
	a.CurrentStep = 0
}

func (a *AgentLoop) Stop() {
	a.IsRunning = false
}

func (a *AgentLoop) AddStep(step AgentStep) {
	step.Timestamp = time.Now()
	a.Steps = append(a.Steps, step)
	a.CurrentStep++
}

func (a *AgentLoop) GetPrompt() string {
	var sb strings.Builder

	sb.WriteString("You are an AI agent working on a task. Follow this cycle:\n\n")
	sb.WriteString("1. THINK: Analyze the problem and what needs to be done\n")
	sb.WriteString("2. PLAN: Create a step-by-step plan\n")
	sb.WriteString("3. ACT: Execute one action using tools\n")
	sb.WriteString("4. VERIFY: Check if the action succeeded\n")
	sb.WriteString("5. REFLECT: What did you learn? What's next?\n\n")

	sb.WriteString(fmt.Sprintf("GOAL: %s\n\n", a.Goal))

	if a.Plan != "" {
		sb.WriteString(fmt.Sprintf("CURRENT PLAN:\n%s\n\n", a.Plan))
	}

	if len(a.Steps) > 0 {
		sb.WriteString("HISTORY:\n")
		for i, step := range a.Steps {
			sb.WriteString(fmt.Sprintf("%d. [%s] %s → %s (%v)\n",
				i+1, phaseNames[step.Phase], step.Thought, step.Result, step.Success))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Respond with your next step in this format:\n\n")
	sb.WriteString("```json\n")
	sb.WriteString("{\n")
	sb.WriteString("  \"phase\": \"think|plan|act|verify|reflect\",\n")
	sb.WriteString("  \"thought\": \"your analysis\",\n")
	sb.WriteString("  \"action\": \"tool_name: args (if phase is act)\",\n")
	sb.WriteString("  \"plan\": \"updated plan (if phase is plan)\"\n")
	sb.WriteString("}\n")
	sb.WriteString("```\n")

	return sb.String()
}

func (a *AgentLoop) ParseStep(response string) (*AgentStep, bool) {
	var result struct {
		Phase   string `json:"phase"`
		Thought string `json:"thought"`
		Action  string `json:"action"`
		Plan    string `json:"plan"`
	}

	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, false
	}

	step := &AgentStep{
		Thought: result.Thought,
		Action:  result.Action,
	}

	switch result.Phase {
	case "think":
		step.Phase = PhaseThink
	case "plan":
		step.Phase = PhasePlan
		if result.Plan != "" {
			a.Plan = result.Plan
		}
	case "act":
		step.Phase = PhaseAct
	case "verify":
		step.Phase = PhaseVerify
	case "reflect":
		step.Phase = PhaseReflect
	default:
		step.Phase = PhaseThink
	}

	return step, true
}

func (a *AgentLoop) IsComplete() bool {
	return a.CurrentStep >= a.MaxSteps || !a.IsRunning
}

func (a *AgentLoop) GetSummary() string {
	if len(a.Steps) == 0 {
		return "No steps completed."
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\033[1;35mAgent Summary:\033[0m %d steps\n\n", len(a.Steps)))

	for i, step := range a.Steps {
		color := phaseColors[step.Phase]
		status := "\033[1;31m✗\033[0m"
		if step.Success {
			status = "\033[1;32m✓\033[0m"
		}

		sb.WriteString(fmt.Sprintf("%s \033[1;%sm[%s]\033[0m %s\n",
			status, color, phaseNames[step.Phase], step.Thought))

		if step.Result != "" && i == len(a.Steps)-1 {
			sb.WriteString(fmt.Sprintf("  → %s\n", step.Result))
		}
	}

	return sb.String()
}

type AgentLoopManager struct {
 loops    map[string]*AgentLoop
 current  *AgentLoop
 enabled  bool
}

func NewAgentLoopManager() *AgentLoopManager {
	return &AgentLoopManager{
		loops:   make(map[string]*AgentLoop),
		enabled: true,
	}
}

func (m *AgentLoopManager) StartLoop(goal string) *AgentLoop {
	loop := NewAgentLoop(goal)
	loop.Start()
	m.current = loop
	return loop
}

func (m *AgentLoopManager) GetCurrent() *AgentLoop {
	return m.current
}

func (m *AgentLoopManager) StopCurrent() {
	if m.current != nil {
		m.current.Stop()
		m.loops[m.current.Goal] = m.current
		m.current = nil
	}
}

func (m *AgentLoopManager) IsEnabled() bool {
	return m.enabled
}

func (m *AgentLoopManager) Toggle() {
	m.enabled = !m.enabled
}
