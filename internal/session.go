package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Session struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	Model     string         `json:"model"`
	Provider  string         `json:"provider"`
	Messages  []ChatMessage  `json:"messages"`
}

type SessionMeta struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Model     string    `json:"model"`
	MsgCount  int       `json:"msg_count"`
}

func SessionsDir() string {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".config", "nexus", "sessions")
	os.MkdirAll(dir, 0755)
	return dir
}

func (s *Session) Save() error {
	dir := SessionsDir()
	path := filepath.Join(dir, s.ID+".json")

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func LoadSession(id string) (*Session, error) {
	dir := SessionsDir()
	path := filepath.Join(dir, id+".json")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}

	return &session, nil
}

func ListSessions() ([]SessionMeta, error) {
	dir := SessionsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var sessions []SessionMeta
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var session Session
		if err := json.Unmarshal(data, &session); err != nil {
			continue
		}

		sessions = append(sessions, SessionMeta{
			ID:        session.ID,
			Name:      session.Name,
			CreatedAt: session.CreatedAt,
			UpdatedAt: session.UpdatedAt,
			Model:     session.Model,
			MsgCount:  len(session.Messages),
		})
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	return sessions, nil
}

func DeleteSession(id string) error {
	dir := SessionsDir()
	path := filepath.Join(dir, id+".json")
	return os.Remove(path)
}

func GenerateSessionID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func AutoSaveSession(session *Session) {
	if session.ID == "" {
		session.ID = GenerateSessionID()
	}
	session.UpdatedAt = time.Now()
	session.Save()
}
