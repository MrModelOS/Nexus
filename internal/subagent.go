package internal

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/nexus-cli/nexus/client"
)

type SubAgentStatus int

const (
	SubAgentPending SubAgentStatus = iota
	SubAgentRunning
	SubAgentDone
	SubAgentFailed
	SubAgentCancelled
)

var subAgentStatusNames = []string{"pending", "running", "done", "failed", "cancelled"}
var subAgentStatusColors = []string{"3", "4", "82", "196", "245"}

type SubAgent struct {
	ID          string
	Task        string
	Status      SubAgentStatus
	Result      string
	Error       error
	StartedAt   time.Time
	FinishedAt  time.Time
	Duration    time.Duration
	AgentType   string
	Messages    []client.Message
	Steps       []SubAgentStep
}

type SubAgentStep struct {
	Action    string
	Result    string
	Success   bool
	Timestamp time.Time
}

type SubAgentManager struct {
	mu          sync.RWMutex
	agents      map[string]*SubAgent
	maxConc     int
	running     int
	onUpdate    func(*SubAgent)
	client      *client.MultiProviderClient
	modelName   string
	temperature float64
}

func NewSubAgentManager(maxConcurrent int) *SubAgentManager {
	return &SubAgentManager{
		agents:      make(map[string]*SubAgent),
		maxConc:     maxConcurrent,
		onUpdate:    nil,
	}
}

func (sam *SubAgentManager) SetClient(c *client.MultiProviderClient, model string, temp float64) {
	sam.mu.Lock()
	defer sam.mu.Unlock()
	sam.client = c
	sam.modelName = model
	sam.temperature = temp
}

func (sam *SubAgentManager) OnUpdate(handler func(*SubAgent)) {
	sam.mu.Lock()
	defer sam.mu.Unlock()
	sam.onUpdate = handler
}

func (sam *SubAgentManager) emitUpdate(agent *SubAgent) {
	sam.mu.RLock()
	handler := sam.onUpdate
	sam.mu.RUnlock()

	if handler != nil {
		go handler(agent)
	}
}

func (sam *SubAgentManager) Spawn(task string, agentType string) (*SubAgent, error) {
	sam.mu.Lock()
	defer sam.mu.Unlock()

	if sam.running >= sam.maxConc {
		return nil, fmt.Errorf("max concurrent agents reached (%d)", sam.maxConc)
	}

	id := fmt.Sprintf("sa-%d", len(sam.agents)+1)
	agent := &SubAgent{
		ID:        id,
		Task:      task,
		Status:    SubAgentPending,
		AgentType: agentType,
		StartedAt: time.Now(),
		Steps:     make([]SubAgentStep, 0),
	}

	sam.agents[id] = agent
	sam.running++

	go sam.run(agent)

	return agent, nil
}

func (sam *SubAgentManager) run(agent *SubAgent) {
	agent.Status = SubAgentRunning
	agent.StartedAt = time.Now()
	sam.emitUpdate(agent)

	prompt := sam.buildPrompt(agent)

	agent.Messages = []client.Message{
		{Role: "user", Content: prompt},
	}

	var fullResponse strings.Builder

	err := sam.client.StreamChat(agent.Messages, sam.modelName, sam.temperature, func(token string) error {
		fullResponse.WriteString(token)
		return nil
	})

	if err != nil {
		agent.Status = SubAgentFailed
		agent.Error = err
		agent.FinishedAt = time.Now()
		agent.Duration = agent.FinishedAt.Sub(agent.StartedAt)

		sam.mu.Lock()
		sam.running--
		sam.mu.Unlock()

		sam.emitUpdate(agent)
		return
	}

	agent.Result = fullResponse.String()
	agent.Status = SubAgentDone
	agent.FinishedAt = time.Now()
	agent.Duration = agent.FinishedAt.Sub(agent.StartedAt)

	sam.mu.Lock()
	sam.running--
	sam.mu.Unlock()

	sam.emitUpdate(agent)
}

func (sam *SubAgentManager) buildPrompt(agent *SubAgent) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("You are a sub-agent with ID: %s\n", agent.ID))
	sb.WriteString(fmt.Sprintf("Task type: %s\n", agent.AgentType))
	sb.WriteString(fmt.Sprintf("Task: %s\n\n", agent.Task))

	sb.WriteString("Complete this task independently. Provide:\n")
	sb.WriteString("1. A brief summary of what you did\n")
	sb.WriteString("2. Any files you created or modified\n")
	sb.WriteString("3. Any issues encountered\n")

	if len(agent.Steps) > 0 {
		sb.WriteString("\nPrevious steps:\n")
		for _, step := range agent.Steps {
			sb.WriteString(fmt.Sprintf("- %s: %s (success: %v)\n", step.Action, step.Result, step.Success))
		}
	}

	return sb.String()
}

