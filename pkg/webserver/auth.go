package webserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/http"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// clientIP extracts the request's source IP for per-IP rate limiting.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// loginThrottled writes a 429 and returns true when the client IP has
// exceeded the login rate.
func (s *Server) loginThrottled(w http.ResponseWriter, r *http.Request) bool {
	if !s.loginRate.Allow(clientIP(r)) {
		http.Error(w, "too many attempts; slow down", http.StatusTooManyRequests)
		return true
	}
	return false
}

// "Log in with Google" via standard OIDC authorization-code flow. The
// account key is the ID token's stable `sub` claim, never the email (emails
// change). Our own session is a random token in an HttpOnly cookie, mapped
// server-side in the sessions table.

const (
	sessionCookie = "galwar_session"
	stateCookie   = "galwar_oauth_state"
	sessionTTL    = 30 * 24 * time.Hour
)

type authenticator struct {
	config   oauth2.Config
	verifier *oidc.IDTokenVerifier
}

func newAuthenticator(ctx context.Context, clientID, clientSecret, baseURL string) (*authenticator, error) {
	provider, err := oidc.NewProvider(ctx, "https://accounts.google.com")
	if err != nil {
		return nil, err
	}
	return &authenticator{
		config: oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  baseURL + "/auth/callback",
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
		},
		verifier: provider.Verifier(&oidc.Config{ClientID: clientID}),
	}, nil
}

func randToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand failure is not a recoverable condition
	}
	return hex.EncodeToString(b)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if s.loginThrottled(w, r) {
		return
	}
	if s.auth == nil {
		http.Error(w, "Google login is not configured on this server", http.StatusServiceUnavailable)
		return
	}
	state := randToken()
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookie,
		Value:    state,
		Path:     "/",
		MaxAge:   600,
		HttpOnly: true,
		Secure:   secureCookies(s.cfg.BaseURL),
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, s.auth.config.AuthCodeURL(state), http.StatusFound)
}

func (s *Server) handleCallback(w http.ResponseWriter, r *http.Request) {
	if s.loginThrottled(w, r) {
		return
	}
	if s.auth == nil {
		http.Error(w, "Google login is not configured on this server", http.StatusServiceUnavailable)
		return
	}
	stateC, err := r.Cookie(stateCookie)
	if err != nil || stateC.Value == "" || r.URL.Query().Get("state") != stateC.Value {
		http.Error(w, "state mismatch", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	token, err := s.auth.config.Exchange(ctx, r.URL.Query().Get("code"))
	if err != nil {
		http.Error(w, "code exchange failed", http.StatusBadGateway)
		return
	}
	rawID, ok := token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "no id_token in response", http.StatusBadGateway)
		return
	}
	idToken, err := s.auth.verifier.Verify(ctx, rawID)
	if err != nil {
		http.Error(w, "id_token verification failed", http.StatusUnauthorized)
		return
	}
	var claims struct {
		Sub   string `json:"sub"`
		Email string `json:"email"`
	}
	// both are required: sub is the account key, and an empty email would
	// create an unidentifiable account and muddy the legacy-email adoption
	// path. Google's "email" scope reliably provides both.
	if err := idToken.Claims(&claims); err != nil || claims.Sub == "" || claims.Email == "" {
		http.Error(w, "bad claims", http.StatusBadGateway)
		return
	}
	if err := s.createSession(w, claims.Sub, claims.Email); err != nil {
		http.Error(w, "session creation failed", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

// handleDevAuth is the development backdoor: /auth/dev?user=email logs in
// as a synthetic identity. Only active with Config.DevAuth.
func (s *Server) handleDevAuth(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.DevAuth {
		http.NotFound(w, r)
		return
	}
	if s.loginThrottled(w, r) {
		return
	}
	email := r.URL.Query().Get("user")
	if email == "" {
		http.Error(w, "missing ?user=", http.StatusBadRequest)
		return
	}
	if err := s.createSession(w, "dev:"+email, email); err != nil {
		http.Error(w, "session creation failed", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		s.cfg.Store.DeleteSession(c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secureCookies(s.cfg.BaseURL),
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Server) createSession(w http.ResponseWriter, sub, email string) error {
	token := randToken()
	expires := time.Now().Add(sessionTTL)
	if err := s.cfg.Store.CreateSession(token, sub, email, expires.Unix()); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   secureCookies(s.cfg.BaseURL),
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

// sessionIdentity resolves the request's session cookie to an auth identity.
func (s *Server) sessionIdentity(r *http.Request) (sub string, email string, ok bool) {
	c, err := r.Cookie(sessionCookie)
	if err != nil || c.Value == "" {
		return "", "", false
	}
	sub, email, ok, err = s.cfg.Store.GetSession(c.Value)
	if err != nil || !ok {
		return "", "", false
	}
	return sub, email, true
}
