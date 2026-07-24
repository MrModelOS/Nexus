package client

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type ProviderType string

const (
	ProviderOllama   ProviderType = "ollama"
	ProviderOpenAI   ProviderType = "openai"
	ProviderAnthropic ProviderType = "anthropic"
	ProviderLMStudio ProviderType = "lmstudio"
)

type MultiProviderClient struct {
	Provider    ProviderType
	BaseURL     string
	APIKey      string
	HTTPClient  *http.Client
}

func NewMultiProvider(provider ProviderType, baseURL, apiKey string) *MultiProviderClient {
	return &MultiProviderClient{
		Provider:   provider,
		BaseURL:    strings.TrimRight(baseURL, "/"),
		APIKey:     apiKey,
		HTTPClient: &http.Client{},
	}
}

type OpenAIRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Stream      bool      `json:"stream"`
	Temperature float64   `json:"temperature,omitempty"`
}

type OpenAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type OpenAIStreamDelta struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

type AnthropicRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Stream      bool      `json:"stream"`
	Temperature float64   `json:"temperature,omitempty"`
	System      string    `json:"system,omitempty"`
}

type AnthropicResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
}

type AnthropicStreamEvent struct {
	Type  string `json:"type"`
	Delta *struct {
		Text string `json:"text"`
	} `json:"delta,omitempty"`
}

func (c *MultiProviderClient) Chat(messages []Message, model string, temperature float64) (string, error) {
	switch c.Provider {
	case ProviderOpenAI, ProviderLMStudio:
		return c.chatOpenAI(messages, model, temperature, false)
	case ProviderAnthropic:
		return c.chatAnthropic(messages, model, temperature, false)
	default:
		return c.chatOllama(messages, model, temperature, false)
	}
}

func (c *MultiProviderClient) StreamChat(messages []Message, model string, temperature float64, onToken func(string) error) error {
	switch c.Provider {
	case ProviderOpenAI, ProviderLMStudio:
		_, err := c.chatOpenAI(messages, model, temperature, true, onToken)
		return err
	case ProviderAnthropic:
		_, err := c.chatAnthropic(messages, model, temperature, true, onToken)
		return err
	default:
		return c.streamOllama(messages, model, temperature, onToken)
	}
}

func (c *MultiProviderClient) ListModels() ([]string, error) {
	switch c.Provider {
	case ProviderOllama:
		return c.listOllamaModels()
	case ProviderOpenAI, ProviderLMStudio:
		return c.listOpenAIModels()
	default:
		return nil, fmt.Errorf("list models not supported for %s", c.Provider)
	}
}

func (c *MultiProviderClient) chatOllama(messages []Message, model string, temperature float64, stream bool, onToken ...func(string) error) (string, error) {
	reqBody := ChatRequest{
		Model:       model,
		Messages:    messages,
		Stream:      stream,
		Temperature: temperature,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", c.BaseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("api error (%d): %s", resp.StatusCode, string(respBody))
	}

	if stream && len(onToken) > 0 {
		return c.readOllamaStream(resp.Body, onToken[0])
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", err
	}

	return chatResp.Message.Content, nil
}

func (c *MultiProviderClient) streamOllama(messages []Message, model string, temperature float64, onToken func(string) error) error {
	_, err := c.chatOllama(messages, model, temperature, true, onToken)
	return err
}

func (c *MultiProviderClient) readOllamaStream(body io.Reader, onToken func(string) error) (string, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), 64*1024)

	var full strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var delta StreamDelta
		if err := json.Unmarshal([]byte(line), &delta); err != nil {
			continue
		}

		if delta.Message.Content != "" {
			full.WriteString(delta.Message.Content)
			if err := onToken(delta.Message.Content); err != nil {
				return full.String(), err
			}
		}
	}

	return full.String(), scanner.Err()
}

func (c *MultiProviderClient) chatOpenAI(messages []Message, model string, temperature float64, stream bool, onToken ...func(string) error) (string, error) {
	reqBody := OpenAIRequest{
		Model:       model,
		Messages:    messages,
		Stream:      stream,
		Temperature: temperature,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", c.BaseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("api error (%d): %s", resp.StatusCode, string(respBody))
	}

	if stream && len(onToken) > 0 {
		return c.readOpenAIStream(resp.Body, onToken[0])
	}

	var openAIResp OpenAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&openAIResp); err != nil {
		return "", err
	}

	if len(openAIResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return openAIResp.Choices[0].Message.Content, nil
}

func (c *MultiProviderClient) readOpenAIStream(body io.Reader, onToken func(string) error) (string, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), 64*1024)

	var full strings.Builder

	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var delta OpenAIStreamDelta
		if err := json.Unmarshal([]byte(data), &delta); err != nil {
			continue
		}

		if len(delta.Choices) > 0 && delta.Choices[0].Delta.Content != "" {
			token := delta.Choices[0].Delta.Content
			full.WriteString(token)
			if err := onToken(token); err != nil {
				return full.String(), err
			}
		}
	}

	return full.String(), scanner.Err()
}

func (c *MultiProviderClient) chatAnthropic(messages []Message, model string, temperature float64, stream bool, onToken ...func(string) error) (string, error) {
	var systemMsg string
	var chatMsgs []Message

	for _, m := range messages {
		if m.Role == "system" {
			systemMsg = m.Content
		} else {
			chatMsgs = append(chatMsgs, m)
		}
	}

	reqBody := AnthropicRequest{
		Model:       model,
		Messages:    chatMsgs,
		MaxTokens:   4096,
		Stream:      stream,
		Temperature: temperature,
		System:      systemMsg,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", c.BaseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("api error (%d): %s", resp.StatusCode, string(respBody))
	}

	if stream && len(onToken) > 0 {
		return c.readAnthropicStream(resp.Body, onToken[0])
	}

	var anthropicResp AnthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&anthropicResp); err != nil {
		return "", err
	}

	if len(anthropicResp.Content) == 0 {
		return "", fmt.Errorf("no content in response")
	}

	return anthropicResp.Content[0].Text, nil
}

func (c *MultiProviderClient) readAnthropicStream(body io.Reader, onToken func(string) error) (string, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), 64*1024)

	var full strings.Builder

	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		var event AnthropicStreamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		if event.Type == "content_block_delta" && event.Delta != nil && event.Delta.Text != "" {
			token := event.Delta.Text
			full.WriteString(token)
			if err := onToken(token); err != nil {
				return full.String(), err
			}
		}
	}

	return full.String(), scanner.Err()
}

func (c *MultiProviderClient) listOllamaModels() ([]string, error) {
	resp, err := c.HTTPClient.Get(c.BaseURL + "/api/tags")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api error: %d", resp.StatusCode)
	}

	var tagsResp OllamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		return nil, err
	}

	var models []string
	for _, m := range tagsResp.Models {
		models = append(models, m.Name)
	}

	return models, nil
}

func (c *MultiProviderClient) listOpenAIModels() ([]string, error) {
	req, err := http.NewRequest("GET", c.BaseURL+"/v1/models", nil)
	if err != nil {
		return nil, err
	}

	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api error: %d", resp.StatusCode)
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var models []string
	for _, m := range result.Data {
		models = append(models, m.ID)
	}

	return models, nil
}
