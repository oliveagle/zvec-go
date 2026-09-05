package server

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// ErrUnauthorized is returned when credentials are missing or invalid.
var ErrUnauthorized = errors.New("unauthorized")

type userEntry struct {
	username string
	readonly bool
	hashed   bool
	hash     [32]byte
	password string // only used when !hashed
}

// Authenticator validates HTTP Basic credentials against a configured user list.
type Authenticator struct {
	users map[string]*userEntry
}

// NewAuthenticator builds an Authenticator from a list of users. It returns an
// error if the user list is empty, if usernames are duplicated, or if a sha256
// password is malformed.
func NewAuthenticator(users []User) (*Authenticator, error) {
	a := &Authenticator{users: make(map[string]*userEntry, len(users))}
	for _, u := range users {
		name := strings.TrimSpace(u.Username)
		if name == "" {
			return nil, errors.New("auth: user with empty username")
		}
		if _, dup := a.users[name]; dup {
			return nil, fmt.Errorf("auth: duplicate username %q", name)
		}
		entry := &userEntry{username: name, readonly: u.Readonly}
		if strings.HasPrefix(u.Password, "sha256:") {
			raw := strings.TrimSpace(strings.TrimPrefix(u.Password, "sha256:"))
			b, err := hex.DecodeString(raw)
			if err != nil || len(b) != 32 {
				return nil, fmt.Errorf("auth: user %q has malformed sha256 password (need 64 hex chars)", name)
			}
			copy(entry.hash[:], b)
			entry.hashed = true
		} else {
			entry.password = u.Password
		}
		a.users[name] = entry
	}
	if len(a.users) == 0 {
		return nil, errors.New("auth: no users configured")
	}
	return a, nil
}

// Authenticate checks the username/password pair. It reports whether the user
// is readonly and whether authentication succeeded.
func (a *Authenticator) Authenticate(username, password string) (readonly bool, ok bool) {
	e, exists := a.users[username]
	if !exists {
		// Burn constant-ish time even for unknown users.
		_ = sha256.Sum256([]byte(password))
		return false, false
	}
	if e.hashed {
		sum := sha256.Sum256([]byte(password))
		if subtle.ConstantTimeCompare(sum[:], e.hash[:]) != 1 {
			return false, false
		}
	} else {
		if subtle.ConstantTimeCompare([]byte(password), []byte(e.password)) != 1 {
			return false, false
		}
	}
	return e.readonly, true
}

type ctxKey int

const readonlyKey ctxKey = iota

// requireAuth wraps an HTTP handler with HTTP Basic authentication. When the
// authenticator is nil, the handler runs unauthenticated.
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.auth == nil {
			next(w, r)
			return
		}
		username, password, ok := r.BasicAuth()
		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="zvec"`)
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		readonly, ok := s.auth.Authenticate(username, password)
		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="zvec"`)
			writeError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), readonlyKey, readonly))
		next(w, r)
	}
}

// isReadonly reports whether the current request's authenticated user is readonly.
func isReadonly(r *http.Request) bool {
	v, _ := r.Context().Value(readonlyKey).(bool)
	return v
}
