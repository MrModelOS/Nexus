package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/nexus-cli/nexus/client"
	"github.com/nexus-cli/nexus/config"
)

type PermissionMode int

const (
	PermManual PermissionMode = iota
	PermAcceptEdits
	PermPlan
	PermAuto
)

var permModeNames = []string{"Manual", "Accept Edits", "Plan", "Auto"}
var permModeColors = []string{"1", "3", "4", "10"}

type Mode int

const (
	ModeNormal Mode = iota
	ModeCommandHint
	ModeModelPicker
	ModeSessionPicker
	ModeSkillPicker
	ModeApproval
	ModeConnect
)

type Model struct {
	viewport        viewport.Model
	textinput       textinput.Model
	history         []ChatMessage
	output          []string
	width           int
	height          int
	ready           bool
	streaming       bool
	streamBuf       string
	provider        string
	modelName       string
	client          *client.Client
	multiClient     *client.MultiProviderClient
	cfg             *config.Config
	err             error
	mode            Mode
	hintCursor      int
	filteredCmds    []int
	modelPicker     bool
	modelCursor     int
	models          []string
	permMode        PermissionMode
	agent           AgentProfile
	queue           []string
	session         *Session
	sessionID       string
	sessions        []SessionMeta
	sessionCursor   int
	skillCursor     int
	skills          []Skill
	projectCtx      *ProjectContext
	gitInfo         *GitInfo
	pendingApproval string
	approvalResolve func(bool)
	autoFix         bool
	fixLoop         int
	searchCursor    int
	mcpServers      []*MCPServer
	tools           *ToolRegistry
	permissions     *PermissionManager
	context         *ContextManager
	pendingToolCall *ToolCall
	vimMode         bool
	agentLoop       *AgentLoopManager
	thinking        *ThinkingState
	plugins         *PluginManager
	vault           *Vault
	costs           *CostTracker
	prompts         *PromptManager
	watcher         *Watcher
	diff            *DiffViewer
	failover        *FailoverManager
	compressor      *ContextCompressor
	connectState    *ConnectState
}

func NewTUI(provider, modelName string, cfg *config.Config) Model {
	ti := textinput.New()
	ti.Placeholder = "Ask me anything..."
	ti.Focus()
	ti.CharLimit = 4096
	ti.Width = 80

	var models []string
	var multiClient *client.MultiProviderClient

	if cfg != nil {
		for name, p := range cfg.Providers {
			for _, m := range p.Models {
				models = append(models, m)
			}

			if name == provider || (provider == "" && name == "ollama") {
				providerType := client.ProviderType(p.Type)
				if providerType == "" {
					providerType = client.ProviderOllama
				}
				multiClient = client.NewMultiProvider(providerType, p.BaseURL, p.APIKey)
			}
		}
	}

	session := &Session{
		ID:        GenerateSessionID(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Model:     modelName,
		Provider:  provider,
		Messages:  make([]ChatMessage, 0),
	}

	projectCtx := LoadProjectContext()

	projectDir, _ := os.Getwd()

	pm := NewPluginManager()
	pm.LoadAll()

	return Model{
		textinput:   ti,
		provider:    provider,
		modelName:   modelName,
		cfg:         cfg,
		models:      models,
		multiClient: multiClient,
		permMode:    PermAuto,
		agent:       AgentBuild,
		session:     session,
		skills:      LoadSkills(),
		projectCtx:  projectCtx,
		gitInfo:     GetGitInfo(),
		tools:       NewToolRegistry(),
		permissions: NewPermissionManager(projectDir),
		context:     NewContextManager(projectDir),
		agentLoop:   NewAgentLoopManager(),
		thinking:    NewThinkingState(),
		plugins:     pm,
		vault:       NewVault(),
		costs:       NewCostTracker(),
		prompts:     NewPromptManager(),
		failover:    NewFailoverManager(cfg),
		compressor:  NewContextCompressor(cfg),
	}
}

func (m *Model) SetClient(c *client.Client) {
	m.client = c
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		tea.EnterAltScreen,
	)
}

