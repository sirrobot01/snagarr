// Package overseerr talks to Overseerr and Jellyseerr, which share one API.
package overseerr

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/sirrobot01/snagarr/internal/httpx"
	"github.com/sirrobot01/snagarr/internal/media"
)

// pageSize is how many requests one /request page returns.
const pageSize = 100

// Request is one Overseerr request. Status is pending, approved, available or
// declined.
type Request struct {
	ID, TMDBID int
	Type       media.Type
	Status     string
}

// Client calls the Overseerr v1 API.
type Client struct {
	rest httpx.Client
}

// New returns a client for an Overseerr or Jellyseerr instance.
func New(baseURL, apiKey string) *Client {
	return &Client{rest: httpx.Client{BaseURL: baseURL, Header: http.Header{"X-Api-Key": {apiKey}}}}
}

// List returns every request, following the pagination.
func (c *Client) List(ctx context.Context) ([]Request, error) {
	var out []Request
	for skip := 0; ; skip += pageSize {
		q := url.Values{"take": {strconv.Itoa(pageSize)}, "skip": {strconv.Itoa(skip)}}
		var body struct {
			Results []request `json:"results"`
		}
		if err := c.rest.Get(ctx, "/api/v1/request", q, &body); err != nil {
			return nil, fmt.Errorf("overseerr list: %w", err)
		}
		for _, r := range body.Results {
			out = append(out, r.request())
		}
		if len(body.Results) < pageSize {
			return out, nil
		}
	}
}

// Create requests a title. TV requests ask for every season.
func (c *Client) Create(ctx context.Context, tmdbID int, t media.Type) (*Request, error) {
	if !t.Valid() {
		return nil, fmt.Errorf("overseerr create: unknown media type %q", t)
	}
	body := map[string]any{"mediaType": string(t), "mediaId": tmdbID}
	if t == media.TV {
		body["seasons"] = "all"
	}
	var created request
	if err := c.rest.Post(ctx, "/api/v1/request", body, &created); err != nil {
		return nil, fmt.Errorf("overseerr create %d: %w", tmdbID, err)
	}
	r := created.request()
	return &r, nil
}

// Ping reports connectivity and the server version.
func (c *Client) Ping(ctx context.Context) (string, error) {
	var body struct {
		Version string `json:"version"`
	}
	if err := c.rest.Get(ctx, "/api/v1/status", nil, &body); err != nil {
		return "", fmt.Errorf("overseerr ping: %w", err)
	}
	if body.Version == "" {
		return "OK", nil
	}
	return "OK · v" + body.Version, nil
}

type request struct {
	ID     int `json:"id"`
	Status int `json:"status"`
	Media  struct {
		TmdbID    int    `json:"tmdbId"`
		MediaType string `json:"mediaType"`
		Status    int    `json:"status"`
	} `json:"media"`
}

func (r request) request() Request {
	out := Request{ID: r.ID, TMDBID: r.Media.TmdbID, Status: status(r.Status, r.Media.Status)}
	if r.Media.MediaType == string(media.TV) {
		out.Type = media.TV
	} else {
		out.Type = media.Movie
	}
	return out
}

// status folds Overseerr's two numeric states into one word. Media status 5
// means the file is on the server, whatever the request state says.
func status(requestStatus, mediaStatus int) string {
	if mediaStatus == 5 {
		return "available"
	}
	switch requestStatus {
	case 2:
		return "approved"
	case 3:
		return "declined"
	default:
		return "pending"
	}
}
