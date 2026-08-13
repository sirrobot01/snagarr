package integration

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Message is one push.
type Message struct {
	Title    string
	Body     string
	Tags     []string
	Priority int // 1..5, 0 means the server default
	ClickURL string
}

// NtfyClient publishes Snagarr's availability pushes to an ntfy topic.
type NtfyClient struct {
	rest  client
	topic string
}

// NewNtfy returns a publisher for one topic on an ntfy server. token may be empty
// for open servers.
func NewNtfy(serverURL, topic, token string) *NtfyClient {
	header := http.Header{}
	if token != "" {
		header.Set("Authorization", "Bearer "+token)
	}
	return &NtfyClient{
		rest:  client{BaseURL: serverURL, HTTP: &http.Client{Timeout: 20 * time.Second}, Header: header},
		topic: topic,
	}
}

// Send publishes a message. ntfy carries the text as the raw request body and
// everything else as headers, so this does not go through the JSON client.
func (n *NtfyClient) Send(ctx context.Context, m Message) error {
	u, err := url.Parse(n.rest.BaseURL)
	if err != nil {
		return fmt.Errorf("ntfy: parse server url %q: %w", n.rest.BaseURL, err)
	}
	u = u.JoinPath(n.topic)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), strings.NewReader(m.Body))
	if err != nil {
		return fmt.Errorf("ntfy send: %w", err)
	}
	req.Header.Set("User-Agent", UserAgent)
	for k, v := range n.rest.Header {
		req.Header[http.CanonicalHeaderKey(k)] = v
	}
	if m.Title != "" {
		req.Header.Set("Title", m.Title)
	}
	if len(m.Tags) > 0 {
		req.Header.Set("Tags", strings.Join(m.Tags, ","))
	}
	if m.Priority >= 1 && m.Priority <= 5 {
		req.Header.Set("Priority", strconv.Itoa(m.Priority))
	}
	if m.ClickURL != "" {
		req.Header.Set("Click", m.ClickURL)
	}

	resp, err := n.rest.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("ntfy send: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("ntfy send: %w", &HTTPError{Status: resp.StatusCode, Body: string(bytes.TrimSpace(b))})
	}
	return nil
}

// Ping checks that the ntfy server is reachable and healthy.
func (n *NtfyClient) Ping(ctx context.Context) error {
	var body struct {
		Healthy bool `json:"healthy"`
	}
	if err := n.rest.Get(ctx, "/v1/health", nil, &body); err != nil {
		return fmt.Errorf("ntfy ping: %w", err)
	}
	if !body.Healthy {
		return errors.New("ntfy ping: server reports unhealthy")
	}
	return nil
}