func (m *Model) filterCommands() {
	input := m.textinput.Value()
	m.filteredCmds = nil

	if !strings.HasPrefix(input, "/") {
		return
	}

	commands := getCommands()
	for i, c := range commands {
		if strings.HasPrefix(c.cmd, input) || input == "/" {
			m.filteredCmds = append(m.filteredCmds, i)
		}
	}

	if m.hintCursor >= len(m.filteredCmds) {
		m.hintCursor = 0
	}
	if m.hintCursor < 0 && len(m.filteredCmds) > 0 {
		m.hintCursor = 0
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	if m.mode == ModeApproval {
		return m.updateApproval(msg)
	}

	if m.mode == ModeConnect {
		return m.updateConnect(msg)
	}

	if m.modelPicker {
		return m.updateModelPicker(msg)
	}

	if m.mode == ModeSessionPicker {
		return m.updateSessionPicker(msg)
	}

	if m.mode == ModeSkillPicker {
		return m.updateSkillPicker(msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if !m.ready {
			m.viewport = viewport.New(m.width, 0)
			m.ready = true
		}
		m.updateLayout()

	case tea.KeyMsg:
		if m.showHints() && len(m.filteredCmds) > 0 {
			switch msg.Type {
			case tea.KeyUp:
				m.hintCursor--
				if m.hintCursor < 0 {
					m.hintCursor = len(m.filteredCmds) - 1
				}
				m.updateLayout()
				return m, nil
			case tea.KeyDown:
				m.hintCursor++
				if m.hintCursor >= len(m.filteredCmds) {
					m.hintCursor = 0
				}
				m.updateLayout()
				return m, nil
			}
		}

		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			if m.mode == ModeCommandHint {
				m.mode = ModeNormal
				m.hintCursor = 0
				m.updateLayout()
				return m, nil
			}
			AutoSaveSession(m.session)
			if m.projectCtx != nil {
				m.projectCtx.AddMemoryEntry(fmt.Sprintf("Session ended at %s with %d messages", time.Now().Format("15:04"), len(m.history)))
			}
			return m, tea.Quit

		case tea.KeyShiftTab:
			m.agent = m.agent.Toggle()
			m.output = append(m.output, fmt.Sprintf("\033[1;33mAgent:\033[0m %s mode", m.agent.String()))
			m.updateLayout()
			return m, nil

		case tea.KeyTab:
			if m.showHints() && len(m.filteredCmds) > 0 {
				commands := getCommands()
				selected := commands[m.filteredCmds[m.hintCursor]]
				m.textinput.SetValue(selected.cmd + " ")
				m.mode = ModeNormal
				m.hintCursor = 0
				m.updateLayout()
				return m, nil
			}
			m.permMode = (m.permMode + 1) % 4
			m.output = append(m.output, fmt.Sprintf("\033[1;33mPermissions:\033[0m %s", permModeNames[m.permMode]))
			m.updateLayout()
			return m, nil

		case tea.KeyEnter:
			if m.streaming {
				return m, nil
			}

			value := strings.TrimSpace(m.textinput.Value())
			if value == "" {
				return m, nil
			}

			cmds = append(cmds, m.handleCommand(value)...)
			return m, tea.Batch(cmds...)

		case tea.KeyUp:
			if !m.showHints() {
				m.viewport.LineUp(1)
			}
		case tea.KeyDown:
			if !m.showHints() {
				m.viewport.LineDown(1)
			}
		case tea.KeyPgUp:
			m.viewport.HalfViewUp()
		case tea.KeyPgDown:
			m.viewport.HalfViewDown()
		}

	case StreamTokenMsg:
		m.streamBuf += msg.Token
		m.updateStreamingDisplay()
		cmds = append(cmds, m.waitForToken())

	case StreamDoneMsg:
		m.streaming = false
		if msg.Error != nil {
			m.err = msg.Error
			m.output = append(m.output, fmt.Sprintf("\033[1;31mError:\033[0m %v", msg.Error))
		} else {
			m.history = append(m.history, ChatMessage{Role: "assistant", Content: msg.FullResponse})
			m.session.Messages = append(m.session.Messages, ChatMessage{Role: "assistant", Content: msg.FullResponse})
			rendered := m.renderMarkdown(msg.FullResponse)
			m.output = append(m.output, fmt.Sprintf("\033[1;32mNexus:\033[0m\n%s", rendered))
		}
		m.streamBuf = ""
		m.updateLayout()

		if len(m.queue) > 0 {
			next := m.queue[0]
			m.queue = m.queue[1:]
			cmds = append(cmds, m.sendMessage(next))
		}

	case ToolResultMsg:
		m.streaming = false
		toolName := msg.Call.Tool
		result := msg.Result

		m.history = append(m.history, ChatMessage{
			Role:    "assistant",
			Content: fmt.Sprintf("[Used tool: %s]", toolName),
		})
		m.session.Messages = append(m.session.Messages, ChatMessage{
			Role:    "assistant",
			Content: fmt.Sprintf("[Used tool: %s]", toolName),
		})

		m.history = append(m.history, ChatMessage{
			Role:    "user",
			Content: fmt.Sprintf("Tool result for %s:\n%s", toolName, result),
		})
		m.session.Messages = append(m.session.Messages, ChatMessage{
			Role:    "user",
			Content: fmt.Sprintf("Tool result for %s:\n%s", toolName, result),
		})

		m.output = append(m.output, fmt.Sprintf("\033[1;35mTool:\033[0m %s", toolName))

		rendered := m.renderMarkdown(result)
		m.output = append(m.output, fmt.Sprintf("\033[1;32mResult:\033[0m\n%s", rendered))

		m.updateLayout()

		if len(m.queue) > 0 {
			next := m.queue[0]
			m.queue = m.queue[1:]
			cmds = append(cmds, m.sendMessage(next))
		}

	case NotificationMsg:
		color := "\033[1;33m"
		if msg.Type == "error" {
			color = "\033[1;31m"
		} else if msg.Type == "success" {
			color = "\033[1;32m"
		} else if msg.Type == "failover" {
			color = "\033[1;35m"
		}
		m.output = append(m.output, fmt.Sprintf("%s⚡ %s\033[0m", color, msg.Message))
		m.updateLayout()

	case FailoverNotificationMsg:
		event := msg.Event
		var notifMsg string
		switch event.Type {
		case "switch":
			notifMsg = fmt.Sprintf("Switched to %s (%s)", event.Model, event.Provider)
		case "failover":
			notifMsg = fmt.Sprintf("Failover: %s → %s (reason: %s)", event.Model, event.Provider, event.Error)
		case "error":
			notifMsg = fmt.Sprintf("Model error: %s - %s", event.Model, event.Error)
		}
		if notifMsg != "" {
			m.output = append(m.output, fmt.Sprintf("\033[1;35m⚡ %s\033[0m", notifMsg))
			m.updateLayout()
		}

	case CompressionNotificationMsg:
		event := msg.Event
		notifMsg := fmt.Sprintf("Context compressed: %d tokens (%d%%)", event.TokensUsed, event.Percent)
		m.output = append(m.output, fmt.Sprintf("\033[1;33m📦 %s\033[0m", notifMsg))
		m.streaming = false
		m.updateLayout()
	}

	var cmd tea.Cmd
	m.textinput, cmd = m.textinput.Update(msg)
	cmds = append(cmds, cmd)

	input := m.textinput.Value()
	if strings.HasPrefix(input, "/") {
		m.mode = ModeCommandHint
		m.filterCommands()
	} else {
		m.mode = ModeNormal
		m.hintCursor = 0
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *Model) updateApproval(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case msg.String() == "y" || msg.String() == "Y" || msg.Type == tea.KeyEnter:
			m.mode = ModeNormal
			pending := m.pendingApproval
			m.pendingApproval = ""
			resolve := m.approvalResolve
			m.approvalResolve = nil
			if resolve != nil {
				resolve(true)
			}
			m.output = append(m.output, fmt.Sprintf("\033[1;32mApproved:\033[0m %s", pending))
			m.updateLayout()
			return m, nil

		case msg.String() == "n" || msg.String() == "N" || msg.Type == tea.KeyEsc:
			m.mode = ModeNormal
			m.pendingApproval = ""
			resolve := m.approvalResolve
			m.approvalResolve = nil
			if resolve != nil {
				resolve(false)
			}
			m.output = append(m.output, "\033[1;31mDenied\033[0m")
			m.updateLayout()
			return m, nil
		}
	}
	return m, nil
}

func (m *Model) updateConnect(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.connectState == nil {
		m.mode = ModeNormal
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()

		switch key {
		case "esc":
			m.mode = ModeNormal
			m.connectState = nil
			m.updateLayout()
			return m, nil

		case "up":
			m.connectState.HandleKey("up")
			m.updateLayout()
			return m, nil

		case "down":
			m.connectState.HandleKey("down")
			m.updateLayout()
			return m, nil

		case "enter":
			m.connectState.HandleKey("enter")

			if m.connectState.Step == ConnectDone {
				m.cfg = m.connectState.ApplyConfig(m.cfg)
				config.Save(m.cfg)

				p := PopularProviders[m.connectState.Provider]
				m.provider = strings.ToLower(p.Name)
				m.provider = strings.ReplaceAll(m.provider, " ", "")
				m.provider = strings.ReplaceAll(m.provider, "(", "")
				m.provider = strings.ReplaceAll(m.provider, ")", "")
				m.provider = strings.ReplaceAll(m.provider, "local", "")

				if len(p.Models) > 0 && m.connectState.ModelCursor < len(p.Models) {
					m.modelName = p.Models[m.connectState.ModelCursor]
				}

				m.output = append(m.output, fmt.Sprintf("\033[1;32m✓ Connected to %s\033[0m", p.Name))
				m.mode = ModeNormal
				m.connectState = nil
			}

			m.updateLayout()
			return m, nil

		default:
			if m.connectState.Step == ConnectEnterAPIKey {
				if key == "backspace" {
					m.connectState.HandleKey("backspace")
				} else if len(key) == 1 {
					m.connectState.HandleKey(key)
				}
				m.updateLayout()
				return m, nil
			}
		}
	}

	return m, nil
}

