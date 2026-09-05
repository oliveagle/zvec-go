package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/oliveagle/zvec-go"
)

// ServiceVersion is the version reported by /healthz and in logs.
const ServiceVersion = "0.1.0"

// Server is a lightweight zvec HTTP service.
type Server struct {
	cfg    *Config
	mgr    *CollectionManager
	auth   *Authenticator
	log    *slog.Logger
	http   *http.Server
}

// New constructs a Server from cfg. It initializes the zvec client, prepares
// the storage directory, opens any existing collections, and builds the
// authenticator. log may be nil (slog.Default is used).
func New(cfg *Config, log *slog.Logger) (*Server, error) {
	if log == nil {
		log = slog.Default()
	}
	if cfg == nil {
		cfg = DefaultConfig()
	}
	cfg.fillDefaults()

	if err := zvec.Init(nil); err != nil {
		return nil, fmt.Errorf("initialize zvec: %w", err)
	}

	mgr, err := NewCollectionManager(cfg.Storage.DataDir)
	if err != nil {
		return nil, err
	}
	if err := mgr.OpenExisting(); err != nil {
		log.Warn("failed to open existing collections", "err", err)
	}
	log.Info("opened existing collections", "count", mgr.Count(), "data_dir", cfg.Storage.DataDir)

	var auth *Authenticator
	if cfg.Auth.Enabled {
		var err error
		auth, err = NewAuthenticator(cfg.Auth.Users)
		if err != nil {
			return nil, fmt.Errorf("configure auth: %w", err)
		}
		for _, u := range cfg.Auth.Users {
			if !strings.HasPrefix(u.Password, "sha256:") {
				log.Warn("user uses a plain-text password; use \"sha256:<hash>\" in production", "user", u.Username)
			}
		}
		log.Info("authentication enabled", "users", len(cfg.Auth.Users))
	} else {
		log.Warn("authentication is DISABLED; do not expose this service to untrusted networks")
	}

	s := &Server{cfg: cfg, mgr: mgr, auth: auth, log: log}
	s.http = &http.Server{
		Addr:         cfg.Server.Addr,
		Handler:      s.routes(),
		ReadTimeout:  cfg.ReadTimeout(),
		WriteTimeout: cfg.WriteTimeout(),
	}
	return s, nil
}

// nameOnly trims surrounding whitespace from a name (the strict charset is
// enforced separately by ValidateCollectionName).
func nameOnly(s string) string { return strings.TrimSpace(s) }

// routes builds the HTTP routing table. /healthz is public; /api/v1/* requires
// Basic auth when enabled.
func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)

	mux.HandleFunc("GET /api/v1/collections", s.requireAuth(s.handleListCollections))
	mux.HandleFunc("POST /api/v1/collections", s.requireAuth(s.handleCreateCollection))
	mux.HandleFunc("GET /api/v1/collections/{name}", s.requireAuth(s.handleGetCollection))
	mux.HandleFunc("DELETE /api/v1/collections/{name}", s.requireAuth(s.handleDropCollection))

	mux.HandleFunc("GET /api/v1/collections/{name}/documents", s.requireAuth(s.handleListDocuments))
	mux.HandleFunc("POST /api/v1/collections/{name}/documents", s.requireAuth(s.handleUpsertDocuments))
	mux.HandleFunc("GET /api/v1/collections/{name}/documents/{id}", s.requireAuth(s.handleGetDocument))
	mux.HandleFunc("DELETE /api/v1/collections/{name}/documents/{id}", s.requireAuth(s.handleDeleteDocument))
	mux.HandleFunc("POST /api/v1/collections/{name}/query", s.requireAuth(s.handleQuery))

	mux.HandleFunc("/", s.handleRoot)
	return mux
}

// Start runs the HTTP server until it returns an error or is shut down.
func (s *Server) Start() error {
	s.log.Info("starting zvec HTTP service", "addr", s.cfg.Server.Addr, "version", ServiceVersion)
	err := s.http.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Shutdown gracefully shuts the server down within the configured timeout.
func (s *Server) Shutdown(ctx context.Context) error {
	if err := s.http.Shutdown(ctx); err != nil {
		return err
	}
	s.mgr.Close()
	s.log.Info("shutdown complete")
	return nil
}

// Handler exposes the HTTP handler (useful for embedding or testing).
func (s *Server) Handler() http.Handler { return s.routes() }
