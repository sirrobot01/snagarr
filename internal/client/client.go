// Package client is the small HTTP client shared by Snagarr's command-line
// commands. It mirrors only the public API fields the CLI needs.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sirrobot01/snagarr/internal/version"
)

const maxResponseSize = 4 << 20

type Client struct {
	server string
	token  string
	http   *http.Client
}

type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string {
	if e.Code == "" {
		return e.Message
	}
	return e.Code + ": " + e.Message
}

type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type Session struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type Item struct {
	ID         int64     `json:"id"`
	Title      string    `json:"title"`
	Year       *int      `json:"year"`
	Status     string    `json:"status"`
	Archived   bool      `json:"archived"`
	CapturedBy *User     `json:"captured_by"`
	CapturedAt time.Time `json:"captured_at"`
	Note       *string   `json:"note"`
}

type ItemsResponse struct {
	Items []Item `json:"items"`
	Total int    `json:"total"`
}

type StatusResponse struct {
	Version string `json:"version"`
	Counts  struct {
		Total       int `json:"total"`
		Ready       int `json:"ready"`
		Pending     int `json:"pending"`
		NeedsReview int `json:"needs_review"`
		Archived    int `json:"archived"`
	} `json:"counts"`
	Services map[string]bool `json:"services"`
}

type CaptureInput struct {
	Query  string `json:"query"`
	Source string `json:"source"`
	Note   string `json:"note,omitempty"`
}

func New(server, token string) (*Client, error) {
	normalized, err := normalizeServer(server)
	if err != nil {
		return nil, err
	}
	return &Client{
		server: normalized,
		token:  strings.TrimSpace(token),
		http:   &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (c *Client) Server() string { return c.server }

func (c *Client) Login(ctx context.Context, username, password string) (Session, error) {
	var session Session
	err := c.do(ctx, http.MethodPost, "/auth/login", map[string]string{
		"username": username,
		"password": password,
	}, &session, false)
	return session, err
}

func (c *Client) Me(ctx context.Context) (User, error) {
	var user User
	err := c.do(ctx, http.MethodGet, "/me", nil, &user, true)
	return user, err
}

func (c *Client) Capture(ctx context.Context, input CaptureInput) (Item, error) {
	var item Item
	err := c.do(ctx, http.MethodPost, "/capture", input, &item, true)
	return item, err
}

func (c *Client) Items(ctx context.Context, status string, archived bool, limit int) (ItemsResponse, error) {
	query := url.Values{}
	if status != "" {
		query.Set("status", status)
	}
	if archived {
		query.Set("archived", "true")
	}
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	path := "/items"
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var items ItemsResponse
	err := c.do(ctx, http.MethodGet, path, nil, &items, true)
	return items, err
}

func (c *Client) Status(ctx context.Context) (StatusResponse, error) {
	var status StatusResponse
	err := c.do(ctx, http.MethodGet, "/status", nil, &status, true)
	return status, err
}

func (c *Client) do(ctx context.Context, method, path string, body, out any, authenticated bool) error {
	var input io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		input = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.server+"/api/v1"+path, input)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", version.UserAgent()+" cli")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if authenticated {
		if c.token == "" {
			return fmt.Errorf("no token configured; run `snagarr login %s`", c.server)
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("reach %s: %w", c.server, err)
	}
	defer res.Body.Close()

	limited := io.LimitReader(res.Body, maxResponseSize)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		var payload struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.NewDecoder(limited).Decode(&payload)
		message := payload.Error.Message
		if message == "" {
			message = res.Status
		}
		return &APIError{Status: res.StatusCode, Code: payload.Error.Code, Message: message}
	}
	if out == nil || res.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(limited).Decode(out); err != nil {
		return fmt.Errorf("decode response from %s: %w", c.server, err)
	}
	return nil
}

func normalizeServer(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("server address is required")
	}
	// A surprisingly common value from import questions and copied config is
	// `http:///host`: the scheme is right, but a path-style slash slipped in
	// before the host. It is unambiguous, so normalize it instead of making the
	// user hunt for a one-character typo.
	for _, scheme := range []string{"http", "https"} {
		prefix := scheme + ":///"
		if strings.HasPrefix(strings.ToLower(raw), prefix) {
			raw = raw[:len(scheme)+3] + strings.TrimLeft(raw[len(prefix):], "/")
			break
		}
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return "", fmt.Errorf("invalid server address %q", raw)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("server address must use http or https")
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return strings.TrimRight(parsed.String(), "/"), nil
}
