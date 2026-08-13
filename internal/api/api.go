// Package api exposes the HTTP surface. The API is the product: the web client
// is one consumer of it, alongside the Shortcut, the bookmarklet and the bot.
package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/sirrobot01/snagarr/internal/config"
	"github.com/sirrobot01/snagarr/internal/reconcile"
	"github.com/sirrobot01/snagarr/internal/resolver"
	"github.com/sirrobot01/snagarr/internal/store"
)

type Server struct {
	store    *store.Store
	settings *config.Manager
	resolver *resolver.Resolver
	engine   *reconcile.Engine
	web      http.Handler
	log      *slog.Logger
}

func New(st *store.Store, settings *config.Manager, res *resolver.Resolver,
	engine *reconcile.Engine, web http.Handler, log *slog.Logger) *Server {
	return &Server{store: st, settings: settings, resolver: res, engine: engine, web: web, log: log}
}

func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.Recoverer, s.logRequests)

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/health", s.health)
		// Webhook senders authenticate with a shared secret in the query
		// string, because Radarr, Tautulli and Emby cannot all set headers.
		r.Post("/webhooks/{service}", s.webhook)

		r.Group(func(r chi.Router) {
			r.Use(s.authenticate)

			r.Get("/me", s.me)
			r.Get("/status", s.status)
			r.Post("/capture", s.capture)
			r.Get("/search", s.search)
			r.Get("/items", s.listItems)
			r.Get("/items/{id}", s.getItem)
			r.Post("/items/{id}/resolve", s.resolveItem)
			r.Post("/items/{id}/archive", s.archiveItem)
			r.Delete("/items/{id}", s.deleteItem)

			r.Group(func(r chi.Router) {
				r.Use(requireAdmin)

				r.Post("/items/{id}/send", s.sendItem)
				r.Get("/users", s.listUsers)
				r.Post("/users", s.createUser)
				r.Patch("/users/{id}", s.updateUser)
				r.Delete("/users/{id}", s.deleteUser)
				r.Get("/users/{id}/tokens", s.listTokens)
				r.Post("/users/{id}/tokens", s.createToken)
				r.Delete("/tokens/{id}", s.revokeToken)
				r.Get("/settings", s.getSettings)
				r.Put("/settings", s.putSettings)
				r.Post("/settings/test", s.testService)
				r.Get("/settings/options", s.serviceOptions)
				r.Post("/admin/sync", s.forceSync)
			})
		})
	})

	r.Mount("/", s.web)
	return r
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": buildVersion})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	writeJSON(w, http.StatusOK, userRef{ID: u.ID, DisplayName: u.DisplayName, Role: u.Role})
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	counts, err := s.store.Counts(r.Context())
	if err != nil {
		s.writeStoreError(w, err, "counts")
		return
	}
	settings := s.settings.Get()
	writeJSON(w, http.StatusOK, map[string]any{
		"version": buildVersion,
		"counts":  counts,
		"sync":    s.engine.Status(),
		"services": map[string]bool{
			"tmdb":      settings.TMDB.Configured(),
			"library":   settings.Library.Configured(),
			"radarr":    settings.Radarr.Configured(),
			"sonarr":    settings.Sonarr.Configured(),
			"overseerr": settings.Overseerr.Configured(),
			"ntfy":      settings.Ntfy.Configured(),
			"telegram":  settings.Telegram.Configured(),
		},
	})
}

func (s *Server) forceSync(w http.ResponseWriter, _ *http.Request) {
	s.engine.Trigger()
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		wrapped := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(wrapped, r)

		// Static asset noise would drown the log; the API is what matters.
		if !isAPIPath(r.URL.Path) {
			return
		}
		s.log.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.Status(),
			"took", time.Since(started).Round(time.Millisecond),
		)
	})
}

func isAPIPath(path string) bool { return len(path) >= 8 && path[:8] == "/api/v1/" }

// buildVersion is set from cmd so package api does not depend on the version
// package purely to echo a string.
var buildVersion = "dev"

// SetVersion records the running version for the health and status endpoints.
func SetVersion(v string) { buildVersion = v }
