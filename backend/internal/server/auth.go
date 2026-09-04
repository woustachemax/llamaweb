package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"llamaweb/internal/auth"
)

const cookieName = "llamaweb_session"

type ctxKey int

const userKey ctxKey = 0

func userFromContext(ctx context.Context) (*auth.User, bool) {
	u, ok := ctx.Value(userKey).(*auth.User)
	return u, ok
}

func bearerToken(r *http.Request) string {
	if c, err := r.Cookie(cookieName); err == nil && c.Value != "" {
		return c.Value
	}
	h := r.Header.Get("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "Bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tok := bearerToken(r)
		if tok == "" {
			writeErr(w, http.StatusUnauthorized, "authentication required")
			return
		}
		user, ok := s.auth.Lookup(tok)
		if !ok {
			writeErr(w, http.StatusUnauthorized, "session expired")
			return
		}
		ctx := context.WithValue(r.Context(), userKey, user)
		next(w, r.WithContext(ctx))
	}
}

func (s *Server) setSessionCookie(w http.ResponseWriter, tok string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    tok,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func authStatus(err error) int {
	switch {
	case errors.Is(err, auth.ErrEmailTaken):
		return http.StatusConflict
	case errors.Is(err, auth.ErrInvalidCredentials):
		return http.StatusUnauthorized
	default:
		return http.StatusBadRequest
	}
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	user, err := s.auth.Register(c.Email, c.Password)
	if err != nil {
		writeErr(w, authStatus(err), err.Error())
		return
	}
	tok, expires := s.auth.Mint(user.ID)
	s.setSessionCookie(w, tok, expires)
	writeJSON(w, http.StatusOK, map[string]any{"user": user.Public(), "token": tok})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	user, err := s.auth.Authenticate(c.Email, c.Password)
	if err != nil {
		writeErr(w, authStatus(err), err.Error())
		return
	}
	tok, expires := s.auth.Mint(user.ID)
	s.setSessionCookie(w, tok, expires)
	writeJSON(w, http.StatusOK, map[string]any{"user": user.Public(), "token": tok})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if tok := bearerToken(r); tok != "" {
		s.auth.Revoke(tok)
	}
	s.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user, _ := userFromContext(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"user": user.Public()})
}
