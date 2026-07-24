package internal

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type MCPServer struct {
	Name    string
	Command string
	Args    []string
	URL     string
	Process *exec.Cmd
	Stdin   io.WriteCloser
	Stdout  *bufio.Reader
	HTTP    *http.Client
}

type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type MCPResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
}

type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type MCPTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

type MCPToolCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type MCPToolResult struct {
	Content []MCPContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

type MCPContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

func LoadMCPServers() []*MCPServer {
	var servers []*MCPServer

	home, err := os.UserHomeDir()
	if err != nil {
		return servers
	}

	configPath := filepath.Join(home, ".config", "nexus", "mcp.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return servers
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return servers
	}

	var config map[string]struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
		URL     string   `json:"url"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return servers
	}

	for name, cfg := range config {
		servers = append(servers, &MCPServer{
			Name:    name,
			Command: cfg.Command,
			Args:    cfg.Args,
			URL:     cfg.URL,
			HTTP:    &http.Client{Timeout: 30 * time.Second},
		})
	}

	return servers
}

func (s *MCPServer) Connect() error {
	if s.URL != "" {
		return nil
	}

	if s.Command == "" {
		return fmt.Errorf("no command or URL specified for %s", s.Name)
	}

	s.Process = exec.Command(s.Command, s.Args...)
	s.Process.Stderr = os.Stderr

	stdin, err := s.Process.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	s.Stdin = stdin

	stdout, err := s.Process.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	s.Stdout = bufio.NewReader(stdout)

	if err := s.Process.Start(); err != nil {
		return fmt.Errorf("start process: %w", err)
	}

	return s.sendInitialize()
}

func (s *MCPServer) sendInitialize() error {
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":   map[string]interface{}{},
			"clientInfo": map[string]string{
				"name":    "nexus",
				"version": "0.1.0",
			},
		},
	}

	resp, err := s.sendRequest(req)
	if err != nil {
		return err
	}

	if resp.Error != nil {
		return fmt.Errorf("mcp error: %s", resp.Error.Message)
	}

	notif := MCPRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}
	return s.sendNotification(notif)
}

func (s *MCPServer) ListTools() ([]MCPTool, error) {
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/list",
	}

	resp, err := s.sendRequest(req)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("mcp error: %s", resp.Error.Message)
	}

	var result struct {
		Tools []MCPTool `json:"tools"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("parse tools: %w", err)
	}

	return result.Tools, nil
}

func (s *MCPServer) CallTool(call MCPToolCall) (*MCPToolResult, error) {
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      call.Name,
			"arguments": call.Arguments,
		},
	}

	resp, err := s.sendRequest(req)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("mcp error: %s", resp.Error.Message)
	}

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("parse result: %w", err)
	}

	return &result, nil
}

func (s *MCPServer) sendRequest(req MCPRequest) (*MCPResponse, error) {
	if s.URL != "" {
		return s.sendHTTPRequest(req)
	}
	return s.sendStdioRequest(req)
}

func (s *MCPServer) sendHTTPRequest(req MCPRequest) (*MCPResponse, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	resp, err := s.HTTP.Post(s.URL, "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var mcpResp MCPResponse
	if err := json.NewDecoder(resp.Body).Decode(&mcpResp); err != nil {
		return nil, err
	}

	return &mcpResp, nil
}

func (s *MCPServer) sendStdioRequest(req MCPRequest) (*MCPResponse, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	if _, err := s.Stdin.Write([]byte(header)); err != nil {
		return nil, err
	}
	if _, err := s.Stdin.Write(data); err != nil {
		return nil, err
	}

	return s.readResponse()
}

func (s *MCPServer) sendNotification(req MCPRequest) error {
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	if s.URL != "" {
		_, err := s.HTTP.Post(s.URL, "application/json", bytes.NewReader(data))
		return err
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	if _, err := s.Stdin.Write([]byte(header)); err != nil {
		return err
	}
	_, err = s.Stdin.Write(data)
	return err
}

func (s *MCPServer) readResponse() (*MCPResponse, error) {
	var buf strings.Builder

	for {
		line, err := s.Stdout.ReadString('\n')
		if err != nil {
			return nil, err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			break
		}

		if strings.HasPrefix(line, "Content-Length: ") {
			var length int
			fmt.Sscanf(line, "Content-Length: %d", &length)
			continue
		}

		buf.WriteString(line)
	}

	var resp MCPResponse
	if err := json.Unmarshal([]byte(buf.String()), &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

func (s *MCPServer) Close() error {
	if s.Process != nil && s.Process.Process != nil {
		return s.Process.Process.Kill()
	}
	return nil
}
