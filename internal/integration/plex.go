package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/sirrobot01/snagarr/internal/store"
)

// Plex type ids used by the collection endpoints.
const (
	plexMovieType = 1
	plexShowType  = 2
)

// Plex reads a Plex Media Server over its HTTP API.
type Plex struct {
	rest client
}

// NewPlex returns a client authenticated with a Plex token.
func NewPlex(baseURL, token string) *Plex {
	return &Plex{rest: client{BaseURL: baseURL, Header: http.Header{"X-Plex-Token": {token}}}}
}

// Ping reports connectivity and how many titles the movie and show libraries hold.
func (p *Plex) Ping(ctx context.Context) (string, error) {
	sections, err := p.Sections(ctx)
	if err != nil {
		return "", fmt.Errorf("plex ping: %w", err)
	}
	total := 0
	for _, s := range sections {
		// Container size 0 asks Plex for the count without the payload.
		q := url.Values{"X-Plex-Container-Start": {"0"}, "X-Plex-Container-Size": {"0"}}
		var body plexResponse
		if err := p.rest.Get(ctx, "/library/sections/"+s.ID+"/all", q, &body); err != nil {
			return "", fmt.Errorf("plex ping section %s: %w", s.ID, err)
		}
		total += body.MediaContainer.TotalSize
	}
	return "OK · " + strconv.Itoa(total) + " items", nil
}

// Sections lists the movie and show libraries. Music and photo libraries are
// dropped.
func (p *Plex) Sections(ctx context.Context) ([]Section, error) {
	var body plexResponse
	if err := p.rest.Get(ctx, "/library/sections", nil, &body); err != nil {
		return nil, fmt.Errorf("plex sections: %w", err)
	}
	var out []Section
	for _, d := range body.MediaContainer.Directory {
		t, ok := plexType(d.Type)
		if !ok {
			continue
		}
		out = append(out, Section{ID: d.Key, Title: d.Title, Type: t})
	}
	return out, nil
}

// Items lists library contents. A zero since sweeps everything; otherwise only
// titles added after since are returned. Empty sectionIDs means every movie and
// show section.
func (p *Plex) Items(ctx context.Context, sectionIDs []string, since time.Time) ([]LibraryItem, error) {
	if len(sectionIDs) == 0 {
		sections, err := p.Sections(ctx)
		if err != nil {
			return nil, err
		}
		for _, s := range sections {
			sectionIDs = append(sectionIDs, s.ID)
		}
	}

	var out []LibraryItem
	for _, id := range sectionIDs {
		q := url.Values{"includeGuids": {"1"}}
		if !since.IsZero() {
			// Plex filters with operators baked into the parameter name.
			q.Set("addedAt>=", strconv.FormatInt(since.Unix(), 10))
		}
		var body plexResponse
		if err := p.rest.Get(ctx, "/library/sections/"+id+"/all", q, &body); err != nil {
			return nil, fmt.Errorf("plex items section %s: %w", id, err)
		}
		for _, m := range body.MediaContainer.Metadata {
			t, ok := plexType(m.Type)
			if !ok {
				continue
			}
			item := m.item(t)
			item.SectionID = id
			out = append(out, item)
		}
	}
	return out, nil
}

// maxCollectionKeys caps how many rating keys ride on one request. The keys go
// in the query string, so a whole library at once would overrun the request line
// the server accepts.
const maxCollectionKeys = 200