func (m *Model) handleCommand(value string) []tea.Cmd {
	var cmds []tea.Cmd

	if value == "/models" || value == "/model" {
		m.output = append(m.output, "\033[1;35mFetching models from Ollama...\033[0m")
		m.textinput.SetValue("")
		m.mode = ModeNormal
		m.updateLayout()
		cmds = append(cmds, m.fetchModels())
		return cmds
	}

	if value == "/clear" {
		m.history = nil
		m.output = nil
		m.textinput.SetValue("")
		m.mode = ModeNormal
		m.updateLayout()
		return nil
	}

	if value == "/quit" || value == "/exit" {
		AutoSaveSession(m.session)
		cmds = append(cmds, tea.Quit)
		return cmds
	}

	if value == "/help" {
		m.output = append(m.output, m.renderHelp())
		m.textinput.SetValue("")
		m.mode = ModeNormal
		m.updateLayout()
		return nil
	}

	if value == "/connect" {
		m.connectState = NewConnectState()
		m.mode = ModeConnect
		m.textinput.SetValue("")
		m.updateLayout()
		return nil
	}

	if value == "/compact" {
		result, err := m.compressor.CompressWithModel(m.history)
		if err != nil {
			m.output = append(m.output, fmt.Sprintf("\033[1;31mError:\033[0m %v", err))
		} else if result.Compressed {
			m.history = result.Messages
			m.output = append(m.output, FormatCompactResult(result))
		} else {
			m.output = append(m.output, "History is already compact.")
		}
		m.textinput.SetValue("")
		m.updateLayout()
		return nil
	}

	if value == "/sessions" {
		sessions, err := ListSessions()
		if err != nil {
			m.output = append(m.output, fmt.Sprintf("\033[1;31mError:\033[0m %v", err))
		} else {
			m.sessions = sessions
			m.mode = ModeSessionPicker
			m.sessionCursor = 0
		}
		m.textinput.SetValue("")
		m.updateLayout()
		return nil
	}

	if value == "/skills" {
		m.skills = LoadSkills()
		m.mode = ModeSkillPicker
		m.skillCursor = 0
		m.textinput.SetValue("")
		m.updateLayout()
		return nil
	}

	if value == "/git" {
		m.gitInfo = GetGitInfo()
		if m.gitInfo != nil && m.gitInfo.HasChanges {
			m.output = append(m.output, RenderGitStatus(m.gitInfo))
		} else {
			m.output = append(m.output, "No changes detected.")
		}
		m.textinput.SetValue("")
		m.updateLayout()
		return nil
	}

	if value == "/commit" {
		m.gitInfo = GetGitInfo()
		if m.gitInfo == nil || len(m.gitInfo.Staged) == 0 {
			m.output = append(m.output, "No staged changes. Use: git add <files>")
			m.textinput.SetValue("")
			m.updateLayout()
			return nil
		}
		cmds = append(cmds, m.generateCommitMessage())
		m.textinput.SetValue("")
		m.updateLayout()
		return cmds
	}

	if strings.HasPrefix(value, "$") {
		skillName := strings.TrimPrefix(value, "$")
		skillName = strings.TrimSpace(skillName)
		if skill := GetSkillByName(skillName); skill != nil {
			m.output = append(m.output, fmt.Sprintf("\033[1;33mSkill loaded:\033[0m %s\n\n%s", skill.Name, skill.Content))
		} else {
			m.output = append(m.output, fmt.Sprintf("\033[1;31mSkill not found:\033[0m %s", skillName))
		}
		m.textinput.SetValue("")
		m.updateLayout()
		return nil
	}

	if value == "/remember" {
		m.output = append(m.output, "Usage: /remember <insight to save>")
		m.textinput.SetValue("")
		m.updateLayout()
		return nil
	}

	if strings.HasPrefix(value, "/remember ") {
		insight := strings.TrimPrefix(value, "/remember ")
		if m.projectCtx != nil {
			err := m.projectCtx.AddMemoryEntry(insight)
			if err != nil {
				m.output = append(m.output, fmt.Sprintf("\033[1;31mError:\033[0m %v", err))
			} else {
				m.output = append(m.output, fmt.Sprintf("\033[1;32mRemembered:\033[0m %s", insight))
			}
		} else {
			m.output = append(m.output, "No project context found. Create NEXUS.md first.")
		}
		m.textinput.SetValue("")
		m.updateLayout()
		return nil
	}

	if strings.HasPrefix(value, "/search ") {
		query := strings.TrimPrefix(value, "/search ")
		query = strings.TrimSpace(query)
		if query == "" {
			m.output = append(m.output, "Usage: /search <query>")
		} else {
			results, err := SearchFiles(query, 20)
			if err != nil {
				m.output = append(m.output, fmt.Sprintf("\033[1;31mError:\033[0m %v", err))
			} else {
				m.output = append(m.output, FormatSearchResults(results))
			}
		}
		m.textinput.SetValue("")
		m.updateLayout()
		return nil
	}

	if value == "/autofix" || value == "/fix" {
		m.output = append(m.output, "\033[1;35mRunning auto-fix checks...\033[0m")
		m.textinput.SetValue("")
		m.updateLayout()
		cmds = append(cmds, m.runAutoFix())
		return cmds
	}

	if value == "/autofix on" {
		m.autoFix = true
		m.output = append(m.output, "\033[1;32mAuto-fix enabled:\033[0m Will run tests after edits")
		m.textinput.SetValue("")
		m.updateLayout()
		return nil
	}

	if value == "/autofix off" {
		m.autoFix = false
		m.output = append(m.output, "\033[1;33mAuto-fix disabled\033[0m")
		m.textinput.SetValue("")
		m.updateLayout()
		return nil
	}

	if value == "/mcp" {
		m.mcpServers = LoadMCPServers()
		if len(m.mcpServers) == 0 {
			m.output = append(m.output, "No MCP servers configured.\nCreate ~/.config/nexus/mcp.json")
		} else {
			var lines []string
			lines = append(lines, "\033[1;35mMCP Servers:\033[0m")
			for _, s := range m.mcpServers {
				lines = append(lines, fmt.Sprintf("  • %s", s.Name))
			}
			m.output = append(m.output, strings.Join(lines, "\n"))
		}
		m.textinput.SetValue("")
		m.updateLayout()
		return nil
	}

	if strings.HasPrefix(value, "/mcp call ") {
		parts := strings.SplitN(strings.TrimPrefix(value, "/mcp call "), " ", 2)
		if len(parts) < 2 {
			m.output = append(m.output, "Usage: /mcp call <server> <tool> [args]")
		} else {
			serverName, toolName := parts[0], parts[1]
			var args map[string]interface{}
			if len(parts) > 2 {
				json.Unmarshal([]byte(parts[2]), &args)
			}
			cmds = append(cmds, m.callMCPTool(serverName, toolName, args))
		}
		m.textinput.SetValue("")
		m.updateLayout()
		return cmds
	}

	if value == "/perm" || value == "/permissions" {
		m.output = append(m.output, m.permissions.GetPermissionSummary())
		m.textinput.SetValue("")
		m.updateLayout()
		return nil
	}

	if strings.HasPrefix(value, "/perm grant ") {
		parts := strings.Fields(strings.TrimPrefix(value, "/perm grant "))
		if len(parts) < 2 {
			m.output = append(m.output, "Usage: /perm grant <path> <read|write|exec|admin> [recursive]")
		} else {
			path := parts[0]
			level := m.permissions.ParseLevel(parts[1])
			recursive := len(parts) > 2 && parts[2] == "recursive"
			m.permissions.GrantPermission(path, level, recursive)
			m.permissions.SavePermissions()
			m.output = append(m.output, fmt.Sprintf("\033[1;32mGranted:\033[0m %s [%s]", path, m.permissions.levelName(level)))
		}
		m.textinput.SetValue("")
		m.updateLayout()
		return nil
	}

	if strings.HasPrefix(value, "/perm revoke ") {
		path := strings.TrimPrefix(value, "/perm revoke ")
		m.permissions.RevokePermission(path)
		m.permissions.SavePermissions()
		m.output = append(m.output, fmt.Sprintf("\033[1;33mRevoked:\033[0m %s", path))
		m.textinput.SetValue("")
		m.updateLayout()
		return nil
	}

	if value == "/context" || value == "/ctx" {
		m.output = append(m.output, m.context.GetProjectStructure())
		m.textinput.SetValue("")
		m.updateLayout()
		return nil
	}

	if value == "/index" {
		m.output = append(m.output, "\033[1;35mIndexing codebase...\033[0m")
		m.textinput.SetValue("")
		m.updateLayout()
		cmds = append(cmds, func() tea.Msg {
			err := m.context.BuildIndex()
			if err != nil {
				return StreamDoneMsg{Error: err}
			}
			return StreamDoneMsg{FullResponse: fmt.Sprintf("Indexed %d files", len(m.context.fileIndex))}
		})
		return cmds
	}

	if value == "/tools" {
		tools := m.tools.List()
		var lines []string
		lines = append(lines, "\033[1;35mAvailable Tools:\033[0m")
		for _, t := range tools {
			lines = append(lines, fmt.Sprintf("  • %s: %s", t.Name, t.Description))
		}
		m.output = append(m.output, strings.Join(lines, "\n"))
		m.textinput.SetValue("")
		m.updateLayout()
		return nil
	}

	if value == "/pool" {
		m.output = append(m.output, m.failover.GetStatus())
		m.textinput.SetValue("")
		m.updateLayout()
		return nil
	}

	if value == "/tokens" {
		tokens, maxTokens, percent := m.compressor.GetTokenUsage(m.history)
		status := fmt.Sprintf("\033[1;35mToken Usage:\033[0m\n\n")
		status += fmt.Sprintf("Used: %d / %d tokens (%d%%)\n", tokens, maxTokens, percent)
		status += fmt.Sprintf("Auto compact: %v\n", m.cfg.GetContextConfig().AutoCompact)
		status += fmt.Sprintf("Compact at: %d%%\n", m.cfg.GetContextConfig().CompactPercent)
		m.output = append(m.output, status)
		m.textinput.SetValue("")
		m.updateLayout()
		return nil
	}

	if strings.HasPrefix(value, "/url ") || strings.HasPrefix(value, "/fetch ") {
		var url string
		if strings.HasPrefix(value, "/url ") {
			url = strings.TrimPrefix(value, "/url ")
		} else {
			url = strings.TrimPrefix(value, "/fetch ")
		}
		url = strings.TrimSpace(url)
		if url == "" {
			m.output = append(m.output, "Usage: /url <url> or /fetch <url>")
		} else {
			m.output = append(m.output, fmt.Sprintf("\033[1;35mFetching:\033[0m %s", url))
			cmds = append(cmds, func() tea.Msg {
				tool := &FetchURLTool{}
				result, err := tool.Execute(map[string]interface{}{"url": url})
				if err != nil {
					return StreamDoneMsg{Error: err}
				}
				return StreamDoneMsg{FullResponse: result}
			})
		}
		m.textinput.SetValue("")
		m.updateLayout()
		return cmds
	}

	if value == "/vim" {
		m.vimMode = !m.vimMode
		state := "disabled"
		if m.vimMode {
			state = "enabled"
		}
		m.output = append(m.output, fmt.Sprintf("\033[1;33mVim mode:\033[0m %s", state))
		m.textinput.SetValue("")
		m.updateLayout()
		return nil
	}

	if value == "/agent" || value == "/agent status" {
		if m.agentLoop.IsEnabled() {
			m.output = append(m.output, "\033[1;32mAgent loop: ENABLED\033[0m\n\nUse /agent <goal> to start a multi-step task")
		} else {
			m.output = append(m.output, "\033[1;31mAgent loop: DISABLED\033[0m\n\nUse /agent on to enable")
		}
		m.textinput.SetValue("")
		m.updateLayout()
		return nil
	}

	if value == "/agent on" {
		m.agentLoop.Toggle()
		m.output = append(m.output, "\033[1;32mAgent loop enabled\033[0m")
		m.textinput.SetValue("")
		m.updateLayout()
		return nil
	}

	if value == "/agent off" {
		m.agentLoop.Toggle()
		m.output = append(m.output, "\033[1;33mAgent loop disabled\033[0m")
		m.textinput.SetValue("")
		m.updateLayout()
		return nil
	}

	if strings.HasPrefix(value, "/agent ") {
		goal := strings.TrimPrefix(value, "/agent ")
		loop := m.agentLoop.StartLoop(goal)
		m.output = append(m.output, fmt.Sprintf("\033[1;35mAgent started:\033[0m %s", goal))
		m.textinput.SetValue("")
		m.updateLayout()
		cmds = append(cmds, m.runAgentLoop(loop))
		return cmds
	}

	if value == "/think" || value == "/think on" {
		m.thinking.Toggle()
		state := "disabled"
		if m.thinking.Enabled {
			state = "enabled"
		}
		m.output = append(m.output, fmt.Sprintf("\033[1;33mThinking display:\033[0m %s", state))
		m.textinput.SetValue("")
		m.updateLayout()
		return nil
	}

	if value == "/think off" {
		m.thinking.Enabled = false
		m.output = append(m.output, "\033[1;33mThinking display disabled\033[0m")
		m.textinput.SetValue("")
		m.updateLayout()
		return nil
	}

	if value == "/plugins" {
		m.output = append(m.output, m.plugins.ListPlugins())
		m.textinput.SetValue("")
		m.updateLayout()
		return nil
	}

	if value == "/vault" {
		m.output = append(m.output, m.vault.RenderStatus()+"\n\n"+m.vault.RenderList())
		m.textinput.SetValue("")
		m.updateLayout()
		return nil
	}

	if strings.HasPrefix(value, "/vault unlock ") {
		key := strings.TrimPrefix(value, "/vault unlock ")
		m.vault.SetMasterKey(key)
		if err := m.vault.Load(); err != nil {
			m.output = append(m.output, fmt.Sprintf("\033[1;31mError:\033[0m %v", err))
		} else {
			m.output = append(m.output, "\033[1;32mVault unlocked\033[0m")
		}
		m.textinput.SetValue("")
		m.updateLayout()
		return nil
	}

	if strings.HasPrefix(value, "/vault set ") {
		parts := strings.SplitN(strings.TrimPrefix(value, "/vault set "), " ", 2)
		if len(parts) < 2 {
			m.output = append(m.output, "Usage: /vault set <name> <value>")
		} else {
			if err := m.vault.Set(parts[0], parts[1]); err != nil {
				m.output = append(m.output, fmt.Sprintf("\033[1;31mError:\033[0m %v", err))
			} else {
				m.output = append(m.output, fmt.Sprintf("\033[1;32mSaved:\033[0m %s", parts[0]))
			}
		}
		m.textinput.SetValue("")
		m.updateLayout()
		return nil
	}

	if value == "/costs" || value == "/usage" {
		m.output = append(m.output, m.costs.GetSessionSummary())
		m.textinput.SetValue("")
		m.updateLayout()
		return nil
	}

	if value == "/prompts" {
		m.output = append(m.output, m.prompts.RenderList())
		m.textinput.SetValue("")
		m.updateLayout()
		return nil
	}

	if strings.HasPrefix(value, "/prompt save ") {
		parts := strings.SplitN(strings.TrimPrefix(value, "/prompt save "), " ", 2)
		if len(parts) < 2 {
			m.output = append(m.output, "Usage: /prompt save <name> <content>")
		} else {
			if err := m.prompts.Add(parts[0], parts[1], "", nil); err != nil {
				m.output = append(m.output, fmt.Sprintf("\033[1;31mError:\033[0m %v", err))
			} else {
				m.output = append(m.output, fmt.Sprintf("\033[1;32mSaved prompt:\033[0m %s", parts[0]))
			}
		}
		m.textinput.SetValue("")
		m.updateLayout()
		return nil
	}

	if strings.HasPrefix(value, "/prompt use ") {
		name := strings.TrimPrefix(value, "/prompt use ")
		if content, ok := m.prompts.Get(name); ok {
			m.textinput.SetValue(content)
			m.output = append(m.output, fmt.Sprintf("\033[1;33mLoaded prompt:\033[0m %s", name))
		} else {
			m.output = append(m.output, fmt.Sprintf("\033[1;31mPrompt not found:\033[0m %s", name))
		}
		m.updateLayout()
		return nil
	}

	if value == "/watch" {
		projectDir, _ := os.Getwd()
		m.watcher = NewWatcher(WatchConfig{
			Paths:     []string{projectDir},
			Extensions: []string{".go", ".py", ".js", ".ts", ".rs", ".yaml", ".json"},
			Debounce:  1 * time.Second,
			OnChange: func(c FileChange) {
				m.output = append(m.output, fmt.Sprintf("\033[1;33mFile changed:\033[0m %s (%s)", c.Path, c.Op))
				m.updateLayout()
			},
		})
		m.watcher.Start()
		m.output = append(m.output, "\033[1;32mWatch started\033[0m")
		m.textinput.SetValue("")
		m.updateLayout()
		return nil
	}

	if value == "/watch stop" {
		if m.watcher != nil {
			m.watcher.Stop()
			m.output = append(m.output, "\033[1;33mWatch stopped\033[0m")
		}
		m.textinput.SetValue("")
		m.updateLayout()
		return nil
	}

	if value == "/diff" {
		m.output = append(m.output, "Paste a diff or use /diff <file> to see changes")
		m.textinput.SetValue("")
		m.updateLayout()
		return nil
	}

	if strings.HasPrefix(value, "/diff ") {
		filePath := strings.TrimPrefix(value, "/diff ")
		cmd := exec.Command("git", "diff", filePath)
		output, err := cmd.CombinedOutput()
		if err != nil {
			m.output = append(m.output, fmt.Sprintf("\033[1;31mError:\033[0m %v", err))
		} else {
			m.diff = NewDiffViewer(m.width, m.height)
			m.diff.ParseDiff(string(output))
			m.output = append(m.output, m.diff.Render())
		}
		m.textinput.SetValue("")
		m.updateLayout()
		return nil
	}

	if strings.HasPrefix(value, "@") {
		resolved := ResolveFileRefs(value)
		if resolved != value {
			m.output = append(m.output, fmt.Sprintf("\033[1;36mYou:\033[0m %s", resolved))
		}
		m.textinput.SetValue("")
		m.updateLayout()
		cmds = append(cmds, m.sendMessage(resolved))
		return cmds
	}

	cmds = append(cmds, m.sendMessage(value))
	return cmds
}

