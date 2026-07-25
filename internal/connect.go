package internal

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/nexus-cli/nexus/config"
)

type ProviderTemplate struct {
	Name        string
	Type        string
	BaseURL     string
	Description string
	Models      []string
	NeedsKey    bool
	SetupURL    string
}

var PopularProviders = []ProviderTemplate{
	{
		Name:        "Ollama (Local)",
		Type:        "ollama",
		BaseURL:     "http://localhost:11434",
		Description: "Free local models - no API key needed",
		Models:      []string{"gemma3:1b", "llama3.2", "qwen2.5-coder", "codellama"},
		NeedsKey:    false,
		SetupURL:    "https://ollama.ai",
	},
	{
		Name:        "OpenAI",
		Type:        "openai",
		BaseURL:     "https://api.openai.com",
		Description: "GPT-4o, GPT-4o-mini, o1-mini",
		Models:      []string{"gpt-4o", "gpt-4o-mini", "o1-mini", "gpt-4-turbo"},
		NeedsKey:    true,
		SetupURL:    "https://platform.openai.com/api-keys",
	},
	{
		Name:        "Anthropic",
		Type:        "anthropic",
		BaseURL:     "https://api.anthropic.com",
		Description: "Claude Sonnet, Claude Haiku",
		Models:      []string{"claude-sonnet-4-20250514", "claude-3-5-haiku-20241022", "claude-3-opus-20240229"},
		NeedsKey:    true,
		SetupURL:    "https://console.anthropic.com/settings/keys",
	},
	{
		Name:        "LM Studio",
		Type:        "lmstudio",
		BaseURL:     "http://localhost:1234",
		Description: "Local models via LM Studio - no API key needed",
		Models:      []string{},
		NeedsKey:    false,
		SetupURL:    "https://lmstudio.ai",
	},
	{
		Name:        "Google Gemini",
		Type:        "openai",
		BaseURL:     "https://generativelanguage.googleapis.com/v1beta/openai",
		Description: "Gemini 2.0 Flash, Gemini Pro",
		Models:      []string{"gemini-2.0-flash", "gemini-1.5-pro", "gemini-1.5-flash"},
		NeedsKey:    true,
		SetupURL:    "https://aistudio.google.com/apikey",
	},
	{
		Name:        "Groq",
		Type:        "openai",
		BaseURL:     "https://api.groq.com/openai/v1",
		Description: "Fast inference - Llama, Mixtral, Gemma",
		Models:      []string{"llama-3.3-70b-versatile", "mixtral-8x7b-32768", "gemma2-9b-it"},
		NeedsKey:    true,
		SetupURL:    "https://console.groq.com/keys",
	},
	{
		Name:        "DeepSeek",
		Type:        "openai",
		BaseURL:     "https://api.deepseek.com",
		Description: "DeepSeek Chat, DeepSeek Coder",
		Models:      []string{"deepseek-chat", "deepseek-coder"},
		NeedsKey:    true,
		SetupURL:    "https://platform.deepseek.com/api_keys",
	},
	{
		Name:        "OpenRouter",
		Type:        "openai",
		BaseURL:     "https://openrouter.ai/api/v1",
		Description: "Access 100+ models via single API",
		Models:      []string{"anthropic/claude-sonnet-4-20250514", "openai/gpt-4o", "meta-llama/llama-3.3-70b-instruct"},
		NeedsKey:    true,
		SetupURL:    "https://openrouter.ai/settings/keys",
	},
}

type ConnectStep int

const (
	ConnectSelectProvider ConnectStep = iota
	ConnectEnterAPIKey
	ConnectSelectModel
	ConnectDone
)

type ConnectState struct {
	Step       ConnectStep
	Provider   int
	Cursor     int
	APIKey     string
	ModelCursor int
}

func NewConnectState() *ConnectState {
	return &ConnectState{
		Step: ConnectSelectProvider,
	}
}

func (cs *ConnectState) Render() string {
	switch cs.Step {
	case ConnectSelectProvider:
		return cs.renderProviderList()
	case ConnectEnterAPIKey:
		return cs.renderAPIKeyInput()
	case ConnectSelectModel:
		return cs.renderModelSelect()
	case ConnectDone:
		return cs.renderDone()
	}
	return ""
}

func (cs *ConnectState) renderProviderList() string {
	var out strings.Builder

	titleS := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	labelS := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	valueS := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252"))
	descS := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	tipS := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Italic(true).PaddingTop(1)

	out.WriteString(titleS.Render("🔌 Connect Provider") + "\n\n")

	for i, p := range PopularProviders {
		prefix := "  "
		if i == cs.Cursor {
			prefix = "▸ "
			out.WriteString(valueS.Render(prefix+p.Name) + "\n")
			out.WriteString("    " + descS.Render(p.Description) + "\n")
			if p.NeedsKey {
				out.WriteString("    " + labelS.Render("API key required") + "\n")
			} else {
				out.WriteString("    " + labelS.Render("No API key needed") + "\n")
			}
			if p.SetupURL != "" {
				out.WriteString("    " + labelS.Render("Get key: "+p.SetupURL) + "\n")
			}
			out.WriteString("\n")
		} else {
			out.WriteString(labelS.Render(prefix+p.Name) + "\n")
		}
	}

	out.WriteString(tipS.Render(" ↑↓ select • Enter connect • Esc cancel"))
	return out.String()
}

