package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/sirrobot01/snagarr/internal/store"
)

// Webhooks always answer 204, including for payloads that match nothing. A
// sender that sees an error retries, and none of these events are worth a retry
// storm — the reconcile loop is the backstop for anything missed here.
func (s *Server) webhook(w http.ResponseWriter, r *http.Request) {
	if !s.webhookAuthorized(r) {
		w.Header().Set("WWW-Authenticate", `Basic realm="snagarr"`)
		writeError(w, http.StatusUnauthorized, codeUnauthorized,
			"send a household username and password, or a bearer token")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	service := chi.URLParam(r, "service")
	switch service {
	case "radarr", "sonarr":
		s.handleImport(r, service, body)
	case "tautulli", "emby", "jellyfin":
		s.handlePlayback(r, service, body)
	default:
		writeError(w, http.StatusNotFound, codeNotFound, "unknown webhook %q", service)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// arrEvent covers the subset of the Radarr and Sonarr payloads Snagarr needs.
type arrEvent struct {
	EventType string `json:"eventType"`
	Movie     struct {
		TMDBID int `json:"tmdbId"`
	} `json:"movie"`
	Series struct {
		TMDBID int `json:"tmdbId"`
		TVDBID int `json:"tvdbId"`
	} `json:"series"`
}

func (s *Server) handleImport(r *http.Request, service string, body []byte) {
	var event arrEvent
	if err := json.Unmarshal(body, &event); err != nil {
		s.log.Warn("could not read webhook payload", "service", service, "error", err)
		return
	}

	switch event.EventType {
	case "Download", "MovieFileImported", "EpisodeFileImported", "Import":
	default:
		return
	}

	tmdbID, mediaType := int64(event.Movie.TMDBID), store.Movie
	if service == "sonarr" {
		tmdbID, mediaType = int64(event.Series.TMDBID), store.TV
	}
	if tmdbID == 0 {
		s.log.Debug("webhook carried no tmdb id", "service", service, "event", event.EventType)
		return
	}
	if err := s.reconciler.MarkAvailable(r.Context(), tmdbID, mediaType); err != nil {
		s.log.Warn("could not apply import webhook", "service", service, "error", err)
	}
}

// playbackEvent spans Tautulli, Emby and Jellyfin. None of them agree on field
// names, so every known spelling is accepted and the first non-empty one wins.
type playbackEvent struct {
	Event  string `json:"event"`
	Action string `json:"action"`

	MediaType string `json:"media_type"`
	TMDBID    string `json:"tmdb_id"`

	Item struct {
		Type        string            `json:"Type"`
		ProviderIDs map[string]string `json:"ProviderIds"`
	} `json:"Item"`
}

func (s *Server) handlePlayback(r *http.Request, service string, body []byte) {
	var event playbackEvent
	if err := json.Unmarshal(body, &event); err != nil {
		s.log.Warn("could not read webhook payload", "service", service, "error", err)
		return
	}

	tmdbID := parseTMDBID(event.TMDBID)
	mediaType := store.Movie
	if event.MediaType == "show" || event.MediaType == "episode" || event.Item.Type == "Episode" || event.Item.Type == "Series" {
		mediaType = store.TV
	}
	if tmdbID == 0 {
		for name, value := range event.Item.ProviderIDs {
			if len(name) == 4 && (name == "Tmdb" || name == "tmdb" || name == "TMDB") {
				tmdbID = parseTMDBID(value)
				break
			}
		}
	}
	if tmdbID == 0 {
		s.log.Debug("playback webhook carried no tmdb id", "service", service)
		return
	}

	if err := s.reconciler.MarkWatched(r.Context(), tmdbID, mediaType, service); err != nil {
		s.log.Warn("could not apply playback webhook", "service", service, "error", err)
	}
}

func parseTMDBID(v string) int64 {
	id, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0
	}
	return id
}
