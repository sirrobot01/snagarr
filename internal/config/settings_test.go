package config

import "testing"

// The operator's own key always beats the embedded default, and either one
// makes TMDB configured.
func TestTMDBKeyResolution(t *testing.T) {
	old := DefaultTMDBKey
	t.Cleanup(func() { DefaultTMDBKey = old })

	var s TMDBSettings

	DefaultTMDBKey = ""
	if s.Configured() {
		t.Error("configured with no key anywhere")
	}

	DefaultTMDBKey = "builtin"
	if got := s.Key(); got != "builtin" {
		t.Errorf("Key() = %q, want the built-in key", got)
	}
	if !s.Configured() {
		t.Error("not configured with the built-in key present")
	}

	s.APIKey = "own"
	if got := s.Key(); got != "own" {
		t.Errorf("Key() = %q, want the operator's key to win", got)
	}
}