func (m *Model) sendMessage(value string) tea.Cmd {
	m.history = append(m.history, ChatMessage{Role: "user", Content: value})
	m.session.Messages = append(m.session.Messages, ChatMessage{Role: "user", Content: value})
	m.output = append(m.output, fmt.Sprintf("\033[1;36mYou:\033[0m %s", value))
	m.textinput.SetValue("")
	m.streaming = true
	m.streamBuf = ""
	m.mode = ModeNormal

	if m.compressor.ShouldCompact(m.history) {
		m.output = append(m.output, "\033[1;33m📦 Compressing context...\033[0m")
		m.updateLayout()
		return m.compressAndSend()
	}

	m.updateLayout()
	return m.streamChat()
}

func (m *Model) compressAndSend() tea.Cmd {
	return func() tea.Msg {
		result, err := m.compressor.CompressWithModel(m.history)
		if err != nil {
			return NotificationMsg{Type: "error", Message: fmt.Sprintf("Compression failed: %v", err)}
		}
		if result.Compressed {
			m.history = result.Messages
			return CompressionNotificationMsg{
				Event: CompressionEvent{
					Type:       "compress",
					TokensUsed: m.compressor.EstimateTokens(m.history),
					TokensMax:  m.cfg.GetContextConfig().MaxTokens,
					Percent:    m.compressor.EstimateTokens(result.Messages) * 100 / m.cfg.GetContextConfig().MaxTokens,
					Summary:    result.Summary,
				},
			}
		}
		return NotificationMsg{Type: "info", Message: "Context already compact"}
	}
}

