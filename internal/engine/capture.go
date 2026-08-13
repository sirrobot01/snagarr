// Package engine is Snagarr's capture → resolve → reconcile pipeline.
//
// Capture reads the raw input of a snag — free text or a link — and extracts
// whatever identifies the title. Resolve turns that into a TMDB entry, or into
// a set of candidates when it cannot decide; it never discards a capture.
// Reconcile keeps the local indexes fresh and derives every item's state from
// them as set arithmetic against local tables, so the request path never waits
// on Plex, Radarr or Sonarr.
package engine

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/sirrobot01/snagarr/internal/store"
)

// Ref is what a link yielded. A TMDBID or IMDBID resolves directly; a bare
// Title falls back to search.
type Ref struct {
	TMDBID int64
	Type   store.MediaType
	IMDBID string
	Title  string
	Year   int
}

func (r Ref) Empty() bool { return r.TMDBID == 0 && r.IMDBID == "" && r.Title == "" }

var (
	tmdbPath   = regexp.MustCompile(`/(movie|tv)/(\d+)`)
	imdbPath   = regexp.MustCompile(`/title/(tt\d+)`)
	tmdbLink   = regexp.MustCompile(`themoviedb\.org/(movie|tv)/(\d+)`)
	imdbLink   = regexp.MustCompile(`imdb\.com/title/(tt\d+)`)
	metaTag    = regexp.MustCompile(`(?i)<meta[^>]+(?:property|name)=["'](og:title|twitter:title)["'][^>]+content=["']([^"']+)["']`)
	metaTagAlt = regexp.MustCompile(`(?i)<meta[^>]+content=["']([^"']+)["'][^>]+(?:property|name)=["'](og:title|twitter:title)["']`)
	titleTag   = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	jsonLDName = regexp.MustCompile(`"name"\s*:\s*"((?:[^"\\]|\\.)*)"`)
	yearInText = regexp.MustCompile(`\b(19|20)\d{2}\b`)
)

// IsURL reports whether the captured text should be treated as a link.
func IsURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		return false
	}
	u, err := url.Parse(raw)
	return err == nil && u.Host != ""
}

// ParseURL pulls an ID straight out of a link whose shape we know. Everything
// else needs Scrape.
func ParseURL(raw string) (Ref, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return Ref{}, false
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")

	switch {
	case host == "themoviedb.org" || strings.HasSuffix(host, ".themoviedb.org"):
		if m := tmdbPath.FindStringSubmatch(u.Path); m != nil {
			id, _ := strconv.ParseInt(m[2], 10, 64)
			return Ref{TMDBID: id, Type: mediaType(m[1])}, true
		}
	case host == "imdb.com" || strings.HasSuffix(host, ".imdb.com"):
		if m := imdbPath.FindStringSubmatch(u.Path); m != nil {
			return Ref{IMDBID: m[1]}, true
		}
	}
	return Ref{}, false
}

// Scrape fetches an unknown page and reads title hints out of it. Letterboxd
// and Trakt both link back to TMDB, so this usually returns an exact ID; other
// pages fall back to og:title, which search can still work with.
func Scrape(ctx context.Context, client *http.Client, raw string) (Ref, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return Ref{}, fmt.Errorf("scrape %s: %w", raw, err)
	}
	// Some sites serve a stub to unknown agents.
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; snagarr)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := client.Do(req)
	if err != nil {
		return Ref{}, fmt.Errorf("scrape %s: %w", raw, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return Ref{}, fmt.Errorf("scrape %s: status %d", raw, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Ref{}, fmt.Errorf("scrape %s: %w", raw, err)
	}
	return parseHints(string(body)), nil
}

func parseHints(body string) Ref {
	var ref Ref

	if m := tmdbLink.FindStringSubmatch(body); m != nil {
		id, _ := strconv.ParseInt(m[2], 10, 64)
		ref.TMDBID, ref.Type = id, mediaType(m[1])
	}
	if m := imdbLink.FindStringSubmatch(body); m != nil {
		ref.IMDBID = m[1]
	}

	ref.Title = cleanTitle(firstMatch(body,
		func(s string) string { return group(metaTag.FindStringSubmatch(s), 2) },
		func(s string) string { return group(metaTagAlt.FindStringSubmatch(s), 1) },
		func(s string) string { return group(jsonLDName.FindStringSubmatch(s), 1) },
		func(s string) string { return group(titleTag.FindStringSubmatch(s), 1) },
	))
	if year := yearInText.FindString(ref.Title); year != "" {
		ref.Year, _ = strconv.Atoi(year)
	}
	return ref
}

func firstMatch(body string, extractors ...func(string) string) string {
	for _, extract := range extractors {
		if v := strings.TrimSpace(extract(body)); v != "" {
			return v
		}
	}
	return ""
}

func group(match []string, n int) string {
	if len(match) > n {
		return match[n]
	}
	return ""
}

// cleanTitle drops the site branding most pages append to their title, so
// "Sinners (2025) • Letterboxd" searches as "Sinners (2025)".
func cleanTitle(title string) string {
	title = html.UnescapeString(title)
	for _, sep := range []string{" • ", " | ", " – ", " — ", " - "} {
		if i := strings.Index(title, sep); i > 0 {
			title = title[:i]
		}
	}
	return strings.TrimSpace(strings.Join(strings.Fields(title), " "))
}

func mediaType(s string) store.MediaType {
	if s == "tv" {
		return store.TV
	}
	return store.Movie
}
