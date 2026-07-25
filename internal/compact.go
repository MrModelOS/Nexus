package internal

import (
	"fmt"
	"strings"
	"sync"

	"github.com/nexus-cli/nexus/client"
	"github.com/nexus-cli/nexus/config"
)

type CompactResult struct {
	Summary  string
	Messages []ChatMessage
	Compressed bool
}

type ContextCompressor struct {
	mu              sync.RWMutex
	config          *config.Config
	fallbackClient  *client.MultiProviderClient
	tokenCount      int
	maxTokens       int
	autoCompact     bool
	compactPercent  int
	onCompress      func(CompressionEvent)
}

type CompressionEvent struct {
	Type        string `json:"type"`
	TokensUsed  int    `json:"tokens_used"`
	TokensMax   int    `json:"tokens_max"`
	Percent     int    `json:"percent"`
	Summary     string `json:"summary,omitempty"`
}

func NewContextCompressor(cfg *config.Config) *ContextCompressor {
	cc := &ContextCompressor{
		config:        cfg,
		maxTokens:     cfg.GetContextConfig().MaxTokens,
		autoCompact:   cfg.GetContextConfig().AutoCompact,
		compactPercent: cfg.GetContextConfig().CompactPercent,
	}

	if compactor := cfg.GetCompactorConfig(); compactor != nil && compactor.Enabled {
		if p, ok := cfg.Providers[compactor.Provider]; ok {
			cc.fallbackClient = client.NewMultiProvider(
				client.ProviderType(p.Type),
				p.BaseURL,
				p.APIKey,
			)
		}
	}

	return cc
}

func (cc *ContextCompressor) OnCompress(handler func(CompressionEvent)) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.onCompress = handler
}

func (cc *ContextCompressor) emitEvent(event CompressionEvent) {
	cc.mu.Lock()
	handler := cc.onCompress
	cc.mu.Unlock()

	if handler != nil {
		go handler(event)
	}
}

func (cc *ContextCompressor) EstimateTokens(messages []ChatMessage) int {
	total := 0
	for _, m := range messages {
		total += estimateTokens(m.Content)
	}
	return total
}

func estimateTokens(text string) int {
	words := strings.Fields(text)
	tokens := 0
	for _, word := range words {
		tokens++
		if len(word) > 4 {
			tokens += len(word) / 4
		}
	}
	return tokens
}

func (cc *ContextCompressor) ShouldCompact(messages []ChatMessage) bool {
	if !cc.autoCompact {
		return false
	}
	tokens := cc.EstimateTokens(messages)
	threshold := cc.maxTokens * cc.compactPercent / 100
	return tokens > threshold
}

func (cc *ContextCompressor) CompressWithModel(messages []ChatMessage) (CompactResult, error) {
	cc.mu.RLock()
	clientCopy := cc.fallbackClient
	configCopy := cc.config.GetCompactorConfig()
	cc.mu.RUnlock()

	if clientCopy == nil || configCopy == nil {
		return cc.LocalCompress(messages)
	}

	tokensBefore := cc.EstimateTokens(messages)

	summary, err := cc.summarizeWithLLM(messages, clientCopy, configCopy)
	if err != nil {
		return cc.LocalCompress(messages)
	}

	recentCount := len(messages) / 4
	if recentCount < 5 {
		recentCount = 5
	}
	if recentCount > len(messages) {
		recentCount = len(messages)
	}

	recentMessages := messages[len(messages)-recentCount:]

	compressed := make([]ChatMessage, 0, recentCount+1)
	compressed = append(compressed, ChatMessage{
		Role:    "system",
		Content: fmt.Sprintf("[Context compressed - %d tokens summarized]\n\n%s", tokensBefore, summary),
	})
	compressed = append(compressed, recentMessages...)

	cc.emitEvent(CompressionEvent{
		Type:       "compress",
		TokensUsed: tokensBefore,
		TokensMax:  cc.maxTokens,
		Percent:    tokensBefore * 100 / cc.maxTokens,
		Summary:    summary,
	})

	return CompactResult{
		Summary:    summary,
		Messages:   compressed,
		Compressed: true,
	}, nil
}

func (cc *ContextCompressor) summarizeWithLLM(
	messages []ChatMessage,
	c *client.MultiProviderClient,
	cfg *config.Compactor,
) (string, error) {
	var sb strings.Builder
	sb.WriteString("Previous conversation:\n\n")

	for _, m := range messages {
		role := "User"
		if m.Role == "assistant" {
			role = "Assistant"
		}
		content := m.Content
		if len(content) > 500 {
			content = content[:500] + "..."
		}
		sb.WriteString(fmt.Sprintf("%s: %s\n\n", role, content))
	}

	prompt := cfg.SummaryPrompt
	if prompt == "" {
		prompt = "Summarize this conversation concisely, preserving key context and decisions:"
	}

	chatMessages := []client.Message{
		{Role: "user", Content: prompt + "\n\n" + sb.String()},
	}

	result, err := c.Chat(chatMessages, cfg.Model, 0.3)
	if err != nil {
		return "", err
	}

	return result, nil
}

func (cc *ContextCompressor) LocalCompress(messages []ChatMessage) (CompactResult, error) {
	tokensBefore := cc.EstimateTokens(messages)

	cutoff := len(messages) - len(messages)/4
	if cutoff < 1 {
		cutoff = 1
	}

	oldMessages := messages[:cutoff]
	recentMessages := messages[cutoff:]

	summary := summarizeMessages(oldMessages)

	compressed := make([]ChatMessage, 0, len(recentMessages)+1)
	compressed = append(compressed, ChatMessage{
		Role:    "system",
		Content: fmt.Sprintf("[Context compressed locally - %d tokens]\n\n%s", tokensBefore, summary),
	})
	compressed = append(compressed, recentMessages...)

	cc.emitEvent(CompressionEvent{
		Type:       "compress_local",
		TokensUsed: tokensBefore,
		TokensMax:  cc.maxTokens,
		Percent:    tokensBefore * 100 / cc.maxTokens,
		Summary:    summary,
	})

	return CompactResult{
		Summary:    summary,
		Messages:   compressed,
		Compressed: true,
	}, nil
}

func (cc *ContextCompressor) GetTokenUsage(messages []ChatMessage) (int, int, int) {
	tokens := cc.EstimateTokens(messages)
	percent := tokens * 100 / cc.maxTokens
	return tokens, cc.maxTokens, percent
}

func (cc *ContextCompressor) GetStatus() string {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	status := "\033[1;35mContext Compressor:\033[0m\n\n"
	status += fmt.Sprintf("Max tokens: %d\n", cc.maxTokens)
	status += fmt.Sprintf("Auto compact: %v\n", cc.autoCompact)
	status += fmt.Sprintf("Compact at: %d%%\n", cc.compactPercent)

	if cc.fallbackClient != nil {
		cfg := cc.config.GetCompactorConfig()
		status += fmt.Sprintf("Compactor model: %s (%s)\n", cfg.Model, cfg.Provider)
	} else {
		status += "Compactor: local (no LLM)\n"
	}

	return status
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

	method := "locally"
	if result.Compressed {
		method = "with LLM"
	}

	return fmt.Sprintf(
		"\033[1;33mCompressed conversation %s.\033[0m",
		method,
	)
}
