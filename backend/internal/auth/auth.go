package auth

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"password_hash"`
	CreatedAt    time.Time `json:"created_at"`
}

type Public struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

func (u *User) Public() Public {
	return Public{ID: u.ID, Email: u.Email}
}

type token struct {
	UserID  string    `json:"user_id"`
	Expires time.Time `json:"expires"`
}

type persisted struct {
	Users  []*User          `json:"users"`
	Tokens map[string]token `json:"tokens"`
}

type Store struct {
	mu     sync.RWMutex
	byID   map[string]*User
	byMail map[string]string
	tokens map[string]token
	path   string
}

var (
	ErrEmailTaken         = errors.New("that email is already registered")
	ErrInvalidCredentials = errors.New("email or password is incorrect")
	ErrBadEmail           = errors.New("enter a valid email address")
	ErrWeakPassword       = errors.New("password must be at least 8 characters")
)

const (
	pbkdf2Iterations = 210000
	pbkdf2KeyLen     = 32
	saltLen          = 16
	tokenTTL         = 30 * 24 * time.Hour
)

func New(path string) (*Store, error) {
	s := &Store{
		byID:   map[string]*User{},
		byMail: map[string]string{},
		tokens: map[string]token{},
		path:   path,
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s, nil
		}
		return nil, err
	}
	var p persisted
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	for _, u := range p.Users {
		s.byID[u.ID] = u
		s.byMail[u.Email] = u.ID
	}
	now := time.Now()
	for tok, entry := range p.Tokens {
		if entry.Expires.After(now) {
			s.tokens[tok] = entry
		}
	}
	return s, nil
}

func (s *Store) persist() {
	list := make([]*User, 0, len(s.byID))
	for _, u := range s.byID {
		list = append(list, u)
	}
	data, err := json.MarshalIndent(persisted{Users: list, Tokens: s.tokens}, "", "  ")
	if err != nil {
		return
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return
	}
	_ = os.Rename(tmp, s.path)
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validEmail(email string) bool {
	at := strings.IndexByte(email, '@')
	if at <= 0 || at == len(email)-1 {
		return false
	}
	if strings.Contains(email, " ") {
		return false
	}
	return strings.Contains(email[at+1:], ".")
}

func newID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func newToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func hashPassword(password string) string {
	salt := make([]byte, saltLen)
	_, _ = rand.Read(salt)
	key, _ := pbkdf2.Key(sha256.New, password, salt, pbkdf2Iterations, pbkdf2KeyLen)
	return strings.Join([]string{
		"pbkdf2-sha256",
		strconv.Itoa(pbkdf2Iterations),
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	}, "$")
}

func verifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter <= 0 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, iter, len(want))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}

func (s *Store) Register(email, password string) (*User, error) {
	email = normalizeEmail(email)
	if !validEmail(email) {
		return nil, ErrBadEmail
	}
	if len(password) < 8 || len(password) > 256 {
		return nil, ErrWeakPassword
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byMail[email]; ok {
		return nil, ErrEmailTaken
	}
	u := &User{
		ID:           newID(),
		Email:        email,
		PasswordHash: hashPassword(password),
		CreatedAt:    time.Now(),
	}
	s.byID[u.ID] = u
	s.byMail[email] = u.ID
	s.persist()
	return u, nil
}

func (s *Store) Authenticate(email, password string) (*User, error) {
	email = normalizeEmail(email)
	s.mu.RLock()
	id, ok := s.byMail[email]
	var u *User
	if ok {
		u = s.byID[id]
	}
	s.mu.RUnlock()
	if u == nil {
		verifyPassword("pbkdf2-sha256$1$AA$AA", password)
		return nil, ErrInvalidCredentials
	}
	if !verifyPassword(u.PasswordHash, password) {
		return nil, ErrInvalidCredentials
	}
	return u, nil
}

func (s *Store) Mint(userID string) (string, time.Time) {
	tok := newToken()
	expires := time.Now().Add(tokenTTL)
	s.mu.Lock()
	s.tokens[tok] = token{UserID: userID, Expires: expires}
	s.persist()
	s.mu.Unlock()
	return tok, expires
}

func (s *Store) Lookup(tok string) (*User, bool) {
	s.mu.RLock()
	entry, ok := s.tokens[tok]
	s.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.Expires) {
		s.Revoke(tok)
		return nil, false
	}
	s.mu.RLock()
	u := s.byID[entry.UserID]
	s.mu.RUnlock()
	if u == nil {
		return nil, false
	}
	return u, true
}

func (s *Store) Revoke(tok string) {
	s.mu.Lock()
	if _, ok := s.tokens[tok]; ok {
		delete(s.tokens, tok)
		s.persist()
	}
	s.mu.Unlock()
}
