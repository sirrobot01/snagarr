package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/sirrobot01/snagarr/internal/config"
)

// getSettings returns the live settings with every secret masked. The client
// echoes masked values back unchanged, and the manager restores the real ones.
//
// Settings are what stays global once every integration belongs to a member:
// the TMDB key and the install-wide knobs. Everything else is a service.
func (s *Server) getSettings(w http.ResponseWriter, _ *http.Request) {
	body, err := s.settingsBody(s.settings.Get())
	if err != nil {
		s.log.Error("could not encode settings", "error", err)
		writeError(w, http.StatusInternalServerError, codeInternal, "settings could not be read")
		return
	}
	writeJSON(w, http.StatusOK, body)
}

// settingsBody decorates each section with the two flags the settings UI needs:
// whether it has enough to work, and whether an environment variable pins it.
//
// Locking is reported per section rather than per field. An operator who pins
// one value almost always pins the rest, and a half-editable card is worse than
// a read-only one.
func (s *Server) settingsBody(settings config.Settings) (map[string]any, error) {
	raw, err := json.Marshal(settings.Masked())
	if err != nil {
		return nil, err
	}
	var sections map[string]map[string]any
	if err := json.Unmarshal(raw, &sections); err != nil {
		return nil, err
	}

	configured := map[string]bool{
		"tmdb": settings.TMDB.Configured(),
		// General always works: the interval has a default and the webhook
		// secret is generated on first run.
		"general": true,
	}
	locked := map[string]bool{}
	for path := range s.settings.Locked() {
		section, _, _ := strings.Cut(path, ".")
		locked[section] = true
	}

	body := make(map[string]any, len(sections))
	for name, section := range sections {
		section["configured"] = configured[name]
		section["locked"] = locked[name]
		body[name] = section
	}
	return body, nil
}

func (s *Server) putSettings(w http.ResponseWriter, r *http.Request) {
	patch, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, codeBadRequest, "could not read the request body")
		return
	}
	if !json.Valid(patch) {
		writeError(w, http.StatusBadRequest, codeBadRequest, "the request body is not valid JSON")
		return
	}

	updated, err := s.settings.Apply(r.Context(), patch)
	if err != nil {
		writeError(w, http.StatusBadRequest, codeBadRequest, "%v", err)
		return
	}
	// A new TMDB key usually means captures waiting to be resolved.
	s.engine.Trigger()

	body, err := s.settingsBody(updated)
	if err != nil {
		s.log.Error("could not encode settings", "error", err)
		writeError(w, http.StatusInternalServerError, codeInternal, "settings could not be read")
		return
	}
	writeJSON(w, http.StatusOK, body)
}

// testSettings pings TMDB. Every other service is tested through
// /services/{id}/test; TMDB is global, so it has no service record to hang off.
func (s *Server) testSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Service string `json:"service"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Service != "tmdb" {
		writeError(w, http.StatusBadRequest, codeBadRequest,
			"service must be tmdb; test the rest through /services/{id}/test")
		return
	}

	catalogue := s.tmdb()
	if catalogue == nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": "not configured"})
		return
	}
	message, err := catalogue.Ping(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": message})
}