func (m *Model) runAgentLoop(loop *AgentLoop) tea.Cmd {
	return func() tea.Msg {
		for !loop.IsComplete() {
			prompt := loop.GetPrompt()

			var messages []client.Message
			messages = append(messages, client.Message{Role: "user", Content: prompt})

			var fullResponse strings.Builder

			err := m.multiClient.StreamChat(messages, m.modelName, m.cfg.Temperature, func(token string) error {
				fullResponse.WriteString(token)
				return nil
			})

			if err != nil {
				return StreamDoneMsg{Error: fmt.Errorf("agent error: %w", err)}
			}

			response := fullResponse.String()

			step, ok := loop.ParseStep(response)
			if !ok {
				step = &AgentStep{
					Phase:   PhaseThink,
					Thought: response,
					Success: true,
				}
			}

			if step.Phase == PhaseAct && step.Action != "" {
				parts := strings.SplitN(step.Action, ":", 2)
				if len(parts) == 2 {
					toolName := strings.TrimSpace(parts[0])
					argsStr := strings.TrimSpace(parts[1])

					var args map[string]interface{}
					json.Unmarshal([]byte(argsStr), &args)

					if tool, ok := m.tools.Get(toolName); ok {
						result, err := tool.Execute(args)
						step.Result = result
						step.Success = err == nil
						if err != nil {
							step.Result = err.Error()
						}
					}
				}
			} else {
				step.Result = "Phase completed"
				step.Success = true
			}

			loop.AddStep(*step)

			if m.agentLoop.IsEnabled() {
				phaseColor := phaseColors[step.Phase]
				m.output = append(m.output, fmt.Sprintf("\033[1;%sm[%s]\033[0m %s",
					phaseColor, phaseNames[step.Phase], step.Thought))
				m.updateLayout()
			}
		}

		return StreamDoneMsg{FullResponse: loop.GetSummary()}
	}
}

func (m *Model) runAutoFix() tea.Cmd {
	return func() tea.Msg {
		results := RunAllChecks()
		m.output = append(m.output, FormatAutoFixResults(results))

		if m.autoFix && m.fixLoop < 3 {
			failedOutput := GetFailedOutput(results)
			if failedOutput != "" {
				m.fixLoop++
				fixPrompt := fmt.Sprintf("The following tests/lints failed. Please fix the issues:\n\n%s\n\nProvide the corrected code.", failedOutput)
				m.history = append(m.history, ChatMessage{Role: "user", Content: fixPrompt})
				m.session.Messages = append(m.session.Messages, ChatMessage{Role: "user", Content: fixPrompt})
			}
		}

		return StreamDoneMsg{FullResponse: ""}
	}
}

