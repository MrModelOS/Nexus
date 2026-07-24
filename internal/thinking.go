package internal

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

type ThinkBlock struct {
	Content   string
	StartTime time.Time
	EndTime   time.Time
	IsOpen    bool
}

type ThinkingState struct {
	Enabled     bool
	InThink     bool
	CurrentThink *ThinkBlock
	History     []*ThinkBlock
	RawOutput   strings.Builder
	Formatted   strings.Builder
}

func NewThinkingState() *ThinkingState {
	return &ThinkingState{
		Enabled: true,
		History: make([]*ThinkBlock, 0),
	}
}

func (t *ThinkingState) ProcessToken(token string) string {
	if !t.Enabled {
		return token
	}

	t.RawOutput.WriteString(token)

	if strings.Contains(token, "<think>") {
		t.InThink = true
		t.CurrentThink = &ThinkBlock{
			StartTime: time.Now(),
			IsOpen:    true,
		}
		return ""
	}

	if strings.Contains(token, "</think>") {
		if t.CurrentThink != nil {
			t.CurrentThink.EndTime = time.Now()
			t.CurrentThink.IsOpen = false
			t.History = append(t.History, t.CurrentThink)
			t.CurrentThink = nil
		}
		t.InThink = false
		return ""
	}

	if t.InThink && t.CurrentThink != nil {
		t.CurrentThink.Content += token
		return ""
	}

	return token
}

func (t *ThinkingState) GetFormattedOutput() string {
	if len(t.History) == 0 && t.CurrentThink == nil {
		return ""
	}

	var out strings.Builder

	for i, block := range t.History {
		duration := block.EndTime.Sub(block.StartTime).Milliseconds()
		out.WriteString(fmt.Sprintf("\033[1;90m💭 Thinking (%dms):\033[0m\n", duration))

		trimmed := strings.TrimSpace(block.Content)
		lines := strings.Split(trimmed, "\n")
		maxLines := 5
		if len(lines) > maxLines {
			lines = lines[:maxLines]
			lines = append(lines, fmt.Sprintf("... (%d more lines)", len(lines)-maxLines))
		}
		for _, line := range lines {
			out.WriteString(fmt.Sprintf("\033[90m  %s\033[0m\n", line))
		}
		out.WriteString("\n")

		_ = i
	}

	if t.CurrentThink != nil && t.CurrentThink.Content != "" {
		out.WriteString("\033[1;36m💭 Thinking...\033[0m\n")
		trimmed := strings.TrimSpace(t.CurrentThink.Content)
		lines := strings.Split(trimmed, "\n")
		maxLines := 3
		if len(lines) > maxLines {
			lines = lines[:maxLines]
		}
		for _, line := range lines {
			out.WriteString(fmt.Sprintf("\033[90m  %s\033[0m\n", line))
		}
	}

	return out.String()
}

func (t *ThinkingState) GetLastThink() string {
	if len(t.History) == 0 {
		return ""
	}
	return t.History[len(t.History)-1].Content
}

func (t *ThinkingState) Reset() {
	t.InThink = false
	t.CurrentThink = nil
	t.History = make([]*ThinkBlock, 0)
	t.RawOutput.Reset()
	t.Formatted.Reset()
}

func (t *ThinkingState) Toggle() {
	t.Enabled = !t.Enabled
}

type ThinkingDisplay struct {
	Width     int
	Height    int
	Collapsed bool
}

func NewThinkingDisplay(width, height int) *ThinkingDisplay {
	return &ThinkingDisplay{
		Width:     width,
		Height:    height,
		Collapsed: true,
	}
}

func (d *ThinkingDisplay) Render(state *ThinkingState) string {
	if !state.Enabled || (len(state.History) == 0 && state.CurrentThink == nil) {
		return ""
	}

	var out strings.Builder

	header := "💭 Thinking"
	if d.Collapsed && len(state.History) > 0 {
		last := state.History[len(state.History)-1]
		duration := last.EndTime.Sub(last.StartTime).Milliseconds()
		header += fmt.Sprintf(" (%d steps, last: %dms)", len(state.History), duration)
	}

	out.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render(header))

	if !d.Collapsed {
		out.WriteString("\n")
		out.WriteString(state.GetFormattedOutput())
	}

	return out.String()
}

func (d *ThinkingDisplay) ToggleCollapse() {
	d.Collapsed = !d.Collapsed
}