func (sam *SubAgentManager) Get(id string) (*SubAgent, bool) {
	sam.mu.RLock()
	defer sam.mu.RUnlock()
	agent, ok := sam.agents[id]
	return agent, ok
}

func (sam *SubAgentManager) GetAll() []*SubAgent {
	sam.mu.RLock()
	defer sam.mu.RUnlock()

	agents := make([]*SubAgent, 0, len(sam.agents))
	for _, a := range sam.agents {
		agents = append(agents, a)
	}
	return agents
}

func (sam *SubAgentManager) GetRunning() []*SubAgent {
	sam.mu.RLock()
	defer sam.mu.RUnlock()

	var running []*SubAgent
	for _, a := range sam.agents {
		if a.Status == SubAgentRunning || a.Status == SubAgentPending {
			running = append(running, a)
		}
	}
	return running
}

func (sam *SubAgentManager) Cancel(id string) bool {
	sam.mu.Lock()
	defer sam.mu.Unlock()

	agent, ok := sam.agents[id]
	if !ok {
		return false
	}

	if agent.Status == SubAgentRunning || agent.Status == SubAgentPending {
		agent.Status = SubAgentCancelled
		agent.FinishedAt = time.Now()
		agent.Duration = agent.FinishedAt.Sub(agent.StartedAt)
		sam.running--
		sam.emitUpdate(agent)
		return true
	}

	return false
}

func (sam *SubAgentManager) CancelAll() {
	sam.mu.Lock()
	defer sam.mu.Unlock()

	for _, agent := range sam.agents {
		if agent.Status == SubAgentRunning || agent.Status == SubAgentPending {
			agent.Status = SubAgentCancelled
			agent.FinishedAt = time.Now()
			agent.Duration = agent.FinishedAt.Sub(agent.StartedAt)
			sam.running--
			sam.emitUpdate(agent)
		}
	}
}

func (sam *SubAgentManager) GetStatus() string {
	sam.mu.RLock()
	defer sam.mu.RUnlock()

	var out strings.Builder
	out.WriteString("\033[1;35mSub-Agents:\033[0m\n\n")
	out.WriteString(fmt.Sprintf("Running: %d/%d\n", sam.running, sam.maxConc))

	if len(sam.agents) == 0 {
		out.WriteString("\n\033[1;245mNo sub-agents spawned yet.\033[0m\n")
		out.WriteString("\033[1;245mAI can spawn them with spawn_subagent tool.\033[0m\n")
		return out.String()
	}

	byStatus := make(map[SubAgentStatus][]*SubAgent)
	for _, a := range sam.agents {
		byStatus[a.Status] = append(byStatus[a.Status], a)
	}

	for status, agents := range byStatus {
		if len(agents) == 0 {
			continue
		}

		color := subAgentStatusColors[status]
		out.WriteString(fmt.Sprintf("\033[1;%sm%s:\033[0m\n", color, strings.ToUpper(subAgentStatusNames[status])))

		for _, a := range agents {
			out.WriteString(fmt.Sprintf("  %s %s\n", a.ID, truncate(a.Task, 40)))
			if a.Status == SubAgentDone && a.Result != "" {
				out.WriteString(fmt.Sprintf("    → %s\n", truncate(a.Result, 60)))
			}
			if a.Error != nil {
				out.WriteString(fmt.Sprintf("    → \033[1;31m%s\033[0m\n", a.Error.Error()))
			}
			if a.Duration > 0 {
				out.WriteString(fmt.Sprintf("    Duration: %s\n", a.Duration.Round(time.Millisecond)))
			}
		}
		out.WriteString("\n")
	}

	return out.String()
}

func (sam *SubAgentManager) GetSummary() string {
	sam.mu.RLock()
	defer sam.mu.RUnlock()

	total := len(sam.agents)
	done := 0
	failed := 0
	running := 0

	for _, a := range sam.agents {
		switch a.Status {
		case SubAgentDone:
			done++
		case SubAgentFailed:
			failed++
		case SubAgentRunning, SubAgentPending:
			running++
		}
	}

	return fmt.Sprintf("Sub-agents: %d total, %d done, %d running, %d failed", total, done, running, failed)
}

