package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/sirrobot01/snagarr/internal/config"
	"github.com/sirrobot01/snagarr/internal/integration"
	"github.com/sirrobot01/snagarr/internal/store"
)

func newTestReconciler(t *testing.T) (*Reconciler, *store.Store, int64) {
	t.Helper()
	ctx := context.Background()

	db, err := store.Open(filepath.Join(t.TempDir(), "snagarr.db"), make([]byte, 32))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	settings, err := config.NewManager(ctx, db)
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	owner := &store.User{Username: "owner", Role: store.RoleAdmin}
	if err := db.CreateUser(ctx, owner); err != nil {
		t.Fatalf("create user: %v", err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewReconciler(db, settings, log), db, owner.ID
}

func newArrService(t *testing.T, db *store.Store, userID int64, kind store.ServiceKind, url string) store.Service {
	t.Helper()
	cfg, err := json.Marshal(map[string]any{"url": url, "api_key": "k"})
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	svc := &store.Service{
		UserID: userID, Kind: kind, Name: string(kind), Enabled: true, Config: cfg,
	}
	if err := db.CreateService(context.Background(), svc); err != nil {
		t.Fatalf("create service: %v", err)
	}
	return *svc
}

// Both services must be in flight at once. Each handler waits for the other to
// arrive, so a pass that walks its services one at a time cannot finish: the
// first request blocks on a second that has not been sent yet.
func TestSyncArrRunsServicesConcurrently(t *testing.T) {
	arrived := make(chan struct{}, 2)
	both := make(chan struct{})

	handler := func(w http.ResponseWriter, r *http.Request) {
		arrived <- struct{}{}
		select {
		case <-both:
		case <-time.After(5 * time.Second):
			t.Errorf("%s was still alone after 5s: services are syncing one at a time", r.Host)
		}
		fmt.Fprint(w, `[]`)
	}
	radarr := httptest.NewServer(http.HandlerFunc(handler))
	defer radarr.Close()
	sonarr := httptest.NewServer(http.HandlerFunc(handler))
	defer sonarr.Close()

	go func() {
		<-arrived
		<-arrived
		close(both)
	}()

	e, db, owner := newTestReconciler(t)
	house := integration.Household{}
	r, err := integration.BuildRadarr(newArrService(t, db, owner, store.KindRadarr, radarr.URL))
	if err != nil {
		t.Fatalf("build radarr: %v", err)
	}
	s, err := integration.BuildSonarr(newArrService(t, db, owner, store.KindSonarr, sonarr.URL))
	if err != nil {
		t.Fatalf("build sonarr: %v", err)
	}
	house.Radarrs = append(house.Radarrs, r)
	house.Sonarrs = append(house.Sonarrs, s)

	e.syncArr(context.Background(), house)
}
