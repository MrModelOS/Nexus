package internal

import (
	"fmt"
	"strings"
)

type CompactResult struct {
	Summary  string
	Messages []ChatMessage
}

func CompactHistory(messages []ChatMessage, maxMessages int) CompactResult {
	if len(messages) <= maxMessages {
		return CompactResult{
			Summary:  "",
			Messages: messages,
		}
	}

	cutoff := len(messages) - maxMessages
	oldMessages := messages[:cutoff]
	recentMessages := messages[cutoff:]

	summary := summarizeMessages(oldMessages)

	return CompactResult{
		Summary:  summary,
		Messages: recentMessages,
	}
}

func summarizeMessages(messages []ChatMessage) string {
	if len(messages) == 0 {
		return ""
	}

	var userMsgs []string
	var assistantMsgs []string

	for _, m := range messages {
		if m.Role == "user" {
			truncated := truncate(m.Content, 200)
			userMsgs = append(userMsgs, truncated)
		} else if m.Role == "assistant" {
			truncated := truncate(m.Content, 200)
			assistantMsgs = append(assistantMsgs, truncated)
		}
	}

	var summary strings.Builder
	summary.WriteString("[Context from previous conversation]\n\n")

	if len(userMsgs) > 0 {
		summary.WriteString("User discussed:\n")
		for i, msg := range userMsgs {
			summary.WriteString(fmt.Sprintf("%d. %s\n", i+1, msg))
		}
	}

	if len(assistantMsgs) > 0 {
		summary.WriteString("\nAssistant responded about:\n")
		for i, msg := range assistantMsgs {
			summary.WriteString(fmt.Sprintf("%d. %s\n", i+1, msg))
		}
	}

	return summary.String()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func FormatCompactResult(result CompactResult) string {
	if result.Summary == "" {
		return "History is already compact."
	}

	return fmt.Sprintf(
		"\033[1;33mCompacted %d older messages into summary.\033[0m\nSummary saved to context.",
		countOldMessages(result),
	)
}

func countOldMessages(result CompactResult) int {
	if result.Summary == "" {
		return 0
	}
	lines := strings.Split(result.Summary, "\n")
	count := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "User discussed:") || strings.HasPrefix(line, "Assistant responded about:") {
			continue
		}
		if len(line) > 3 && line[0] >= '1' && line[0] <= '9' {
			count++
		}
	}
	return count
}
