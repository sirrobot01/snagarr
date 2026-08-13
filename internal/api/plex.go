package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/sirrobot01/snagarr/internal/library"
	"github.com/sirrobot01/snagarr/internal/store"
	"github.com/sirrobot01/snagarr/internal/version"
)

// plexClientIDKey holds the identifier plex.tv ties issued tokens to. It must
// survive restarts: regenerating it invalidates every token already granted.
const plexClientIDKey = "plex_client_id"

func (s *Server) plexAuth(ctx context.Context) (*library.PlexAuth, error) {
	id, err := s.store.Setting(ctx, plexClientIDKey)
	if err == nil && len(id) > 0 {
		return library.NewPlexAuth(string(id), version.Version), nil
	}
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}

	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	fresh := hex.EncodeToString(buf)
	if err := s.store.SetSetting(ctx, plexClientIDKey, []byte(fresh)); err != nil {
		return nil, err
	}
	return library.NewPlexAuth(fresh, version.Version), nil
}

// createPlexPin starts the plex.tv sign-in flow, so an operator never has to
// find an X-Plex-Token by hand.
func (s *Server) createPlexPin(w http.ResponseWriter, r *http.Request) {
	auth, err := s.plexAuth(r.Context())
	if err != nil {
		s.writeStoreError(w, err, "plex client id")
		return
	}
	pin, err := auth.CreatePin(r.Context(), s.settings.Get().General.PublicURL)
	if err != nil {
		writeError(w, http.StatusBadGateway, codeUpstreamError, "plex.tv rejected the request: %v", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": pin.ID, "code": pin.Code, "auth_url": pin.AuthURL, "expires_at": pin.ExpiresAt,
	})
}

// checkPlexPin is polled by the browser while the user signs in.
func (s *Server) checkPlexPin(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, codeBadRequest, "pin id must be a number")
		return
	}
	auth, err := s.plexAuth(r.Context())
	if err != nil {
		s.writeStoreError(w, err, "plex client id")
		return
	}

	token, err := auth.CheckPin(r.Context(), id)
	switch {
	case errors.Is(err, library.ErrPinPending):
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "pending"})
	case errors.Is(err, library.ErrPinExpired):
		writeError(w, http.StatusGone, codeConflict, "the sign-in expired; start again")
	case err != nil:
		writeError(w, http.StatusBadGateway, codeUpstreamError, "plex.tv rejected the request: %v", err)
	default:
		writeJSON(w, http.StatusOK, map[string]string{"status": "linked", "token": token})
	}
}

// listPlexServers turns a fresh token into a list of reachable servers, so the
// user picks one instead of typing a URL.
func (s *Server) listPlexServers(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, codeBadRequest, "token is required")
		return
	}
	auth, err := s.plexAuth(r.Context())
	if err != nil {
		s.writeStoreError(w, err, "plex client id")
		return
	}
	resources, err := auth.Resources(r.Context(), token)
	if err != nil {
		writeError(w, http.StatusBadGateway, codeUpstreamError, "plex.tv rejected the request: %v", err)
		return
	}

	servers := make([]map[string]any, 0, len(resources))
	for _, res := range resources {
		connections := make([]map[string]any, 0, len(res.Connections))
		for _, c := range res.Connections {
			connections = append(connections, map[string]any{"uri": c.URI, "local": c.Local, "relay": c.Relay})
		}
		servers = append(servers, map[string]any{
			"name": res.Name, "client_identifier": res.ClientIdentifier, "connections": connections,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"servers": servers})
}
