package internal

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nexus-cli/nexus/client"
	"github.com/nexus-cli/nexus/config"
)

type FailoverEvent struct {
	Type      string    `json:"type"`
	Model     string    `json:"model"`
	Provider  string    `json:"provider"`
	Error     string    `json:"error,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

type FailoverManager struct {
	mu              sync.RWMutex
	config          *config.Config
	models          []config.PoolModel
	currentIndex    int
	currentClient   *client.MultiProviderClient
	currentModel    string
	currentProvider string
	eventLog        []FailoverEvent
	onEvent         func(FailoverEvent)
}

func NewFailoverManager(cfg *config.Config) *FailoverManager {
	fm := &FailoverManager{
		config:   cfg,
		models:   cfg.GetPoolModels(),
		eventLog: make([]FailoverEvent, 0, 100),
	}

	sort.Slice(fm.models, func(i, j int) bool {
		return fm.models[i].Priority < fm.models[j].Priority
	})

	if len(fm.models) > 0 {
		fm.switchToModel(0)
	}

	return fm
}

func (fm *FailoverManager) OnEvent(handler func(FailoverEvent)) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.onEvent = handler
}

func (fm *FailoverManager) emitEvent(event FailoverEvent) {
	fm.mu.Lock()
	fm.eventLog = append(fm.eventLog, event)
	if len(fm.eventLog) > 1000 {
		fm.eventLog = fm.eventLog[len(fm.eventLog)-500:]
	}
	handler := fm.onEvent
	fm.mu.Unlock()

	if handler != nil {
		go handler(event)
	}
}

func (fm *FailoverManager) switchToModel(index int) {
	if index < 0 || index >= len(fm.models) {
		return
	}

	fm.currentIndex = index
	model := fm.models[index]

	fm.currentModel = model.Name
	fm.currentProvider = model.Provider

	if model.BaseURL != "" {
		fm.currentClient = client.NewMultiProvider(
			client.ProviderType(model.Provider),
			model.BaseURL,
			model.APIKey,
		)
	} else if p, ok := fm.config.Providers[model.Provider]; ok {
		fm.currentClient = client.NewMultiProvider(
			client.ProviderType(p.Type),
			p.BaseURL,
			p.APIKey,
		)
	}

	fm.emitEvent(FailoverEvent{
		Type:      "switch",
		Model:     model.Name,
		Provider:  model.Provider,
		Timestamp: time.Now(),
	})
}

func (fm *FailoverManager) GetClient() *client.MultiProviderClient {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	return fm.currentClient
}

func (fm *FailoverManager) GetCurrentModel() string {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	return fm.currentModel
}

func (fm *FailoverManager) GetCurrentProvider() string {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	return fm.currentProvider
}

func (fm *FailoverManager) StreamChatWithFailover(
	messages []client.Message,
	model string,
	temperature float64,
	onToken func(string) error,
) (string, error) {
	fm.mu.RLock()
	clientCopy := fm.currentClient
	modelCopy := fm.currentModel
	fm.mu.RUnlock()

	if clientCopy == nil {
		return "", fmt.Errorf("no models available in pool")
	}

	maxRetries := fm.config.Pool.MaxRetries
	retryDelay := time.Duration(fm.config.Pool.RetryDelay) * time.Millisecond

	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(retryDelay)
		}

		err := clientCopy.StreamChat(messages, modelCopy, temperature, onToken)
		if err == nil {
			return "", nil
		}

		lastErr = err

		fm.emitEvent(FailoverEvent{
			Type:      "error",
			Model:     modelCopy,
			Error:     err.Error(),
			Timestamp: time.Now(),
		})

		if fm.isRateLimitError(err) || fm.isServerError(err) {
			fm.mu.Lock()
			nextIndex := fm.findNextModel()
			if nextIndex != fm.currentIndex {
				fm.switchToModel(nextIndex)
				clientCopy = fm.currentClient
				modelCopy = fm.currentModel
				fm.mu.Unlock()

				fm.emitEvent(FailoverEvent{
					Type:      "failover",
					Model:     modelCopy,
					Provider:  fm.currentProvider,
					Error:     err.Error(),
					Timestamp: time.Now(),
				})
				continue
			}
			fm.mu.Unlock()
		}

		if !fm.isRetryableError(err) {
			return "", err
		}
	}

	return "", fmt.Errorf("all retries exhausted, last error: %w", lastErr)
}

func (fm *FailoverManager) ChatWithFailover(
	messages []client.Message,
	model string,
	temperature float64,
) (string, error) {
	fm.mu.RLock()
	clientCopy := fm.currentClient
	modelCopy := fm.currentModel
	fm.mu.RUnlock()

	if clientCopy == nil {
		return "", fmt.Errorf("no models available in pool")
	}

	maxRetries := fm.config.Pool.MaxRetries
	retryDelay := time.Duration(fm.config.Pool.RetryDelay) * time.Millisecond

	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(retryDelay)
		}

		result, err := clientCopy.Chat(messages, modelCopy, temperature)
		if err == nil {
			return result, nil
		}

		lastErr = err

		fm.emitEvent(FailoverEvent{
			Type:      "error",
			Model:     modelCopy,
			Error:     err.Error(),
			Timestamp: time.Now(),
		})

		if fm.isRateLimitError(err) || fm.isServerError(err) {
			fm.mu.Lock()
			nextIndex := fm.findNextModel()
			if nextIndex != fm.currentIndex {
				fm.switchToModel(nextIndex)
				clientCopy = fm.currentClient
				modelCopy = fm.currentModel
				fm.mu.Unlock()

				fm.emitEvent(FailoverEvent{
					Type:      "failover",
					Model:     modelCopy,
					Provider:  fm.currentProvider,
					Error:     err.Error(),
					Timestamp: time.Now(),
				})
				continue
			}
			fm.mu.Unlock()
		}

		if !fm.isRetryableError(err) {
			return "", err
		}
	}

	return "", fmt.Errorf("all retries exhausted, last error: %w", lastErr)
}

func (fm *FailoverManager) findNextModel() int {
	if len(fm.models) <= 1 {
		return fm.currentIndex
	}

	nextIndex := (fm.currentIndex + 1) % len(fm.models)
	return nextIndex
}

func (fm *FailoverManager) isRateLimitError(err error) bool {
	errStr := err.Error()
	return strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "too many requests") ||
		strings.Contains(errStr, "quota exceeded")
}

func (fm *FailoverManager) isServerError(err error) bool {
	errStr := err.Error()
	return strings.Contains(errStr, "500") ||
		strings.Contains(errStr, "502") ||
		strings.Contains(errStr, "503") ||
		strings.Contains(errStr, "504") ||
		strings.Contains(errStr, "server error") ||
		strings.Contains(errStr, "timeout")
}

func (fm *FailoverManager) isRetryableError(err error) bool {
	return fm.isRateLimitError(err) || fm.isServerError(err) || fm.isNetworkError(err)
}

func (fm *FailoverManager) isNetworkError(err error) bool {
	errStr := err.Error()
	return strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "dial tcp") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "network")
}

func (fm *FailoverManager) GetEventLog() []FailoverEvent {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	events := make([]FailoverEvent, len(fm.eventLog))
	copy(events, fm.eventLog)
	return events
}

func (fm *FailoverManager) GetStatus() string {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	status := "\033[1;35mModel Pool Status:\033[0m\n\n"
	status += fmt.Sprintf("Active: %s (%s)\n", fm.currentModel, fm.currentProvider)
	status += fmt.Sprintf("Priority: %d\n", fm.models[fm.currentIndex].Priority)
	status += fmt.Sprintf("Pool size: %d models\n", len(fm.models))

	status += "\n\033[1;36mAvailable Models:\033[0m\n"
	for i, m := range fm.models {
		prefix := "  "
		if i == fm.currentIndex {
			prefix = "▸ "
		}
		status += fmt.Sprintf("%s%s [%s] P%d\n", prefix, m.Name, m.Provider, m.Priority)
	}

	recentEvents := fm.eventLog
	if len(recentEvents) > 5 {
		recentEvents = recentEvents[len(recentEvents)-5:]
	}
	if len(recentEvents) > 0 {
		status += "\n\033[1;33mRecent Events:\033[0m\n"
		for _, e := range recentEvents {
			status += fmt.Sprintf("  [%s] %s: %s\n", e.Type, e.Model, e.Error)
		}
	}

	return status
}

func (fm *FailoverManager) HTTPStatusFromError(err error) int {
	errStr := err.Error()
	if strings.Contains(errStr, "429") {
		return http.StatusTooManyRequests
	}
	if strings.Contains(errStr, "500") {
		return http.StatusInternalServerError
	}
	if strings.Contains(errStr, "502") {
		return http.StatusBadGateway
	}
	if strings.Contains(errStr, "503") {
		return http.StatusServiceUnavailable
	}
	if strings.Contains(errStr, "504") {
		return http.StatusGatewayTimeout
	}
	return 0
}
