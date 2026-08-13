package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// A generated Shortcut carries a live bearer token, and the operator will paste
// its link into a chat app. The link is therefore single-use and short-lived,
// rather than a stable authenticated URL.
const shortcutLinkTTL = 15 * time.Minute

const shortcutCachePrefix = "shortcut:"

// createShortcutLink issues a token for a household member and hands back a
// download link for a Shortcut with that token already inside it. The member
// never has to build one by hand.
func (s *Server) createShortcutLink(w http.ResponseWriter, r *http.Request) {
	u, ok := s.loadUser(w, r)
	if !ok {
		return
	}

	_, secret, err := s.store.CreateToken(r.Context(), u.ID, "Apple Shortcut")
	if err != nil {
		s.writeStoreError(w, err, "token")
		return
	}

	key := make([]byte, 24)
	if _, err := rand.Read(key); err != nil {
		writeError(w, http.StatusInternalServerError, codeInternal, "could not generate a link")
		return
	}
	handle := hex.EncodeToString(key)
	if err := s.store.HTTPCache().Set(r.Context(), shortcutCachePrefix+handle, []byte(secret), shortcutLinkTTL); err != nil {
		s.writeStoreError(w, err, "shortcut link")
		return
	}

	base := s.publicBase(r)
	download := base + "/api/v1/shortcut/" + handle
	writeJSON(w, http.StatusCreated, map[string]any{
		"url":        download,
		"import_url": "shortcuts://import-shortcut/?url=" + url.QueryEscape(download) + "&name=Snag",
		"expires_at": time.Now().UTC().Add(shortcutLinkTTL),
		// The phone fetches the download URL itself, so a LAN-only or plain-http
		// host fails. The UI warns on this.
		"public": strings.HasPrefix(base, "https://"),
	})
}

// serveShortcut returns the generated Shortcut file. The link itself is the
// credential, so this route carries no bearer auth; it is consumed on first use.
func (s *Server) serveShortcut(w http.ResponseWriter, r *http.Request) {
	handle := chi.URLParam(r, "handle")
	secret, ok := s.store.HTTPCache().Get(r.Context(), shortcutCachePrefix+handle)
	if !ok {
		writeError(w, http.StatusNotFound, codeNotFound, "this link has expired or was already used")
		return
	}
	if err := s.store.HTTPCache().Delete(r.Context(), shortcutCachePrefix+handle); err != nil {
		s.log.Warn("could not consume shortcut link", "error", err)
	}

	body, err := buildShortcutFile(shortcutOptions{
		BaseURL: s.publicBase(r),
		Token:   string(secret),
		Name:    defaultShortcutName,
	})
	if err != nil {
		s.log.Error("could not build shortcut", "error", err)
		writeError(w, http.StatusInternalServerError, codeInternal, "could not build the shortcut")
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+defaultShortcutName+`.shortcut"`)
	w.Header().Set("Cache-Control", "no-store")
	if _, err := w.Write(body); err != nil {
		s.log.Warn("could not write shortcut", "error", err)
	}
}

// publicBase prefers the configured public URL, because the phone resolves the
// Shortcut download link itself and the request Host is often an internal name.
func (s *Server) publicBase(r *http.Request) string {
	if configured := strings.TrimRight(s.settings.Get().General.PublicURL, "/"); configured != "" {
		return configured
	}
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}
