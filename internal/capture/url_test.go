package capture

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirrobot01/snagarr/internal/media"
)

func TestParseURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want Ref
		ok   bool
	}{
		{"tmdb movie with slug", "https://www.themoviedb.org/movie/1233413-sinners", Ref{TMDBID: 1233413, Type: media.Movie}, true},
		{"tmdb tv", "https://themoviedb.org/tv/95396", Ref{TMDBID: 95396, Type: media.TV}, true},
		{"tmdb with query", "https://www.themoviedb.org/movie/693134?language=en", Ref{TMDBID: 693134, Type: media.Movie}, true},
		{"imdb", "https://www.imdb.com/title/tt31193180/", Ref{IMDBID: "tt31193180"}, true},
		{"imdb mobile", "https://m.imdb.com/title/tt0087182/", Ref{IMDBID: "tt0087182"}, true},
		{"letterboxd needs scraping", "https://letterboxd.com/film/sinners-2025/", Ref{}, false},
		{"not a media url", "https://example.com/post/1", Ref{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseURL(tt.raw)
			if ok != tt.ok || got != tt.want {
				t.Errorf("ParseURL(%q) = %+v, %v; want %+v, %v", tt.raw, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestIsURL(t *testing.T) {
	for raw, want := range map[string]bool{
		"https://letterboxd.com/film/sinners": true,
		"http://plex.lan:32400":               true,
		"sinners":                             false,
		"the one with the whale":              false,
		"https://":                            false,
	} {
		if got := IsURL(raw); got != want {
			t.Errorf("IsURL(%q) = %v, want %v", raw, got, want)
		}
	}
}

func TestScrapeExtractsTMDBLinkAndTitle(t *testing.T) {
	page := `<!doctype html><html><head>
		<meta property="og:title" content="Sinners (2025) &bull; Letterboxd">
		<title>Sinners (2025) directed by Ryan Coogler</title>
		</head><body>
		<a href="https://www.themoviedb.org/movie/1233413/">TMDb</a>
		<a href="https://www.imdb.com/title/tt31193180/">IMDb</a>
		</body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(page))
	}))
	defer srv.Close()

	got, err := Scrape(context.Background(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("Scrape: %v", err)
	}
	if got.TMDBID != 1233413 || got.Type != media.Movie {
		t.Errorf("TMDB ref = %d/%s, want 1233413/movie", got.TMDBID, got.Type)
	}
	if got.IMDBID != "tt31193180" {
		t.Errorf("IMDB ref = %q, want tt31193180", got.IMDBID)
	}
	if got.Title != "Sinners (2025)" {
		t.Errorf("title = %q, want branding stripped to %q", got.Title, "Sinners (2025)")
	}
	if got.Year != 2025 {
		t.Errorf("year = %d, want 2025", got.Year)
	}
}

func TestScrapeFallsBackToTitleTag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><head><title>The Substance | Reviews</title></head><body></body></html>`))
	}))
	defer srv.Close()

	got, err := Scrape(context.Background(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("Scrape: %v", err)
	}
	if got.Title != "The Substance" {
		t.Errorf("title = %q, want %q", got.Title, "The Substance")
	}
}

func TestScrapeReportsDeadLinks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "gone", http.StatusNotFound)
	}))
	defer srv.Close()

	if _, err := Scrape(context.Background(), srv.Client(), srv.URL); err == nil {
		t.Error("Scrape of a 404 returned no error; the capture must land in Needs Review")
	}
}