func (sam *SubAgentManager) CollectResults() string {
	sam.mu.RLock()
	defer sam.mu.RUnlock()

	var out strings.Builder
	out.WriteString("\033[1;35mSub-Agent Results:\033[0m\n\n")

	for _, a := range sam.agents {
		if a.Status == SubAgentDone {
			out.WriteString(fmt.Sprintf("\033[1;82m✓ %s\033[0m (%s)\n", a.ID, a.AgentType))
			out.WriteString(fmt.Sprintf("  Task: %s\n", a.Task))
			out.WriteString(fmt.Sprintf("  Result:\n%s\n\n", a.Result))
		}
	}

	for _, a := range sam.agents {
		if a.Status == SubAgentFailed {
			out.WriteString(fmt.Sprintf("\033[1;196m✗ %s\033[0m (%s)\n", a.ID, a.AgentType))
			out.WriteString(fmt.Sprintf("  Task: %s\n", a.Task))
			out.WriteString(fmt.Sprintf("  Error: %s\n\n", a.Error.Error()))
		}
	}

	return out.String()
}

type SubAgentTool struct {
	manager *SubAgentManager
}

func (t *SubAgentTool) Name() string        { return "spawn_subagent" }
func (t *SubAgentTool) Description() string { return "Spawn a sub-agent to handle a task in parallel" }
func (t *SubAgentTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"task": map[string]interface{}{
				"type":        "string",
				"description": "Task description for the sub-agent",
			},
			"type": map[string]interface{}{
				"type":        "string",
				"description": "Agent type: research, code, test, docs, review",
				"enum":        []string{"research", "code", "test", "docs", "review"},
			},
		},
		"required": []string{"task"},
	}
}

func (t *SubAgentTool) Execute(args map[string]interface{}) (string, error) {
	task, _ := args["task"].(string)
	agentType, _ := args["type"].(string)

	if task == "" {
		return "", fmt.Errorf("task is required")
	}

	if agentType == "" {
		agentType = "research"
	}

	agent, err := t.manager.Spawn(task, agentType)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Sub-agent %s spawned for: %s", agent.ID, task), nil
}

type SubAgentStatusTool struct {
	manager *SubAgentManager
}

func (t *SubAgentStatusTool) Name() string        { return "subagent_status" }
func (t *SubAgentStatusTool) Description() string { return "Check status of sub-agents" }
func (t *SubAgentStatusTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{
				"type":        "string",
				"description": "Sub-agent ID (optional, returns all if empty)",
			},
		},
	}
}

func (t *SubAgentStatusTool) Execute(args map[string]interface{}) (string, error) {
	id, _ := args["id"].(string)

	if id != "" {
		agent, ok := t.manager.Get(id)
		if !ok {
			return "", fmt.Errorf("sub-agent %s not found", id)
		}
		return fmt.Sprintf("Agent %s: %s\nTask: %s\nResult: %s",
			agent.ID, subAgentStatusNames[agent.Status], agent.Task, agent.Result), nil
	}

	return t.manager.GetStatus(), nil
}

type SubAgentCollectTool struct {
	manager *SubAgentManager
}

func (t *SubAgentCollectTool) Name() string        { return "collect_subagents" }
func (t *SubAgentCollectTool) Description() string { return "Collect results from all completed sub-agents" }
func (t *SubAgentCollectTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (t *SubAgentCollectTool) Execute(args map[string]interface{}) (string, error) {
	return t.manager.CollectResults(), nil
}

type SubAgentCancelTool struct {
	manager *SubAgentManager
}

func (t *SubAgentCancelTool) Name() string        { return "cancel_subagent" }
func (t *SubAgentCancelTool) Description() string { return "Cancel a running sub-agent" }
func (t *SubAgentCancelTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{
				"type":        "string",
				"description": "Sub-agent ID to cancel",
			},
		},
		"required": []string{"id"},
	}
}

func (t *SubAgentCancelTool) Execute(args map[string]interface{}) (string, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return "", fmt.Errorf("id is required")
	}

	if t.manager.Cancel(id) {
		return fmt.Sprintf("Sub-agent %s cancelled", id), nil
	}
	return "", fmt.Errorf("could not cancel sub-agent %s", id)
}
