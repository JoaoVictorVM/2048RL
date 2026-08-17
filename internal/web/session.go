package web

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/JoaoVictorVM/2048RL/internal/game"
)

const (
	DefaultSessionTTL    = 30 * time.Minute
	sessionSweepInterval = time.Minute
)

type Session struct {
	Token string
	Game  *game.Game

	mu           sync.Mutex
	lastActiveAt time.Time
}

func (s *Session) Lock() { s.mu.Lock() }

func (s *Session) Unlock() { s.mu.Unlock() }

type SessionStore struct {
	mu       sync.Mutex
	sessions map[string]*Session
	ttl      time.Duration
	now      func() time.Time
}

func NewSessionStore(ttl time.Duration) *SessionStore {
	if ttl <= 0 {
		ttl = DefaultSessionTTL
	}
	return &SessionStore{
		sessions: map[string]*Session{},
		ttl:      ttl,
		now:      time.Now,
	}
}

func (s *SessionStore) Create(g *game.Game) (*Session, error) {
	token, err := newSessionToken()
	if err != nil {
		return nil, err
	}

	session := &Session{Token: token, Game: g, lastActiveAt: s.now()}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[token] = session
	return session, nil
}

func (s *SessionStore) Get(token string) (*Session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[token]
	if !ok {
		return nil, false
	}
	if s.expired(session) {
		delete(s.sessions, token)
		return nil, false
	}

	session.lastActiveAt = s.now()
	return session, true
}

func (s *SessionStore) Sweep() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	removed := 0
	for token, session := range s.sessions {
		if s.expired(session) {
			delete(s.sessions, token)
			removed++
		}
	}
	return removed
}

func (s *SessionStore) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sessions)
}

func (s *SessionStore) expired(session *Session) bool {
	return s.now().Sub(session.lastActiveAt) > s.ttl
}

// UUID v4 gerado com crypto/rand: o token identifica a sessão de quem está
// jogando, então não pode ser previsível mesmo sem autenticação.
func newSessionToken() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("web: não foi possível gerar o token de sessão: %w", err)
	}
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80

	encoded := hex.EncodeToString(buf[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", encoded[0:8], encoded[8:12], encoded[12:16], encoded[16:20], encoded[20:32]), nil
}