func (cs *ConnectState) renderAPIKeyInput() string {
	var out strings.Builder

	titleS := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	labelS := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	valueS := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252"))

	p := PopularProviders[cs.Provider]

	out.WriteString(titleS.Render("🔌 "+p.Name) + "\n\n")
	out.WriteString(labelS.Render("Enter your API key:") + "\n\n")
	out.WriteString("  " + valueS.Render(cs.APIKey) + "█\n\n")
	out.WriteString(labelS.Render("Press Enter to save • Esc to cancel") + "\n")

	return out.String()
}

func (cs *ConnectState) renderModelSelect() string {
	var out strings.Builder

	titleS := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	labelS := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	valueS := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252"))
	tipS := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Italic(true).PaddingTop(1)

	p := PopularProviders[cs.Provider]

	out.WriteString(titleS.Render("🔌 "+p.Name+" - Select Model") + "\n\n")

	for i, model := range p.Models {
		prefix := "  "
		if i == cs.ModelCursor {
			prefix = "▸ "
			out.WriteString(valueS.Render(prefix+model) + "\n")
		} else {
			out.WriteString(labelS.Render(prefix+model) + "\n")
		}
	}

	out.WriteString(tipS.Render(" ↑↓ select • Enter confirm • Esc skip"))
	return out.String()
}

func (cs *ConnectState) renderDone() string {
	var out strings.Builder

	titleS := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))
	labelS := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	p := PopularProviders[cs.Provider]

	out.WriteString(titleS.Render("✓ Connected!") + "\n\n")
	out.WriteString(labelS.Render("Provider: "+p.Name) + "\n")
	if cs.APIKey != "" {
		out.WriteString(labelS.Render("API key: saved") + "\n")
	}
	out.WriteString(labelS.Render("Type /models to see available models") + "\n")

	return out.String()
}

func (cs *ConnectState) HandleKey(key string) bool {
	switch cs.Step {
	case ConnectSelectProvider:
		switch key {
		case "up":
			cs.Cursor--
			if cs.Cursor < 0 {
				cs.Cursor = len(PopularProviders) - 1
			}
		case "down":
			cs.Cursor++
			if cs.Cursor >= len(PopularProviders) {
				cs.Cursor = 0
			}
		case "enter":
			cs.Provider = cs.Cursor
			p := PopularProviders[cs.Provider]
			if p.NeedsKey {
				cs.Step = ConnectEnterAPIKey
			} else {
				cs.Step = ConnectSelectModel
			}
		case "esc":
			return false
		}
	case ConnectEnterAPIKey:
		switch key {
		case "enter":
			if len(cs.APIKey) > 10 {
				cs.Step = ConnectSelectModel
			}
		case "esc":
			cs.Step = ConnectSelectProvider
			cs.APIKey = ""
		case "backspace":
			if len(cs.APIKey) > 0 {
				cs.APIKey = cs.APIKey[:len(cs.APIKey)-1]
			}
		default:
			if len(key) == 1 {
				cs.APIKey += key
			}
		}
	case ConnectSelectModel:
		p := PopularProviders[cs.Provider]
		switch key {
		case "up":
			cs.ModelCursor--
			if cs.ModelCursor < 0 {
				cs.ModelCursor = len(p.Models) - 1
			}
		case "down":
			cs.ModelCursor++
			if cs.ModelCursor >= len(p.Models) {
				cs.ModelCursor = 0
			}
		case "enter":
			cs.Step = ConnectDone
		case "esc":
			if len(p.Models) > 0 {
				cs.ModelCursor = 0
			}
			cs.Step = ConnectDone
		}
	case ConnectDone:
		return false
	}
	return true
}

func (cs *ConnectState) ApplyConfig(cfg *config.Config) *config.Config {
	if cfg.Providers == nil {
		cfg.Providers = make(map[string]config.Provider)
	}

	p := PopularProviders[cs.Provider]
	providerName := strings.ToLower(p.Name)
	providerName = strings.ReplaceAll(providerName, " ", "")
	providerName = strings.ReplaceAll(providerName, "(", "")
	providerName = strings.ReplaceAll(providerName, ")", "")
	providerName = strings.ReplaceAll(providerName, "local", "")

	cfg.Providers[providerName] = config.Provider{
		Type:    p.Type,
		BaseURL: p.BaseURL,
		APIKey:  cs.APIKey,
		Models:  p.Models,
	}

	if len(p.Models) > 0 && cs.ModelCursor < len(p.Models) {
		cfg.DefaultModel = p.Models[cs.ModelCursor]
	}

	return cfg
}

func HasValidModels(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	for _, p := range cfg.Providers {
		if len(p.Models) > 0 {
			return true
		}
	}
	return false
}

func GetConnectHint() string {
	return "\033[1;33m💡 No models configured. Run /connect to set up a provider\033[0m"
}
