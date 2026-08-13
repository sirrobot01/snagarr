package integration

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"
)

// plexProduct names Snagarr on the sign-in page and in the account's device list.
const plexProduct = "Snagarr"

// plexTVURL is the account API, which is a different host from the user's server.
const plexTVURL = "https://plex.tv"

// plexSignInURL is the page the user signs in on.
const plexSignInURL = "https://app.plex.tv/auth"

// ErrPinPending reports that the user has not finished signing in.
var ErrPinPending = errors.New("plex: sign-in not finished")

// ErrPinExpired reports that the pin can no longer be claimed.
var ErrPinExpired = errors.New("plex: pin expired")

// PlexAuth runs the plex.tv PIN flow, which trades a short code for the
// X-Plex-Token a Plex client needs.
type PlexAuth struct {
	rest     client
	clientID string
}

// NewPlexAuth returns a plex.tv client identified by clientID. clientID must be
// stable across restarts: plex.tv scopes every token to the identifier that
// claimed it, so a fresh one throws away the tokens already granted.
func NewPlexAuth(clientID, version string) *PlexAuth {
	return &PlexAuth{
		rest: client{
			BaseURL: plexTVURL,
			Header: http.Header{
				"X-Plex-Product":           {plexProduct},
				"X-Plex-Version":           {version},
				"X-Plex-Client-Identifier": {clientID},
				"X-Plex-Platform":          {runtime.GOOS},
				"X-Plex-Device":            {runtime.GOOS},
				"X-Plex-Device-Name":       {plexProduct},
			},
		},
		clientID: clientID,
	}
}

// Pin is a sign-in attempt in progress.
type Pin struct {
	ID        int
	Code      string
	AuthURL   string // send the user here to sign in
	ExpiresAt time.Time
}

// CreatePin starts the flow. forwardURL is where plex.tv returns the user after
// signing in; pass "" to omit it.
func (a *PlexAuth) CreatePin(ctx context.Context, forwardURL string) (*Pin, error) {
	var body plexPin
	// strong=true asks for a code long enough that it cannot be guessed.
	if err := a.rest.Post(ctx, "/api/v2/pins?strong=true", nil, &body); err != nil {
		return nil, fmt.Errorf("plex create pin: %w", err)
	}

	q := url.Values{
		"clientID":                 {a.clientID},
		"code":                     {body.Code},
		"context[device][product]": {plexProduct},
	}
	if forwardURL != "" {
		q.Set("forwardUrl", forwardURL)
	}
	// The sign-in page reads its parameters from the fragment, not the query.
	return &Pin{
		ID:        body.ID,
		Code:      body.Code,
		AuthURL:   plexSignInURL + "#?" + q.Encode(),
		ExpiresAt: body.ExpiresAt,
	}, nil
}

// CheckPin returns the token once the user has signed in. It returns
// ErrPinPending until then, and ErrPinExpired once the pin is dead.
func (a *PlexAuth) CheckPin(ctx context.Context, pinID int) (string, error) {
	var body plexPin
	if err := a.rest.Get(ctx, "/api/v2/pins/"+strconv.Itoa(pinID), nil, &body); err != nil {
		// plex.tv forgets a pin once it expires, so a 404 ends the flow.
		if StatusOf(err) == http.StatusNotFound {
			return "", ErrPinExpired
		}
		return "", fmt.Errorf("plex check pin %d: %w", pinID, err)
	}
	if body.AuthToken != "" {
		return body.AuthToken, nil
	}
	if !body.ExpiresAt.IsZero() && !time.Now().Before(body.ExpiresAt) {
		return "", ErrPinExpired
	}
	return "", ErrPinPending
}

// Resource is a Plex Media Server the account can reach.
type Resource struct {
	Name             string
	ClientIdentifier string
	Connections      []Connection
}

// Connection is one address a server answers on.
type Connection struct {
	URI          string
	Local, Relay bool
}

// Resources lists the servers token can reach, so a caller can offer a choice
// instead of asking for a URL. Connections come back in the order worth trying:
// local, then remote, then the relay, which plex.tv rate limits.
func (a *PlexAuth) Resources(ctx context.Context, token string) ([]Resource, error) {
	rest := a.rest
	rest.Header = a.rest.Header.Clone()
	rest.Header.Set("X-Plex-Token", token)

	var body []struct {
		Name             string `json:"name"`
		ClientIdentifier string `json:"clientIdentifier"`
		Provides         string `json:"provides"`
		Connections      []struct {
			URI   string `json:"uri"`
			Local bool   `json:"local"`
			Relay bool   `json:"relay"`
		} `json:"connections"`
	}
	q := url.Values{"includeHttps": {"1"}, "includeRelay": {"1"}}
	if err := rest.Get(ctx, "/api/v2/resources", q, &body); err != nil {
		return nil, fmt.Errorf("plex resources: %w", err)
	}

	rank := func(c Connection) int {
		switch {
		case c.Relay:
			return 2
		case c.Local:
			return 0
		default:
			return 1
		}
	}

	var out []Resource
	for _, r := range body {
		// provides is a comma separated list, such as "server,player".
		if !slices.Contains(strings.Split(r.Provides, ","), "server") {
			continue
		}
		res := Resource{Name: r.Name, ClientIdentifier: r.ClientIdentifier}
		for _, c := range r.Connections {
			res.Connections = append(res.Connections, Connection{URI: c.URI, Local: c.Local, Relay: c.Relay})
		}
		slices.SortStableFunc(res.Connections, func(x, y Connection) int {
			return cmp.Compare(rank(x), rank(y))
		})
		out = append(out, res)
	}
	return out, nil
}

type plexPin struct {
	ID        int       `json:"id"`
	Code      string    `json:"code"`
	AuthToken string    `json:"authToken"`
	ExpiresAt time.Time `json:"expiresAt"`
}
