package integration

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sirrobot01/snagarr/internal/store"
)

// embyPageSize is how many items one /Items call returns.
const embyPageSize = 500

// itemFields are the extra fields /Items must be asked for; none of them are
// returned by default.
const itemFields = "ProviderIds,DateCreated,ProductionYear"

// Emby reads an Emby or Jellyfin server. Their APIs match closely enough for one
// client.
type Emby struct {
	rest     client
	jellyfin bool
}

// NewEmby returns a client for Emby, or for Jellyfin when jellyfin is true.
// Jellyfin accepts the same X-Emby-Token header.
func NewEmby(baseURL, apiKey string, jellyfin bool) *Emby {
	return &Emby{
		rest:     client{BaseURL: baseURL, Header: http.Header{"X-Emby-Token": {apiKey}}},
		jellyfin: jellyfin,
	}
}

// Ping reports connectivity and how many movies and series the server holds.
func (e *Emby) Ping(ctx context.Context) (string, error) {
	q := url.Values{
		"Recursive":        {"true"},
		"IncludeItemTypes": {"Movie,Series"},
		"Limit":            {"0"},
	}
	var body embyItems
	if err := e.rest.Get(ctx, "/Items", q, &body); err != nil {
		return "", fmt.Errorf("%s ping: %w", e.name(), err)
	}
	return "OK · " + strconv.Itoa(body.TotalRecordCount) + " items", nil
}

// Sections lists the movie and show libraries.
func (e *Emby) Sections(ctx context.Context) ([]Section, error) {
	var body []struct {
		Name           string `json:"Name"`
		ItemID         string `json:"ItemId"`
		CollectionType string `json:"CollectionType"`
	}
	if err := e.rest.Get(ctx, "/Library/VirtualFolders", nil, &body); err != nil {
		return nil, fmt.Errorf("%s sections: %w", e.name(), err)
	}
	var out []Section
	for _, f := range body {
		var t store.MediaType
		switch strings.ToLower(f.CollectionType) {
		case "movies":
			t = store.Movie
		case "tvshows":
			t = store.TV
		default:
			continue
		}
		out = append(out, Section{ID: f.ItemID, Title: f.Name, Type: t})
	}
	return out, nil
}

// Items lists library contents. A zero since sweeps everything; otherwise only
// titles saved after since are returned. Empty sectionIDs means every library.
func (e *Emby) Items(ctx context.Context, sectionIDs []string, since time.Time) ([]LibraryItem, error) {
	parents := sectionIDs
	if len(parents) == 0 {
		parents = []string{""}
	}

	var out []LibraryItem
	for _, parent := range parents {
		for start := 0; ; start += embyPageSize {
			q := url.Values{
				"Recursive":        {"true"},
				"IncludeItemTypes": {"Movie,Series"},
				"Fields":           {itemFields},
				"StartIndex":       {strconv.Itoa(start)},
				"Limit":            {strconv.Itoa(embyPageSize)},
			}
			if parent != "" {
				q.Set("ParentId", parent)
			}
			if !since.IsZero() {
				q.Set("MinDateLastSaved", since.UTC().Format(time.RFC3339))
			}
			var body embyItems
			if err := e.rest.Get(ctx, "/Items", q, &body); err != nil {
				return nil, fmt.Errorf("%s items: %w", e.name(), err)
			}
			for _, it := range body.Items {
				item, ok := it.item()
				if !ok {
					continue
				}
				out = append(out, item)
			}
			if len(body.Items) < embyPageSize {
				break
			}
		}
	}
	return out, nil
}

// SyncCollection makes the named collection hold exactly itemIDs.
func (e *Emby) SyncCollection(ctx context.Context, name string, itemIDs []string) error {
	id, err := e.findCollection(ctx, name)
	if err != nil {
		return err
	}
	if id == "" {
		if len(itemIDs) == 0 {
			return nil
		}
		q := url.Values{"Name": {name}, "Ids": {strings.Join(itemIDs, ",")}}
		if err := e.rest.Post(ctx, "/Collections?"+q.Encode(), nil, nil); err != nil {
			return fmt.Errorf("%s create collection %s: %w", e.name(), name, err)
		}
		return nil
	}

	q := url.Values{"ParentId": {id}, "Recursive": {"true"}}
	var body embyItems
	if err := e.rest.Get(ctx, "/Items", q, &body); err != nil {
		return fmt.Errorf("%s collection members %s: %w", e.name(), name, err)
	}
	var current []string
	for _, it := range body.Items {
		current = append(current, it.ID)
	}

	add, remove := diffMembers(current, itemIDs)
	if len(add) > 0 {
		path := "/Collections/" + id + "/Items?" + url.Values{"Ids": {strings.Join(add, ",")}}.Encode()
		if err := e.rest.Post(ctx, path, nil, nil); err != nil {
			return fmt.Errorf("%s collection add %s: %w", e.name(), name, err)
		}
	}
	if len(remove) > 0 {
		path := "/Collections/" + id + "/Items?" + url.Values{"Ids": {strings.Join(remove, ",")}}.Encode()
		if err := e.rest.Delete(ctx, path); err != nil {
			return fmt.Errorf("%s collection remove %s: %w", e.name(), name, err)
		}
	}
	return nil
}

func (e *Emby) findCollection(ctx context.Context, name string) (string, error) {
	q := url.Values{"Recursive": {"true"}, "IncludeItemTypes": {"BoxSet"}}
	var body embyItems
	if err := e.rest.Get(ctx, "/Items", q, &body); err != nil {
		return "", fmt.Errorf("%s collections: %w", e.name(), err)
	}
	for _, it := range body.Items {
		if strings.EqualFold(it.Name, name) {
			return it.ID, nil
		}
	}
	return "", nil
}

func (e *Emby) name() string {
	if e.jellyfin {
		return "jellyfin"
	}
	return "emby"
}

type embyItems struct {
	Items            []embyItem `json:"Items"`
	TotalRecordCount int        `json:"TotalRecordCount"`
}

type embyItem struct {
	ID             string            `json:"Id"`
	Name           string            `json:"Name"`
	Type           string            `json:"Type"`
	ProductionYear int               `json:"ProductionYear"`
	DateCreated    time.Time         `json:"DateCreated"`
	ProviderIDs    map[string]string `json:"ProviderIds"`
}

func (i embyItem) item() (LibraryItem, bool) {
	var t store.MediaType
	switch i.Type {
	case "Movie":
		t = store.Movie
	case "Series":
		t = store.TV
	default:
		return LibraryItem{}, false
	}
	it := LibraryItem{
		ProviderItemID: i.ID,
		Type:           t,
		Title:          i.Name,
		Year:           i.ProductionYear,
		AddedAt:        i.DateCreated,
	}
	// Emby writes "Tmdb", Jellyfin "TMDB" on some versions.
	for k, v := range i.ProviderIDs {
		switch {
		case strings.EqualFold(k, "tmdb"):
			it.TMDBID, _ = strconv.Atoi(v)
		case strings.EqualFold(k, "imdb"):
			it.IMDBID = v
		case strings.EqualFold(k, "tvdb"):
			it.TVDBID, _ = strconv.Atoi(v)
		}
	}
	return it, true
}
