package internal

import (
	"bufio"
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
)

var (
	bannerBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("33")).
			Padding(1, 2).
			Width(55)

	bannerTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("86"))

	bannerLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240"))

	bannerValueStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("252"))

	bannerTipStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Italic(true).
			PaddingTop(1)
)

func DrawStartupBanner(modelName string) {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "~"
	} else {
		homeDir, _ := os.UserHomeDir()
		if len(cwd) >= len(homeDir) && cwd[:len(homeDir)] == homeDir {
			cwd = "~" + cwd[len(homeDir):]
		}
	}

	header := bannerTitleStyle.Render(" >_ Nexus CLI")

	modelInfo := fmt.Sprintf("%-10s %s %s",
		bannerLabelStyle.Render("model:"),
		bannerValueStyle.Render(modelName),
		bannerLabelStyle.Render("/model to change"),
	)

	dirInfo := fmt.Sprintf("%-10s %s",
		bannerLabelStyle.Render("directory:"),
		bannerValueStyle.Render(cwd),
	)

	content := fmt.Sprintf("%s\n\n%s\n%s", header, modelInfo, dirInfo)
	boxed := bannerBoxStyle.Render(content)

	tipText := bannerTipStyle.Render(" Tip: Use /review on my current changes")

	fmt.Println()
	fmt.Println(boxed)
	fmt.Println(tipText)
	fmt.Println()
}

func WaitForEnter() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\033[2K\r")
	reader.ReadString('\n')
}

func DrawHelp() {
	cmdStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("12")).
		Bold(true)

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	fmt.Println()
	fmt.Println(cmdStyle.Render("  Commands:"))
	fmt.Println()

	commands := [][2]string{
		{"/clear", "Clear chat history"},
		{"/model", "Change or view current model"},
		{"/review", "Review staged git changes with AI"},
		{"/help", "Show this help message"},
		{"/quit", "Exit Nexus"},
	}

	for _, cmd := range commands {
		fmt.Printf("    %-12s %s\n", cmdStyle.Render(cmd[0]), descStyle.Render(cmd[1]))
	}

	fmt.Println()
}

func FormatModel(provider, model string) string {
	if provider != "" {
		return fmt.Sprintf("%s/%s", provider, model)
	}
	return model
}
