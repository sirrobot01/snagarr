package library

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const (
	plexPinPendingFixture = `{"id":42,"code":"WXYZ1234","authToken":null,"expiresAt":"2030-01-01T00:00:00Z"}`
	plexPinClaimedFixture = `{"id":42,"code":"WXYZ1234","authToken":"plex-token","expiresAt":"2030-01-01T00:00:00Z"}`
	plexPinDeadFixture    = `{"id":42,"code":"WXYZ1234","authToken":null,"expiresAt":"2020-01-01T00:00:00Z"}`

	plexResourcesFixture = `[
	{"name":"Living Room TV","clientIdentifier":"player1","provides":"player",
	 "connections":[{"uri":"https://10.0.0.9:32400","local":true,"relay":false}]},
	{"name":"Tower","clientIdentifier":"server1","provides":"server,downloads","connections":[
		{"uri":"https://relay.plex.direct:443","local":false,"relay":true},
		{"uri":"https://remote.plex.direct:32400","local":false,"relay":false},
		{"uri":"https://local.plex.direct:32400","local":true,"relay":false}
	]}
]`
)

func newTestPlexAuth(baseURL string) *PlexAuth {
	a := NewPlexAuth("stable-client-id", "1.2.3")
	a.rest.BaseURL = baseURL
	return a
}

func TestPlexAuthCreatePin(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.URL.Path; got != "/api/v2/pins" {
			t.Errorf("path = %q, want /api/v2/pins", got)
		}
		if got := r.URL.Query().Get("strong"); got != "true" {
			t.Errorf("strong = %q, want true", got)
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(plexPinPendingFixture))
	}))
	defer srv.Close()

	forward := "http://snagarr.local/settings/plex?done=1"
	pin, err := newTestPlexAuth(srv.URL).CreatePin(context.Background(), forward)
	if err != nil {
		t.Fatalf("CreatePin: %v", err)
	}
	if pin.ID != 42 || pin.Code != "WXYZ1234" {
		t.Errorf("pin = %+v, want id 42 code WXYZ1234", pin)
	}
	if want := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC); !pin.ExpiresAt.Equal(want) {
		t.Errorf("ExpiresAt = %v, want %v", pin.ExpiresAt, want)
	}

	base, frag, ok := strings.Cut(pin.AuthURL, "#?")
	if !ok {
		t.Fatalf("AuthURL = %q, want a #? fragment", pin.AuthURL)
	}
	if base != "https://app.plex.tv/auth" {
		t.Errorf("AuthURL base = %q, want https://app.plex.tv/auth", base)
	}
	if !strings.Contains(frag, "forwardUrl=http%3A%2F%2Fsnagarr.local%2Fsettings%2Fplex%3Fdone%3D1") {
		t.Errorf("fragment = %q, want a percent encoded forwardUrl", frag)
	}
	q, err := url.ParseQuery(frag)
	if err != nil {
		t.Fatalf("ParseQuery(%q): %v", frag, err)
	}
	want := map[string]string{
		"clientID":                 "stable-client-id",
		"code":                     "WXYZ1234",
		"context[device][product]": "Snagarr",
		"forwardUrl":               forward,
	}
	for k, v := range want {
		if got := q.Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
}

func TestPlexAuthCreatePinWithoutForwardURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(plexPinPendingFixture))
	}))
	defer srv.Close()

	pin, err := newTestPlexAuth(srv.URL).CreatePin(context.Background(), "")
	if err != nil {
		t.Fatalf("CreatePin: %v", err)
	}
	if strings.Contains(pin.AuthURL, "forwardUrl") {
		t.Errorf("AuthURL = %q, want no forwardUrl", pin.AuthURL)
	}
}

func TestPlexAuthCheckPin(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		want    string
		wantErr error
	}{
		{name: "pending", body: plexPinPendingFixture, wantErr: ErrPinPending},
		{name: "signed in", body: plexPinClaimedFixture, want: "plex-token"},
		{name: "expired", body: plexPinDeadFixture, wantErr: ErrPinExpired},
		{
			name:    "forgotten",
			status:  http.StatusNotFound,
			body:    `{"errors":[{"code":1020,"message":"Code not found or expired"}]}`,
			wantErr: ErrPinExpired,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.URL.Path; got != "/api/v2/pins/42" {
					t.Errorf("path = %q, want /api/v2/pins/42", got)
				}
				if tc.status != 0 {
					w.WriteHeader(tc.status)
				}
				w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			token, err := newTestPlexAuth(srv.URL).CheckPin(context.Background(), 42)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("CheckPin error = %v, want %v", err, tc.wantErr)
			}
			if token != tc.want {
				t.Errorf("token = %q, want %q", token, tc.want)
			}
		})
	}
}

func TestPlexAuthResources(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/api/v2/resources" {
			t.Errorf("path = %q, want /api/v2/resources", got)
		}
		if got := r.URL.Query().Get("includeHttps"); got != "1" {
			t.Errorf("includeHttps = %q, want 1", got)
		}
		if got := r.URL.Query().Get("includeRelay"); got != "1" {
			t.Errorf("includeRelay = %q, want 1", got)
		}
		if got := r.Header.Get("X-Plex-Token"); got != "plex-token" {
			t.Errorf("token header = %q, want plex-token", got)
		}
		w.Write([]byte(plexResourcesFixture))
	}))
	defer srv.Close()

	resources, err := newTestPlexAuth(srv.URL).Resources(context.Background(), "plex-token")
	if err != nil {
		t.Fatalf("Resources: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("got %d resources, want 1: %+v", len(resources), resources)
	}
	if resources[0].Name != "Tower" || resources[0].ClientIdentifier != "server1" {
		t.Errorf("resource = %+v, want Tower/server1", resources[0])
	}
	want := []Connection{
		{URI: "https://local.plex.direct:32400", Local: true},
		{URI: "https://remote.plex.direct:32400"},
		{URI: "https://relay.plex.direct:443", Relay: true},
	}
	if len(resources[0].Connections) != len(want) {
		t.Fatalf("got %d connections, want %d: %+v", len(resources[0].Connections), len(want), resources[0].Connections)
	}
	for i, w := range want {
		if resources[0].Connections[i] != w {
			t.Errorf("connection %d = %+v, want %+v", i, resources[0].Connections[i], w)
		}
	}
}

func TestPlexAuthSendsClientIdentity(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		want := map[string]string{
			"X-Plex-Client-Identifier": "stable-client-id",
			"X-Plex-Product":           "Snagarr",
			"X-Plex-Version":           "1.2.3",
			"Accept":                   "application/json",
		}
		for k, v := range want {
			if got := r.Header.Get(k); got != v {
				t.Errorf("%s %s: %s = %q, want %q", r.Method, r.URL.Path, k, got, v)
			}
		}
		if r.URL.Path == "/api/v2/resources" {
			w.Write([]byte(`[]`))
			return
		}
		w.Write([]byte(plexPinPendingFixture))
	}))
	defer srv.Close()

	auth := newTestPlexAuth(srv.URL)
	if _, err := auth.CreatePin(context.Background(), ""); err != nil {
		t.Fatalf("CreatePin: %v", err)
	}
	if _, err := auth.CheckPin(context.Background(), 42); !errors.Is(err, ErrPinPending) {
		t.Fatalf("CheckPin error = %v, want %v", err, ErrPinPending)
	}
	if _, err := auth.Resources(context.Background(), "plex-token"); err != nil {
		t.Fatalf("Resources: %v", err)
	}
	if got := calls.Load(); got != 3 {
		t.Errorf("got %d requests, want 3", got)
	}
}
