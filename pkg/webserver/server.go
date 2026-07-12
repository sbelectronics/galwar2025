// Package webserver is the browser front-end: an xterm.js page speaking a
// line protocol over WebSocket to the same ConsoleUI that drives the local
// console and telnet. All game rules stay server-side; the browser is a dumb
// terminal.
package webserver

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/sbelectronics/galwar/pkg/galwar"
	"github.com/sbelectronics/galwar/pkg/ratelimit"
)

//go:embed static
var staticFiles embed.FS

type Config struct {
	Universe *galwar.UniverseType
	Store    *galwar.Store

	// BaseURL is the externally visible URL (scheme decides cookie
	// Secure); RedirectURL for OAuth is BaseURL + /auth/callback.
	BaseURL string

	// Google OIDC credentials. Empty ClientID disables Google login.
	GoogleClientID     string
	GoogleClientSecret string

	// DevAuth enables /auth/dev?user=email - a login backdoor for
	// development and tests. Never enable in production.
	DevAuth bool

	// IdleTimeout ends a game session with no input (default 15 minutes).
	IdleTimeout time.Duration
}

type Server struct {
	cfg       Config
	auth      *authenticator // nil when Google login is not configured
	mux       *http.ServeMux
	loginRate *ratelimit.Keyed // per-IP login throttle

	mu     sync.Mutex
	active map[galwar.PlayerId]*gameSession
}

func New(ctx context.Context, cfg Config) (*Server, error) {
	if cfg.IdleTimeout == 0 {
		cfg.IdleTimeout = 15 * time.Minute
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:8080"
	}
	s := &Server{
		cfg:       cfg,
		mux:       http.NewServeMux(),
		loginRate: ratelimit.NewKeyed(0.2, 10), // ~1 login attempt / 5s, burst 10, per IP
		active:    map[galwar.PlayerId]*gameSession{},
	}

	if cfg.GoogleClientID != "" {
		auth, err := newAuthenticator(ctx, cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.BaseURL)
		if err != nil {
			return nil, fmt.Errorf("initializing Google OIDC: %w", err)
		}
		s.auth = auth
	}

	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return nil, err
	}
	s.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	s.mux.HandleFunc("/", s.handleIndex)
	s.mux.HandleFunc("/auth/login", s.handleLogin)
	s.mux.HandleFunc("/auth/callback", s.handleCallback)
	s.mux.HandleFunc("/auth/logout", s.handleLogout)
	s.mux.HandleFunc("/auth/me", s.handleMe)
	s.mux.HandleFunc("/auth/dev", s.handleDevAuth)
	s.mux.HandleFunc("/ws", s.handleWS)
	return s, nil
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "missing index", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

// handleMe reports auth state to the page: 401 when not logged in,
// otherwise the player's handle (or email pre-registration).
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	sub, email, ok := s.sessionIdentity(r)
	if !ok {
		http.Error(w, "not logged in", http.StatusUnauthorized)
		return
	}
	name := email
	s.cfg.Universe.Do(func() {
		if p := s.lookupPlayer(sub, email); p != nil {
			name = p.GetName()
		}
	})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"name": name, "google": s.auth != nil, "dev": s.cfg.DevAuth})
}

// lookupPlayer resolves an auth identity to a player: by Google sub first,
// then by email for records that predate web auth (adopting the sub).
// Must run on the universe actor.
func (s *Server) lookupPlayer(sub, email string) *galwar.Player {
	u := s.cfg.Universe
	if p := u.Players.GetBySub(sub); p != nil {
		return p
	}
	if p := u.Players.GetByEmail(email); p != nil && p.GoogleSub == "" {
		p.GoogleSub = sub
		u.MarkDirty()
		return p
	}
	return nil
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	// default CheckOrigin: same-host only, which is what we want
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	sub, email, ok := s.sessionIdentity(r)
	if !ok {
		http.Error(w, "not logged in", http.StatusUnauthorized)
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade: %v", err)
		return
	}
	gs := newGameSession(s, conn, sub, email)
	go gs.run()
}

// attach registers a live session for a player, taking over from any
// existing one (the "my modem dropped" rule: newest connection wins).
func (s *Server) attach(id galwar.PlayerId, gs *gameSession) {
	s.mu.Lock()
	old := s.active[id]
	s.active[id] = gs
	s.mu.Unlock()
	if old != nil && old != gs {
		old.Printf("\nAnother session has taken over this account. Goodbye!\n")
		old.close()
	}
}

func (s *Server) detach(id galwar.PlayerId, gs *gameSession) {
	s.mu.Lock()
	if s.active[id] == gs {
		delete(s.active, id)
	}
	s.mu.Unlock()
}

// CloseAllSessions disconnects every live game session (server shutdown).
func (s *Server) CloseAllSessions() {
	s.mu.Lock()
	sessions := make([]*gameSession, 0, len(s.active))
	for _, gs := range s.active {
		sessions = append(sessions, gs)
	}
	s.mu.Unlock()
	for _, gs := range sessions {
		gs.Printf("\nServer is shutting down. Goodbye!\n")
		gs.close()
	}
}

func secureCookies(baseURL string) bool {
	return strings.HasPrefix(baseURL, "https://")
}
