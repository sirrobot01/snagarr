package integration

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestClientResolvesPaths(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.RequestURI()
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	cases := []struct {
		name   string
		suffix string
		path   string
		query  url.Values
		want   string
	}{
		{name: "plain", path: "/api/v3/movie", want: "/api/v3/movie"},
		{name: "trailing slash on base", suffix: "/", path: "/api/v3/movie", want: "/api/v3/movie"},
		{name: "subpath", suffix: "/radarr", path: "/api/v3/movie", want: "/radarr/api/v3/movie"},
		{name: "subpath and trailing slash", suffix: "/radarr/", path: "/api/v3/movie", want: "/radarr/api/v3/movie"},
		{
			name:  "query",
			path:  "/api/v3/movie/lookup",
			query: url.Values{"term": {"tmdb:603"}},
			want:  "/api/v3/movie/lookup?term=tmdb%3A603",
		},
		{
			name:  "query in path is merged",
			path:  "/library/collections?smart=0",
			query: url.Values{"title": {"Snagged"}},
			want:  "/library/collections?smart=0&title=Snagged",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := client{BaseURL: srv.URL + tc.suffix}
			if err := c.Get(context.Background(), tc.path, tc.query, nil); err != nil {
				t.Fatalf("Get: %v", err)
			}
			if got != tc.want {
				t.Errorf("request uri = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestClientReturnsErrorOnNon2xx(t *testing.T) {
	body := strings.Repeat("x", 900)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, body, http.StatusBadRequest)
	}))
	defer srv.Close()

	c := client{BaseURL: srv.URL}
	err := c.Get(context.Background(), "/movie", nil, nil)
	if err == nil {
		t.Fatal("Get: want error")
	}
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("Get error = %T, want *HTTPError", err)
	}
	if httpErr.Status != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", httpErr.Status)
	}
	if len(httpErr.Body) != maxErrorBody {
		t.Errorf("body length = %d, want %d", len(httpErr.Body), maxErrorBody)
	}
	if StatusOf(err) != http.StatusBadRequest {
		t.Errorf("StatusOf = %d, want 400", StatusOf(err))
	}
	if StatusOf(errors.New("boom")) != 0 {
		t.Error("StatusOf on a plain error, want 0")
	}
}