func (m *Model) callMCPTool(serverName, toolName string, args map[string]interface{}) tea.Cmd {
	return func() tea.Msg {
		var server *MCPServer
		for _, s := range m.mcpServers {
			if s.Name == serverName {
				server = s
				break
			}
		}

		if server == nil {
			return StreamDoneMsg{Error: fmt.Errorf("mcp server not found: %s", serverName)}
		}

		if err := server.Connect(); err != nil {
			return StreamDoneMsg{Error: fmt.Errorf("connect to %s: %w", serverName, err)}
		}
		defer server.Close()

		result, err := server.CallTool(MCPToolCall{
			Name:      toolName,
			Arguments: args,
		})

		if err != nil {
			return StreamDoneMsg{Error: err}
		}

		var output strings.Builder
		for _, content := range result.Content {
			output.WriteString(content.Text)
		}

		return StreamDoneMsg{FullResponse: output.String()}
	}
}

func (m *Model) generateCommitMessage() tea.Cmd {
	return func() tea.Msg {
		diff, err := GetGitDiff(true)
		if err != nil {
			return StreamDoneMsg{Error: err}
		}

		if len(diff) > 5000 {
			diff = diff[:5000] + "\n\n... (truncated)"
		}

		prompt := fmt.Sprintf("Generate a concise conventional commit message for these changes:\n\n```diff\n%s\n```", diff)

		var messages []client.Message
		messages = append(messages, client.Message{Role: "user", Content: prompt})

		var fullResponse strings.Builder

		err = m.multiClient.StreamChat(messages, m.modelName, m.cfg.Temperature, func(token string) error {
			fullResponse.WriteString(token)
			return nil
		})

		if err != nil {
			return StreamDoneMsg{Error: err}
		}

		return StreamDoneMsg{FullResponse: fullResponse.String()}
	}
}

func (m *Model) fetchModels() tea.Cmd {
	return func() tea.Msg {
		models, err := m.multiClient.ListModels()
		if err != nil {
			return StreamDoneMsg{Error: fmt.Errorf("fetch models: %w", err)}
		}

		if len(models) == 0 {
			return StreamDoneMsg{FullResponse: "No models found in Ollama. Pull a model with: ollama pull <model>"}
		}

		m.models = models
		m.modelPicker = true
		m.modelCursor = 0

		var output strings.Builder
		output.WriteString(fmt.Sprintf("\033[1;35mAvailable models:\033[0m %d\n\n", len(models)))
		for _, mdl := range models {
			if mdl == m.modelName {
				output.WriteString(fmt.Sprintf("  \033[1;32m▸ %s (current)\033[0m\n", mdl))
			} else {
				output.WriteString(fmt.Sprintf("    %s\n", mdl))
			}
		}
		output.WriteString("\n\033[1;33m↑↓ navigate • Enter select • Esc cancel\033[0m")

		return StreamDoneMsg{FullResponse: output.String()}
	}
}

func (m *Model) updateModelPicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			m.modelPicker = false
			m.updateLayout()
			return m, nil
		case tea.KeyUp:
			if m.modelCursor > 0 {
				m.modelCursor--
			}
			m.updateLayout()
			return m, nil
		case tea.KeyDown:
			if m.modelCursor < len(m.models)-1 {
				m.modelCursor++
			}
			m.updateLayout()
			return m, nil
		case tea.KeyEnter:
			if len(m.models) > 0 {
				m.modelName = m.models[m.modelCursor]
				m.session.Model = m.modelName
				m.output = append(m.output, fmt.Sprintf("\033[1;33mModel changed to:\033[0m %s", m.modelName))
			}
			m.modelPicker = false
			m.updateLayout()
			return m, nil
		}
	}
	return m, nil
}

func (m *Model) updateSessionPicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			m.mode = ModeNormal
			m.updateLayout()
			return m, nil
		case tea.KeyUp:
			if m.sessionCursor > 0 {
				m.sessionCursor--
			}
			m.updateLayout()
			return m, nil
		case tea.KeyDown:
			if m.sessionCursor < len(m.sessions)-1 {
				m.sessionCursor++
			}
			m.updateLayout()
			return m, nil
		case tea.KeyEnter:
			if len(m.sessions) > 0 {
				selected := m.sessions[m.sessionCursor]
				session, err := LoadSession(selected.ID)
				if err == nil {
					m.session = session
					m.history = session.Messages
					m.output = nil
					for _, h := range m.history {
						if h.Role == "user" {
							m.output = append(m.output, fmt.Sprintf("\033[1;36mYou:\033[0m %s", h.Content))
						} else if h.Role == "assistant" {
							rendered := m.renderMarkdown(h.Content)
							m.output = append(m.output, fmt.Sprintf("\033[1;32mNexus:\033[0m\n%s", rendered))
						}
					}
					m.output = append(m.output, fmt.Sprintf("\033[1;33mSession loaded:\033[0m %d messages", len(m.history)))
				}
			}
			m.mode = ModeNormal
			m.updateLayout()
			return m, nil
		case tea.KeyDelete:
			if len(m.sessions) > 0 {
				selected := m.sessions[m.sessionCursor]
				DeleteSession(selected.ID)
				m.sessions = append(m.sessions[:m.sessionCursor], m.sessions[m.sessionCursor+1:]...)
				if m.sessionCursor >= len(m.sessions) && len(m.sessions) > 0 {
					m.sessionCursor = len(m.sessions) - 1
				}
			}
			m.updateLayout()
			return m, nil
		}
	}
	return m, nil
}

func (m *Model) updateSkillPicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			m.mode = ModeNormal
			m.updateLayout()
			return m, nil
		case tea.KeyUp:
			if m.skillCursor > 0 {
				m.skillCursor--
			}
			m.updateLayout()
			return m, nil
		case tea.KeyDown:
			if m.skillCursor < len(m.skills)-1 {
				m.skillCursor++
			}
			m.updateLayout()
			return m, nil
		case tea.KeyEnter:
			if len(m.skills) > 0 {
				skill := m.skills[m.skillCursor]
				m.output = append(m.output, fmt.Sprintf("\033[1;33mSkill loaded:\033[0m %s\n\n%s", skill.Name, skill.Content))
			}
			m.mode = ModeNormal
			m.updateLayout()
			return m, nil
		}
	}
	return m, nil
}

func (m Model) showHints() bool {
	return m.mode == ModeCommandHint
}

func (m *Model) updateLayout() {
	headerHeight := 1
	inputHeight := 2
	statusHeight := 1
	separatorHeight := 1

	fixedHeight := headerHeight + separatorHeight + inputHeight + statusHeight
	viewportHeight := m.height - fixedHeight
	if viewportHeight < 1 {
		viewportHeight = 1
	}

	m.viewport.Width = m.width
	m.viewport.Height = viewportHeight
	m.textinput.Width = m.width - 4

	m.viewport.SetContent(m.renderOutput())
	m.viewport.GotoBottom()
}

func (m Model) View() string {
	if !m.ready {
		return "  Initializing..."
	}

	header := m.renderHeader()
	status := m.renderStatus()
	input := m.renderInput()
	separator := lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Render(strings.Repeat("─", m.width))

	var bottom strings.Builder
	bottom.WriteString(separator)
	bottom.WriteString("\n")
	bottom.WriteString(input)

	if m.showHints() && len(m.filteredCmds) > 0 {
		bottom.WriteString("\n")
		bottom.WriteString(m.renderHints())
	}

	if m.mode == ModeApproval {
		bottom.WriteString("\n")
		bottom.WriteString(m.renderApproval())
	}

	bottom.WriteString("\n")
	bottom.WriteString(status)

	if m.mode == ModeConnect && m.connectState != nil {
		connectView := m.connectState.Render()
		return lipgloss.JoinVertical(
			lipgloss.Left,
			header,
			connectView,
			bottom.String(),
		)
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		m.viewport.View(),
		bottom.String(),
	)
}

