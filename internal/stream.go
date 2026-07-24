package internal

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/nexus-cli/nexus/client"
)

var (
	userStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("6")).
			Bold(true)
	assistantStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("2"))
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("1")).
			Bold(true)
	streamingSt = lipgloss.NewStyle().
			Foreground(lipgloss.Color("5"))
)

func PrintUser(text string) {
	fmt.Fprintln(os.Stderr, userStyle.Render("You: ")+text)
}

func PrintAssistantStart() {
	fmt.Fprint(os.Stderr, assistantStyle.Render("Nexus: "))
}

func PrintAssistantEnd() {
	fmt.Fprintln(os.Stderr)
}

func PrintError(err error) {
	fmt.Fprintln(os.Stderr, errorStyle.Render("Error: ")+err.Error())
}

func CreateMessages(history []client.Message, systemPrompt string) []client.Message {
	var msgs []client.Message
	if systemPrompt != "" {
		msgs = append(msgs, client.Message{Role: "system", Content: systemPrompt})
	}
	msgs = append(msgs, history...)
	return msgs
}
