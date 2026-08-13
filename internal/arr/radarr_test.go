package arr

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

const lookupFixture = `[{
	"title":"Sinners",
	"titleSlug":"sinners-1233413",
	"tmdbId":1233413,
	"imdbId":"tt31193180",
	"year":2025,
	"images":[{"coverType":"poster","remoteUrl":"https://image.tmdb.org/a.jpg"}]
}]`

func TestRadarrAdd(t *testing.T) {
	var calls []string
	var posted map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/api/v3/movie/lookup":
			if got := r.URL.Query().Get("term"); got != "tmdb:1233413" {
				t.Errorf("lookup term = %q, want tmdb:1233413", got)
			}
			if got := r.Header.Get("X-Api-Key"); got != "secret" {
				t.Errorf("api key header = %q, want secret", got)
			}
			w.Write([]byte(lookupFixture))
		case "/api/v3/movie":
			if err := json.NewDecoder(r.Body).Decode(&posted); err != nil {
				t.Fatalf("decode post body: %v", err)
			}
			w.Write([]byte(`{"id":42,"tmdbId":1233413,"imdbId":"tt31193180","title":"Sinners","year":2025,"monitored":true,"hasFile":false,"qualityProfileId":6}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	opts := AddOptions{QualityProfileID: 6, RootFolder: "/movies", Monitor: true, SearchOnAdd: true}
	item, err := NewRadarr(srv.URL, "secret").Add(context.Background(), 1233413, opts)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	wantCalls := []string{"GET /api/v3/movie/lookup", "POST /api/v3/movie"}
	if len(calls) != len(wantCalls) {
		t.Fatalf("calls = %v, want %v", calls, wantCalls)
	}
	for i, want := range wantCalls {
		if calls[i] != want {
			t.Errorf("call %d = %q, want %q", i, calls[i], want)
		}
	}

	if posted["titleSlug"] != "sinners-1233413" {
		t.Errorf("posted body dropped the lookup payload: %+v", posted)
	}
	if posted["rootFolderPath"] != "/movies" {
		t.Errorf("rootFolderPath = %v, want /movies", posted["rootFolderPath"])
	}
	if posted["qualityProfileId"] != float64(6) {
		t.Errorf("qualityProfileId = %v, want 6", posted["qualityProfileId"])
	}
	if posted["monitored"] != true {
		t.Errorf("monitored = %v, want true", posted["monitored"])
	}
	addOptions, _ := posted["addOptions"].(map[string]any)
	if addOptions["searchForMovie"] != true {
		t.Errorf("addOptions = %v, want searchForMovie true", posted["addOptions"])
	}

	want := Item{ID: 42, TMDBID: 1233413, IMDBID: "tt31193180", Title: "Sinners", Year: 2025, Monitored: true, QualityProfileID: 6}
	if *item != want {
		t.Errorf("item = %+v, want %+v", *item, want)
	}
}

func TestRadarrAddAlreadyAdded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v3/movie/lookup" {
			w.Write([]byte(lookupFixture))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`[{"errorMessage":"This movie has already been added","propertyName":"TmdbId"}]`))
	}))
	defer srv.Close()

	_, err := NewRadarr(srv.URL, "secret").Add(context.Background(), 1233413, AddOptions{})
	if !errors.Is(err, ErrAlreadyAdded) {
		t.Fatalf("Add error = %v, want ErrAlreadyAdded", err)
	}
}