func (m Model) renderHeader() string {
	title := lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Bold(true).Render("nex")
	agent := RenderAgentBadge(m.agent)
	model := lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true).Render(fmt.Sprintf("%s/%s", m.provider, m.modelName))

	permMode := lipgloss.NewStyle().Foreground(lipgloss.Color(permModeColors[m.permMode])).Render(permModeNames[m.permMode])

	var parts []string
	parts = append(parts, title)
	parts = append(parts, agent)
	parts = append(parts, model)
	parts = append(parts, permMode)

	joined := strings.Join(parts, " ")

	sepWidth := m.width - lipgloss.Width(joined) - 2
	if sepWidth < 0 {
		sepWidth = 0
	}
	separator := strings.Repeat("─", sepWidth)

	gitPart := ""
	if m.gitInfo != nil && m.gitInfo.Branch != "" {
		gitPart = " " + lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("("+m.gitInfo.Branch+")")
	}

	tokens := ""
	if m.compressor != nil {
		t, max, _ := m.compressor.GetTokenUsage(m.history)
		tokens = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(fmt.Sprintf("[%d/%d]", t, max))
	}

	return fmt.Sprintf("%s %s%s %s", joined, separator, gitPart, tokens)
}

func (m Model) renderInput() string {
	prompt := lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true).Render("❯ ")
	return prompt + m.textinput.View()
}

func (m Model) renderStatus() string {
	permColor := permModeColors[m.permMode]
	permLabel := lipgloss.NewStyle().
		Foreground(lipgloss.Color(permColor)).
		Bold(true).
		Render(permModeNames[m.permMode])

	left := fmt.Sprintf("Tab:%s • ↑↓ scroll • Enter send", permLabel)

	if m.streaming {
		left = streamingSt.Render("● streaming...")
	}

	if len(m.queue) > 0 {
		left += fmt.Sprintf(" • %d queued", len(m.queue))
	}

	right := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(fmt.Sprintf("%d msgs", len(m.history)))
	padding := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if padding < 0 {
		padding = 0
	}

	return left + strings.Repeat(" ", padding) + right
}

func (m Model) renderOutput() string {
	if len(m.output) == 0 {
		return m.renderWelcome()
	}

	var out strings.Builder
	out.WriteString(strings.Join(m.output, "\n\n"))

	if m.modelPicker {
		out.WriteString("\n\n")
		out.WriteString(m.renderModelPicker())
	}

	if m.mode == ModeSessionPicker {
		out.WriteString("\n\n")
		out.WriteString(m.renderSessionPicker())
	}

	if m.mode == ModeSkillPicker {
		out.WriteString("\n\n")
		out.WriteString(m.renderSkillPicker())
	}

	return out.String()
}

func (m Model) renderWelcome() string {
	cwd, _ := os.Getwd()
	if homeDir, _ := os.UserHomeDir(); len(cwd) >= len(homeDir) && cwd[:len(homeDir)] == homeDir {
		cwd = "~" + cwd[len(homeDir):]
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("33")).
		Padding(1, 2).
		Width(55)

	titleS := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	labelS := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	valueS := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252"))
	tipS := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Italic(true).PaddingTop(1)

	header := titleS.Render(" >_ Nexus")
	modelInfo := fmt.Sprintf("%-10s %s %s",
		labelS.Render("model:"),
		valueS.Render(m.modelName),
		labelS.Render("/models to change"),
	)
	dirInfo := fmt.Sprintf("%-10s %s",
		labelS.Render("directory:"),
		valueS.Render(cwd),
	)
	agentInfo := fmt.Sprintf("%-10s %s",
		labelS.Render("agent:"),
		valueS.Render(m.agent.String()),
	)

	content := fmt.Sprintf("%s\n\n%s\n%s\n%s", header, modelInfo, dirInfo, agentInfo)
	boxed := boxStyle.Render(content)

	var tips []string
	tips = append(tips, "Shift+Tab: switch agent • Tab: cycle permissions")

	if !HasValidModels(m.cfg) {
		tips = []string{
			"\033[1;92m⚡ Quick start: type /connect to set up a provider\033[0m",
			"Supports: OpenAI, Anthropic, Groq, DeepSeek, Ollama, LM Studio",
		}
	}

	tip := tipS.Render(strings.Join(tips, "\n "))

	return boxed + "\n" + tip
}

func (m Model) renderApproval() string {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("3")).
		Padding(0, 1).
		Render(fmt.Sprintf("Approve: %s? (y/n)", m.pendingApproval))
}

func (m Model) renderModelPicker() string {
	var lines []string
	lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true).Render("  Select model:"))
	lines = append(lines, "")

	for i, mdl := range m.models {
		if i == m.modelCursor {
			lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true).Background(lipgloss.Color("236")).Padding(0, 1).Render("▸ "+mdl))
		} else {
			lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("7")).Padding(0, 1).Render("  "+mdl))
		}
	}

	lines = append(lines, "")
	lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true).Render("  Enter to select • Esc to cancel"))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("33")).
		Padding(1, 2).
		Render(strings.Join(lines, "\n"))
}

func (m Model) renderSessionPicker() string {
	var lines []string
	lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true).Render("  Select session:"))
	lines = append(lines, "")

	if len(m.sessions) == 0 {
		lines = append(lines, "  No saved sessions")
	} else {
		for i, s := range m.sessions {
			name := s.Name
			if name == "" {
				name = fmt.Sprintf("Session %s", s.ID[:8])
			}
			msgInfo := fmt.Sprintf("%d msgs", s.MsgCount)
			timeInfo := s.UpdatedAt.Format("Jan 02 15:04")

			if i == m.sessionCursor {
				lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true).Background(lipgloss.Color("236")).Padding(0, 1).Render(fmt.Sprintf("▸ %s (%s, %s)", name, msgInfo, timeInfo)))
			} else {
				lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("7")).Padding(0, 1).Render(fmt.Sprintf("  %s (%s, %s)", name, msgInfo, timeInfo)))
			}
		}
	}

	lines = append(lines, "")
	lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true).Render("  Enter to load • Del to delete • Esc to cancel"))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("33")).
		Padding(1, 2).
		Render(strings.Join(lines, "\n"))
}

func (m Model) renderSkillPicker() string {
	var lines []string
	lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true).Render("  Select skill:"))
	lines = append(lines, "")

	if len(m.skills) == 0 {
		lines = append(lines, "  No skills found")
		lines = append(lines, "  Create in ~/.config/nexus/skills/")
	} else {
		for i, s := range m.skills {
			desc := s.Description
			if len(desc) > 40 {
				desc = desc[:37] + "..."
			}

			if i == m.skillCursor {
				lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true).Background(lipgloss.Color("236")).Padding(0, 1).Render(fmt.Sprintf("▸ $%s", s.Name)))
				if desc != "" {
					lines = append(lines, "    "+lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Render(desc))
				}
			} else {
				lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("7")).Padding(0, 1).Render(fmt.Sprintf("  $%s", s.Name)))
				if desc != "" {
					lines = append(lines, "    "+lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(desc))
				}
			}
		}
	}

	lines = append(lines, "")
	lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true).Render("  Enter to load • Esc to cancel"))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("33")).
		Padding(1, 2).
		Render(strings.Join(lines, "\n"))
}

