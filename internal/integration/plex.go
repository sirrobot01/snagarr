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
			out = append(out, m.item(t))
		}
	}
	return out, nil
}

// SyncCollection makes the named collection hold exactly itemIDs.
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
func (p *Plex) SyncCollection(ctx context.Context, name string, itemIDs []string) error {
	sections, err := p.Sections(ctx)
	if err != nil {
		return err
	}
	key, err := p.findCollection(ctx, sections, name)
	if err != nil {
		return err
	}
	if key == "" {
		if len(itemIDs) == 0 {
			return nil
		}
		return p.createCollection(ctx, name, itemIDs)
	}

	var body plexResponse
	if err := p.rest.Get(ctx, "/library/collections/"+key+"/children", nil, &body); err != nil {
		return fmt.Errorf("plex collection members %s: %w", name, err)
	}
	var current []string
	for _, m := range body.MediaContainer.Metadata {
		current = append(current, m.RatingKey)
	}

	add, remove := diffMembers(current, itemIDs)
	if len(add) > 0 {
		uri, err := p.itemURI(ctx, add)
		if err != nil {
			return err
		}
		path := "/library/collections/" + key + "/items?" + url.Values{"uri": {uri}}.Encode()
		if err := p.rest.Put(ctx, path, nil, nil); err != nil {
			return fmt.Errorf("plex collection add %s: %w", name, err)
		}
	}
	for _, id := range remove {
		if err := p.rest.Delete(ctx, "/library/collections/"+key+"/children/"+id); err != nil {
			return fmt.Errorf("plex collection remove %s from %s: %w", id, name, err)
		}
	}
	return nil
}

func (p *Plex) findCollection(ctx context.Context, sections []Section, name string) (string, error) {
	for _, s := range sections {
		var body plexResponse
		if err := p.rest.Get(ctx, "/library/sections/"+s.ID+"/collections", nil, &body); err != nil {
			return "", fmt.Errorf("plex collections in section %s: %w", s.ID, err)
		}
		for _, m := range body.MediaContainer.Metadata {
			if strings.EqualFold(m.Title, name) {
				return m.RatingKey, nil
			}
		}
	}
	return "", nil
}

func (p *Plex) createCollection(ctx context.Context, name string, itemIDs []string) error {
	sectionID, kind, err := p.itemSection(ctx, itemIDs[0])
	if err != nil {
		return err
	}
	uri, err := p.itemURI(ctx, itemIDs)
	if err != nil {
		return err
	}
	q := url.Values{
		"type":      {strconv.Itoa(kind)},
		"title":     {name},
		"smart":     {"0"},
		"sectionId": {sectionID},
		"uri":       {uri},
	}
	if err := p.rest.Post(ctx, "/library/collections?"+q.Encode(), nil, nil); err != nil {
		return fmt.Errorf("plex create collection %s: %w", name, err)
	}
	return nil
}

// itemSection reports which library a rating key lives in, which is the only way
// to learn the section and type a new collection needs.
func (p *Plex) itemSection(ctx context.Context, ratingKey string) (string, int, error) {
	var body plexResponse
	if err := p.rest.Get(ctx, "/library/metadata/"+ratingKey, nil, &body); err != nil {
		return "", 0, fmt.Errorf("plex metadata %s: %w", ratingKey, err)
	}
	if len(body.MediaContainer.Metadata) == 0 {
		return "", 0, fmt.Errorf("plex metadata %s: no result", ratingKey)
	}
	m := body.MediaContainer.Metadata[0]
	kind := plexMovieType
	if m.Type == "show" {
		kind = plexShowType
	}
	return strconv.Itoa(m.LibrarySectionID), kind, nil
}

func (p *Plex) itemURI(ctx context.Context, itemIDs []string) (string, error) {
	var body plexResponse
	if err := p.rest.Get(ctx, "/identity", nil, &body); err != nil {
		return "", fmt.Errorf("plex identity: %w", err)
	}
	machine := body.MediaContainer.MachineIdentifier
	if machine == "" {
		return "", fmt.Errorf("plex identity: no machine identifier")
	}
	return "server://" + machine + "/com.plexapp.plugins.library/library/metadata/" + strings.Join(itemIDs, ","), nil
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
	RatingKey        string `json:"ratingKey"`
	Type             string `json:"type"`
	Title            string `json:"title"`
	Year             int    `json:"year"`
	AddedAt          int64  `json:"addedAt"`
	LibrarySectionID int    `json:"librarySectionID"`
	GUID             []struct {
		ID string `json:"id"`
	} `json:"Guid"`
}

func (m plexMetadata) item(t store.MediaType) LibraryItem {
	it := LibraryItem{ProviderItemID: m.RatingKey, Type: t, Title: m.Title, Year: m.Year}
	if m.AddedAt > 0 {
		it.AddedAt = time.Unix(m.AddedAt, 0).UTC()
	}
	for _, g := range m.GUID {
		scheme, value, ok := strings.Cut(g.ID, "://")
		if !ok {
			continue
		}
		// Episode-level guids append a season/episode path; keep the id only.
		if i := strings.IndexByte(value, '/'); i >= 0 {
			value = value[:i]
		}
		switch scheme {
		case "tmdb":
			it.TMDBID, _ = strconv.Atoi(value)
		case "imdb":
			it.IMDBID = value
		case "tvdb":
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