// SyncCollection makes the named collection hold exactly members.
//
// A Plex collection belongs to one library section and carries that section's
// type, so a household that snags both films and series keeps two collections of
// the same name, one per section. Offering a show to the movie collection is
// refused with a bare HTML 400.
//
// Plex's collection API is spread over several endpoints and none of them takes
// a JSON body:
//
//	GET    /library/sections/{section}/collections          list collections
//	GET    /library/collections/{key}/children              current members
//	POST   /library/collections?type=&title=&smart=0&sectionId=&uri=  create
//	PUT    /library/collections/{key}/items?uri=            add members
//	DELETE /library/collections/{key}/children/{item}       remove one member
//
// The uri parameter is a server:// reference carrying the machine identifier and
// a comma-separated list of rating keys.
func (p *Plex) SyncCollection(ctx context.Context, name string, members []CollectionMember) error {
	sections, err := p.Sections(ctx)
	if err != nil {
		return err
	}
	machine, err := p.machineID(ctx)
	if err != nil {
		return err
	}

	// A section holding none of the members is still visited, because that is
	// how the last title to leave the list leaves the collection.
	wanted := make(map[string][]string, len(sections))
	for _, s := range sections {
		wanted[s.ID] = nil
	}
	for _, m := range members {
		if _, ok := wanted[m.SectionID]; !ok {
			continue // A section this server no longer reports.
		}
		wanted[m.SectionID] = append(wanted[m.SectionID], m.ID)
	}

	// One section that will not take its titles must not stop the others, and a
	// failed add must not cost the removals.
	var errs []error
	for _, s := range sections {
		if err := p.syncSection(ctx, name, machine, s, wanted[s.ID]); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (p *Plex) syncSection(ctx context.Context, name, machine string, section Section, itemIDs []string) error {
	key, err := p.findCollection(ctx, section.ID, name)
	if err != nil {
		return err
	}
	if key == "" {
		if len(itemIDs) == 0 {
			return nil
		}
		// Creating seeds the collection with the first keys; the diff below adds
		// whatever did not fit.
		if err := p.createCollection(ctx, name, machine, section, itemIDs[:min(len(itemIDs), maxCollectionKeys)]); err != nil {
			return err
		}
		if key, err = p.findCollection(ctx, section.ID, name); err != nil {
			return err
		}
		if key == "" {
			return fmt.Errorf("plex create collection %s in section %s: not found afterwards", name, section.ID)
		}
	}

	var body plexResponse
	if err := p.rest.Get(ctx, "/library/collections/"+key+"/children", nil, &body); err != nil {
		return fmt.Errorf("plex collection members %s: %w", name, err)
	}
	var current []string
	for _, m := range body.MediaContainer.Metadata {
		current = append(current, m.RatingKey)
	}

	var errs []error
	add, remove := diffMembers(current, itemIDs)
	for chunk := range slices.Chunk(add, maxCollectionKeys) {
		q := url.Values{"uri": {itemURI(machine, chunk)}}
		if err := p.rest.Put(ctx, "/library/collections/"+key+"/items?"+q.Encode(), nil, nil); err != nil {
			errs = append(errs, fmt.Errorf("plex collection add %s: %w", name, err))
			break
		}
	}
	for _, id := range remove {
		if err := p.rest.Delete(ctx, "/library/collections/"+key+"/children/"+id); err != nil {
			errs = append(errs, fmt.Errorf("plex collection remove %s from %s: %w", id, name, err))
		}
	}
	return errors.Join(errs...)
}

// findCollection looks in one section only, because that is the scope a Plex
// collection has. The same name in another section is a different collection.
func (p *Plex) findCollection(ctx context.Context, sectionID, name string) (string, error) {
	var body plexResponse
	if err := p.rest.Get(ctx, "/library/sections/"+sectionID+"/collections", nil, &body); err != nil {
		return "", fmt.Errorf("plex collections in section %s: %w", sectionID, err)
	}
	for _, m := range body.MediaContainer.Metadata {
		if strings.EqualFold(m.Title, name) {
			return m.RatingKey, nil
		}
	}
	return "", nil
}

func (p *Plex) createCollection(ctx context.Context, name, machine string, section Section, itemIDs []string) error {
	kind := plexMovieType
	if section.Type == store.TV {
		kind = plexShowType
	}
	q := url.Values{
		"type":      {strconv.Itoa(kind)},
		"title":     {name},
		"smart":     {"0"},
		"sectionId": {section.ID},
		"uri":       {itemURI(machine, itemIDs)},
	}
	if err := p.rest.Post(ctx, "/library/collections?"+q.Encode(), nil, nil); err != nil {
		return fmt.Errorf("plex create collection %s: %w", name, err)
	}
	return nil
}

// machineID names this server in the uri parameter the collection endpoints
// take.
func (p *Plex) machineID(ctx context.Context) (string, error) {
	var body plexResponse
	if err := p.rest.Get(ctx, "/identity", nil, &body); err != nil {
		return "", fmt.Errorf("plex identity: %w", err)
	}
	if body.MediaContainer.MachineIdentifier == "" {
		return "", fmt.Errorf("plex identity: no machine identifier")
	}
	return body.MediaContainer.MachineIdentifier, nil
}

func itemURI(machine string, itemIDs []string) string {
	return "server://" + machine + "/com.plexapp.plugins.library/library/metadata/" + strings.Join(itemIDs, ",")
}

type plexResponse struct {
	MediaContainer struct {
		TotalSize         int    `json:"totalSize"`
		MachineIdentifier string `json:"machineIdentifier"`
		Directory         []struct {
			Key   string `json:"key"`
			Title string `json:"title"`
			Type  string `json:"type"`
		} `json:"Directory"`
		Metadata []plexMetadata `json:"Metadata"`
	} `json:"MediaContainer"`
}

type plexMetadata struct {
	RatingKey string    `json:"ratingKey"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Year      int       `json:"year"`
	AddedAt   int64     `json:"addedAt"`
	GUID      plexGUIDs `json:"Guid"`
}

// plexGUIDs holds the guid values of one metadata record. Plex sends them in two
// shapes: an array of objects under "Guid" when includeGuids is set, and always a
// single string under "guid". Both land in this field because the JSON decoder
// matches tags case-insensitively, so take either and keep every value.
type plexGUIDs []string

func (g *plexGUIDs) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*g = append(*g, s)
		return nil
	}
	var list []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(b, &list); err != nil {
		return err
	}
	for _, v := range list {
		*g = append(*g, v.ID)
	}
	return nil
}

func (m plexMetadata) item(t store.MediaType) LibraryItem {
	it := LibraryItem{ProviderItemID: m.RatingKey, Type: t, Title: m.Title, Year: m.Year}
	if m.AddedAt > 0 {
		it.AddedAt = time.Unix(m.AddedAt, 0).UTC()
	}
	for _, g := range m.GUID {
		scheme, value, ok := strings.Cut(g, "://")
		if !ok {
			continue
		}
		// Episode-level guids append a season/episode path and legacy agent guids
		// a query string; keep the id only.
		value, _, _ = strings.Cut(value, "?")
		if i := strings.IndexByte(value, '/'); i >= 0 {
			value = value[:i]
		}
		switch scheme {
		case "tmdb", "com.plexapp.agents.themoviedb":
			it.TMDBID, _ = strconv.Atoi(value)
		case "imdb", "com.plexapp.agents.imdb":
			it.IMDBID = value
		case "tvdb", "com.plexapp.agents.thetvdb":
			it.TVDBID, _ = strconv.Atoi(value)
		}
	}
	return it
}

func plexType(kind string) (store.MediaType, bool) {
	switch kind {
	case "movie":
		return store.Movie, true
	case "show":
		return store.TV, true
	default:
		return "", false
	}
}
