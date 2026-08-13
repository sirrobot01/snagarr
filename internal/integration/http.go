package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// UserAgent is sent as the User-Agent header on every request. Commands may
// overwrite it at startup to carry a build version.
var UserAgent = "snagarr"

// maxErrorBody caps how much of a failing response body an *HTTPError keeps.
const maxErrorBody = 512

var defaultHTTP = &http.Client{Timeout: 20 * time.Second}

// client is the JSON-over-HTTP transport every integration client in this
// package shares, so none of them reimplement request plumbing. It talks JSON
// to a service rooted at BaseURL.
type client struct {
	BaseURL string
	HTTP    *http.Client // nil uses a shared client with a 20s timeout
	Header  http.Header  // sent on every request
}

// HTTPError is returned for every response outside the 2xx range.
type HTTPError struct {
	Status int
	Body   string
}

// Error implements the error interface.
func (e *HTTPError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("http %d", e.Status)
	}
	return fmt.Sprintf("http %d: %s", e.Status, e.Body)
}

// StatusOf returns the HTTP status carried by err, or 0 when err is not an *HTTPError.
func StatusOf(err error) int {
	var e *HTTPError
	if errors.As(err, &e) {
		return e.Status
	}
	return 0
}

// Get sends a GET request and decodes the JSON response into out. A nil out
// discards the body. path may carry its own query string, which is merged with
// query.
func (c *client) Get(ctx context.Context, path string, query url.Values, out any) error {
	return c.do(ctx, http.MethodGet, path, query, nil, out)
}

// Post sends body as JSON and decodes the JSON response into out. A nil body
// sends no request body; a nil out discards the response body.
func (c *client) Post(ctx context.Context, path string, body, out any) error {
	return c.do(ctx, http.MethodPost, path, nil, body, out)
}

// Put sends body as JSON and decodes the JSON response into out.
func (c *client) Put(ctx context.Context, path string, body, out any) error {
	return c.do(ctx, http.MethodPut, path, nil, body, out)
}

// Delete sends a DELETE request and discards the response body.
func (c *client) Delete(ctx context.Context, path string) error {
	return c.do(ctx, http.MethodDelete, path, nil, nil, nil)
}

func (c *client) do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	u, err := c.resolve(path, query)
	if err != nil {
		return err
	}

	var payload io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode %s %s body: %w", method, u.Path, err)
		}
		payload = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), payload)
	if err != nil {
		return fmt.Errorf("build %s %s: %w", method, u.Path, err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", UserAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range c.Header {
		req.Header[http.CanonicalHeaderKey(k)] = v
	}

	client := c.HTTP
	if client == nil {
		client = defaultHTTP
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, u.Redacted(), err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
		return &HTTPError{Status: resp.StatusCode, Body: string(bytes.TrimSpace(b))}
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s %s: %w", method, u.Path, err)
	}
	return nil
}

// resolve joins path onto BaseURL, keeping any subpath the base carries and
// collapsing duplicate slashes.
func (c *client) resolve(path string, query url.Values) (*url.URL, error) {
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base url %q: %w", c.BaseURL, err)
	}
	raw := ""
	if before, after, ok := strings.Cut(path, "?"); ok {
		raw, path = after, before
	}
	u = u.JoinPath(path)
	switch {
	case raw != "" && len(query) > 0:
		u.RawQuery = raw + "&" + query.Encode()
	case raw != "":
		u.RawQuery = raw
	case len(query) > 0:
		u.RawQuery = query.Encode()
	}
	return u, nil
}
