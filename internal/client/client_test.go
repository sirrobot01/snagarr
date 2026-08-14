package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCaptureSendsCLIPayloadAndToken(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("authorization = %q", got)
		}
		var input CaptureInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Fatal(err)
		}
		if input.Query != "Sinners" || input.Source != "cli" {
			t.Fatalf("input = %#v", input)
		}
		_ = json.NewEncoder(w).Encode(Item{ID: 7, Title: "Sinners", Status: "needs_review"})
	}))
	defer server.Close()

	client, err := New(server.URL+"/", "secret")
	if err != nil {
		t.Fatal(err)
	}
	item, err := client.Capture(context.Background(), CaptureInput{Query: "Sinners", Source: "cli"})
	if err != nil {
		t.Fatal(err)
	}
	if item.ID != 7 || item.Title != "Sinners" {
		t.Fatalf("item = %#v", item)
	}
}

func TestAPIErrorKeepsServerMessage(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"code":"unauthorized","message":"the token is not valid"}}`))
	}))
	defer server.Close()

	client, err := New(server.URL, "wrong")
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Me(context.Background())
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error = %T %v", err, err)
	}
	if apiErr.Status != http.StatusUnauthorized || apiErr.Message != "the token is not valid" {
		t.Fatalf("api error = %#v", apiErr)
	}
}

func TestNewNormalizesBareServerAddress(t *testing.T) {
	t.Parallel()

	client, err := New("localhost:8080/", "")
	if err != nil {
		t.Fatal(err)
	}
	if got := client.Server(); got != "http://localhost:8080" {
		t.Fatalf("server = %q", got)
	}
}

func TestNewNormalizesExtraSlashAfterScheme(t *testing.T) {
	t.Parallel()

	client, err := New("http:///localhost:8080", "")
	if err != nil {
		t.Fatal(err)
	}
	if got := client.Server(); got != "http://localhost:8080" {
		t.Fatalf("server = %q", got)
	}
}
