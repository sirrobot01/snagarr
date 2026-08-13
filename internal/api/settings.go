package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/sirrobot01/snagarr/internal/arr"
	"github.com/sirrobot01/snagarr/internal/config"
)

// getSettings returns the live settings with every secret masked. The client
// echoes masked values back unchanged, and the manager restores the real ones.
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
// one value of a service almost always pins the rest, and a half-editable card
// is worse than a read-only one.
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
		"tmdb":      settings.TMDB.Configured(),
		"library":   settings.Library.Configured(),
		"radarr":    settings.Radarr.Configured(),
		"sonarr":    settings.Sonarr.Configured(),
		"overseerr": settings.Overseerr.Configured(),
		"ntfy":      settings.Ntfy.Configured(),
		"telegram":  settings.Telegram.Configured(),
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
	// New credentials usually mean new indexes to build.
	s.engine.Trigger()

	body, err := s.settingsBody(updated)
	if err != nil {
		s.log.Error("could not encode settings", "error", err)
		writeError(w, http.StatusInternalServerError, codeInternal, "settings could not be read")
		return
	}
	writeJSON(w, http.StatusOK, body)
}

// testService reports a service's reachability verbatim, so the settings card
// can show the upstream failure rather than a generic error.
func (s *Server) testService(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Service string `json:"service"`
	}
	if !decode(w, r, &req) {
		return
	}

	set := s.clients()
	var message string
	var err error

	switch req.Service {
	case "tmdb":
		message, err = pingOrUnconfigured(set.TMDB == nil, func() (string, error) { return set.TMDB.Ping(r.Context()) })
	case "library":
		message, err = pingOrUnconfigured(set.Library == nil, func() (string, error) { return set.Library.Ping(r.Context()) })
	case "radarr":
		message, err = pingOrUnconfigured(set.Radarr == nil, func() (string, error) { return set.Radarr.Ping(r.Context()) })
	case "sonarr":
		message, err = pingOrUnconfigured(set.Sonarr == nil, func() (string, error) { return set.Sonarr.Ping(r.Context()) })
	case "overseerr":
		message, err = pingOrUnconfigured(set.Overseerr == nil, func() (string, error) { return set.Overseerr.Ping(r.Context()) })
	case "ntfy":
		message, err = pingOrUnconfigured(set.Ntfy == nil, func() (string, error) { return "OK", set.Ntfy.Ping(r.Context()) })
	default:
		writeError(w, http.StatusBadRequest, codeBadRequest, "unknown service %q", req.Service)
		return
	}

	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": message})
}

// serviceOptions lets the settings UI offer real quality profiles, root folders
// and library sections instead of asking for numeric IDs.
func (s *Server) serviceOptions(w http.ResponseWriter, r *http.Request) {
	set := s.clients()
	ctx := r.Context()

	switch r.URL.Query().Get("service") {
	case "radarr":
		if set.Radarr == nil {
			writeError(w, http.StatusServiceUnavailable, codeNotConfigured, "Radarr is not configured")
			return
		}
		s.writeArrOptions(ctx, w, set.Radarr)
	case "sonarr":
		if set.Sonarr == nil {
			writeError(w, http.StatusServiceUnavailable, codeNotConfigured, "Sonarr is not configured")
			return
		}
		s.writeArrOptions(ctx, w, set.Sonarr)
	case "library":
		if set.Library == nil {
			writeError(w, http.StatusServiceUnavailable, codeNotConfigured, "no media server is configured")
			return
		}
		sections, err := set.Library.Sections(ctx)
		if err != nil {
			writeError(w, http.StatusBadGateway, codeUpstreamError, "%v", err)
			return
		}
		rows := make([]map[string]any, 0, len(sections))
		for _, sec := range sections {
			rows = append(rows, map[string]any{"id": sec.ID, "title": sec.Title, "type": sec.Type})
		}
		writeJSON(w, http.StatusOK, map[string]any{"sections": rows})
	default:
		writeError(w, http.StatusBadRequest, codeBadRequest, "service must be radarr, sonarr or library")
	}
}

// arrOptions is satisfied by both *arr.Radarr and *arr.Sonarr, which expose the
// same two lookups.
type arrOptions interface {
	QualityProfiles(ctx context.Context) ([]arr.Profile, error)
	RootFolders(ctx context.Context) ([]arr.RootFolder, error)
}

func (s *Server) writeArrOptions(ctx context.Context, w http.ResponseWriter, c arrOptions) {
	profiles, err := c.QualityProfiles(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, codeUpstreamError, "%v", err)
		return
	}
	folders, err := c.RootFolders(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, codeUpstreamError, "%v", err)
		return
	}

	profileRows := make([]map[string]any, 0, len(profiles))
	for _, p := range profiles {
		profileRows = append(profileRows, map[string]any{"id": p.ID, "name": p.Name})
	}
	folderRows := make([]map[string]any, 0, len(folders))
	for _, f := range folders {
		folderRows = append(folderRows, map[string]any{"path": f.Path, "free_space": f.FreeSpace})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"quality_profiles": profileRows,
		"root_folders":     folderRows,
	})
}

var errNotConfigured = errors.New("not configured")

func pingOrUnconfigured(unconfigured bool, ping func() (string, error)) (string, error) {
	if unconfigured {
		return "", errNotConfigured
	}
	return ping()
}
