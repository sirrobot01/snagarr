package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sirrobot01/snagarr/internal/client"
)

func TestLoginWithTokenVerifiesAndSavesSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/me" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("authorization = %q", got)
		}
		_ = json.NewEncoder(w).Encode(client.User{ID: 1, Username: "amina", Role: "admin"})
	}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "client.json")
	t.Setenv(configPathEnv, path)
	var output bytes.Buffer
	command := New()
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs([]string{"login", server.URL, "--token", "secret", "--no-color"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}

	if got := output.String(); !strings.Contains(got, "Logged in as @amina") || strings.Contains(got, "secret") {
		t.Fatalf("output = %q", got)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var saved savedConfig
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Server != server.URL || saved.Token != "secret" {
		t.Fatalf("config = %#v", saved)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if permissions := info.Mode().Perm(); permissions != 0o600 {
		t.Fatalf("permissions = %o", permissions)
	}
}

func TestSnagPrintsCleanHumanOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/capture" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		var input client.CaptureInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Fatal(err)
		}
		if input.Query != "The Bear" || input.Source != "cli" || input.Note != "weekend" {
			t.Fatalf("input = %#v", input)
		}
		note := "weekend"
		_ = json.NewEncoder(w).Encode(client.Item{ID: 42, Title: "The Bear", Status: "needs_review", Note: &note})
	}))
	defer server.Close()

	var output bytes.Buffer
	command := New()
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs([]string{
		"snag", "The", "Bear", "--note", "weekend", "--server", server.URL, "--token", "secret", "--no-color",
	})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}

	got := output.String()
	for _, expected := range []string{"✓  The Bear", "Status", "Needs review", "Item", "#42", "Note", "weekend"} {
		if !strings.Contains(got, expected) {
			t.Fatalf("output %q does not contain %q", got, expected)
		}
	}
}

func TestListJSONIsMachineReadable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(client.ItemsResponse{
			Items: []client.Item{{ID: 3, Title: "Sinners", Status: "available", CapturedAt: time.Now()}},
			Total: 1,
		})
	}))
	defer server.Close()

	var output bytes.Buffer
	command := New()
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs([]string{"list", "--json", "--server", server.URL, "--token", "secret"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	var response client.ItemsResponse
	if err := json.Unmarshal(output.Bytes(), &response); err != nil {
		t.Fatalf("invalid JSON %q: %v", output.String(), err)
	}
	if response.Total != 1 || response.Items[0].Title != "Sinners" {
		t.Fatalf("response = %#v", response)
	}
}