func (m Model) renderHints() string {
	commands := getCommands()
	var lines []string

	for j, idx := range m.filteredCmds {
		c := commands[idx]
		if j == m.hintCursor {
			lines = append(lines, fmt.Sprintf("  %s  %s",
				lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true).Background(lipgloss.Color("236")).Padding(0, 1).Render("▸ "+c.cmd),
				lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Background(lipgloss.Color("236")).Render(c.desc),
			))
		} else {
			lines = append(lines, fmt.Sprintf("  %-12s %s",
				lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true).Render(c.cmd),
				lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(c.desc),
			))
		}
	}

	content := strings.Join(lines, "\n")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(0, 1).
		Render(content)
}

func (m Model) renderHelp() string {
	cmdStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	var lines []string
	lines = append(lines, "\033[1;35mCommands:\033[0m")
	lines = append(lines, "")

	for _, c := range getCommands() {
		lines = append(lines, fmt.Sprintf("  %-16s %s", cmdStyle.Render(c.cmd), descStyle.Render(c.desc)))
	}

	lines = append(lines, "")
	lines = append(lines, "\033[1;35mAgent Modes:\033[0m")
	lines = append(lines, "")
	lines = append(lines, "  Shift+Tab   Toggle build/plan agent")
	lines = append(lines, "  build       Full access for development")
	lines = append(lines, "  plan        Read-only for analysis")
	lines = append(lines, "")
	lines = append(lines, "\033[1;35mFeatures:\033[0m")
	lines = append(lines, "")
	lines = append(lines, "  @file       Reference file in context")
	lines = append(lines, "  /search     Search files with ripgrep")
	lines = append(lines, "  /autofix    Run tests/linters")
	lines = append(lines, "  /mcp        MCP server integration")
	lines = append(lines, "  /perm       Manage permissions")
	lines = append(lines, "  /context    Show project structure")
	lines = append(lines, "  /index      Index codebase")
	lines = append(lines, "  /tools      List available tools")
	lines = append(lines, "  /vim        Toggle vim mode")
	lines = append(lines, "")
	lines = append(lines, "\033[1;35mAgent & AI:\033[0m")
	lines = append(lines, "")
	lines = append(lines, "  /agent      Multi-step agent loop")
	lines = append(lines, "  /think      Show AI reasoning")
	lines = append(lines, "  /costs      Token usage tracking")
	lines = append(lines, "")
	lines = append(lines, "\033[1;35mSecurity & Plugins:\033[0m")
	lines = append(lines, "")
	lines = append(lines, "  /vault      Encrypted credential storage")
	lines = append(lines, "  /plugins    Plugin manager")
	lines = append(lines, "  /prompts    Prompt templates")
	lines = append(lines, "")
	lines = append(lines, "\033[1;35mMonitoring:\033[0m")
	lines = append(lines, "")
	lines = append(lines, "  /watch      File watcher")
	lines = append(lines, "  /diff       Interactive diff viewer")

	return strings.Join(lines, "\n")
}

func (m Model) renderMarkdown(text string) string {
	w := m.width - 6
	if w < 40 {
		w = 40
	}

	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(w),
	)
	if err != nil {
		return text
	}

	out, err := r.Render(text)
	if err != nil {
		return text
	}

	return strings.TrimSuffix(out, "\n")
}

func (m *Model) updateStreamingDisplay() {
	if len(m.output) > 0 {
		m.output = m.output[:len(m.output)-1]
	}

	rendered := m.renderMarkdown(m.streamBuf)
	m.output = append(m.output, fmt.Sprintf("\033[1;32mNexus:\033[0m\n%s▌", rendered))
	m.viewport.SetContent(m.renderOutput())
	m.viewport.GotoBottom()
}

func (m Model) waitForToken() tea.Cmd {
	return func() tea.Msg {
		return StreamDoneMsg{}
	}
}

func (m Model) reviewGit() tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("git", "diff", "--cached")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return StreamDoneMsg{Error: fmt.Errorf("git diff failed: %w", err)}
		}

		diff := string(output)
		if strings.TrimSpace(diff) == "" {
			return StreamDoneMsg{Error: fmt.Errorf("no staged changes found. Use: git add <files>")}
		}

		if len(diff) > 10000 {
			diff = diff[:10000] + "\n\n... (truncated)"
		}

		prompt := fmt.Sprintf("Review the following git diff and suggest improvements or catch bugs:\n\n```diff\n%s\n```", diff)

		var messages []client.Message

		if m.projectCtx != nil && m.projectCtx.Instructions != "" {
			messages = append(messages, client.Message{Role: "system", Content: m.projectCtx.Instructions})
		}

		messages = append(messages, client.Message{Role: "user", Content: prompt})

		var fullResponse strings.Builder

		err = m.multiClient.StreamChat(messages, m.modelName, m.cfg.Temperature, func(token string) error {
			fullResponse.WriteString(token)
			return nil
		})

		if err != nil {
			return StreamDoneMsg{Error: err}
		}

		return StreamDoneMsg{FullResponse: fullResponse.String()}
	}
}

func (m Model) streamChat() tea.Cmd {
	return func() tea.Msg {
		var messages []client.Message

		systemPrompt := "You are Nexus, a helpful AI assistant running in the terminal. You can use tools to help users with their tasks."

		if m.projectCtx != nil {
			if p := m.projectCtx.GetSystemPrompt(); p != "" {
				systemPrompt = p
			}
		}

		if m.tools != nil {
			toolPrompt := m.tools.GetSystemPrompt()
			if toolPrompt != "" {
				systemPrompt += "\n\n" + toolPrompt
			}
		}

		messages = append(messages, client.Message{Role: "system", Content: systemPrompt})

		for _, h := range m.history {
			messages = append(messages, client.Message{Role: h.Role, Content: h.Content})
		}

		var fullResponse strings.Builder

		inputTokens := m.costs.EstimateTokens(systemPrompt)
		for _, h := range m.history {
			inputTokens += m.costs.EstimateTokens(h.Content)
		}

		errStream := func(token string) error {
			processed := m.thinking.ProcessToken(token)
			if processed != "" {
				fullResponse.WriteString(processed)
			}
			return nil
		}

		var err error
		if m.failover != nil && m.cfg.Pool != nil && m.cfg.Pool.Enabled {
			_, err = m.failover.StreamChatWithFailover(messages, m.modelName, m.cfg.Temperature, errStream)
			if err == nil {
				return StreamDoneMsg{FullResponse: fullResponse.String()}
			}
		} else if m.multiClient != nil {
			err = m.multiClient.StreamChat(messages, m.modelName, m.cfg.Temperature, errStream)
		} else if m.client != nil {
			err = m.client.StreamChat(messages, m.modelName, m.cfg.Temperature, errStream)
		} else {
			return StreamDoneMsg{Error: fmt.Errorf("no client configured")}
		}

		if err != nil {
			return StreamDoneMsg{Error: err}
		}

		response := fullResponse.String()
		outputTokens := m.costs.EstimateTokens(response)
		m.costs.Track(m.modelName, inputTokens, outputTokens)

		if m.thinking.Enabled && m.thinking.GetLastThink() != "" {
			m.output = append(m.output, m.thinking.GetFormattedOutput())
			m.thinking.Reset()
		}

		if m.tools != nil {
			if call, ok := m.tools.ParseToolCall(response); ok {
				result, err := m.tools.Execute(*call)
				if err != nil {
					return StreamDoneMsg{
						FullResponse: response,
						Error:        fmt.Errorf("tool error: %w", err),
					}
				}

				return ToolResultMsg{
					Call:   call,
					Result: result,
				}
			}
		}

		return StreamDoneMsg{FullResponse: response}
	}
}
