package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"crypto/rand"
	"encoding/hex"
)

type Message struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	Voiced    bool      `json:"voiced"`
	Original  string    `json:"original,omitempty"`
}

type Session struct {
	ID        string    `json:"id"`
	OwnerID   string    `json:"owner_id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Messages  []Message `json:"messages"`
}

type Store struct {
	mu        sync.RWMutex
	sessions  map[string]*Session
	statePath string
	corpusDir string
}

var ErrNotFound = errors.New("not found")

func newID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func New(statePath, corpusDir string) (*Store, error) {
	s := &Store{
		sessions:  map[string]*Session{},
		statePath: statePath,
		corpusDir: corpusDir,
	}
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		return nil, err
	}
	if corpusDir != "" {
		if err := os.MkdirAll(corpusDir, 0o755); err != nil {
			return nil, err
		}
	}
	if data, err := os.ReadFile(statePath); err == nil {
		var list []*Session
		if err := json.Unmarshal(data, &list); err == nil {
			for _, sess := range list {
				s.sessions[sess.ID] = sess
			}
		}
	}
	return s, nil
}

func (s *Store) persist() {
	list := make([]*Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		list = append(list, sess)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].UpdatedAt.After(list[j].UpdatedAt) })
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return
	}
	tmp := s.statePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, s.statePath)
}

func (s *Store) CreateSession(ownerID, title string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if title == "" {
		title = "New chat"
	}
	sess := &Session{
		ID:        newID(),
		OwnerID:   ownerID,
		Title:     title,
		CreatedAt: now,
		UpdatedAt: now,
		Messages:  []Message{},
	}
	s.sessions[sess.ID] = sess
	s.persist()
	return cloneSession(sess)
}

func (s *Store) ListSessions(ownerID string) []*Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]*Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		if sess.OwnerID != ownerID {
			continue
		}
		clone := *sess
		clone.Messages = nil
		list = append(list, &clone)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].UpdatedAt.After(list[j].UpdatedAt) })
	return list
}

func (s *Store) GetSession(ownerID, id string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[id]
	if !ok || sess.OwnerID != ownerID {
		return nil, ErrNotFound
	}
	return cloneSession(sess), nil
}

func (s *Store) DeleteSession(ownerID, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[id]
	if !ok || sess.OwnerID != ownerID {
		return ErrNotFound
	}
	delete(s.sessions, id)
	s.persist()
	return nil
}

func (s *Store) AppendMessage(ownerID, sessionID string, m Message) (Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[sessionID]
	if !ok || sess.OwnerID != ownerID {
		return Message{}, ErrNotFound
	}
	if m.ID == "" {
		m.ID = newID()
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now()
	}
	sess.Messages = append(sess.Messages, m)
	sess.UpdatedAt = time.Now()
	if sess.Title == "New chat" && m.Role == "user" {
		sess.Title = truncate(m.Content, 48)
	}
	s.persist()
	if m.Role == "user" {
		s.appendCorpus(ownerID, m.Content)
	}
	return m, nil
}

func (s *Store) appendCorpus(ownerID, text string) {
	if s.corpusDir == "" {
		return
	}
	path := filepath.Join(s.corpusDir, "corpus_"+ownerID+".txt")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(text + "\n<|endoftext|>\n")
}

func (s *Store) History(ownerID, sessionID string) ([]Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[sessionID]
	if !ok || sess.OwnerID != ownerID {
		return nil, ErrNotFound
	}
	return append([]Message(nil), sess.Messages...), nil
}

func cloneSession(sess *Session) *Session {
	clone := *sess
	clone.Messages = append([]Message(nil), sess.Messages...)
	return &clone
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
