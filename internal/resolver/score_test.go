package resolver

import "testing"

func TestScoreRanksExactTitlesHighest(t *testing.T) {
	tests := []struct {
		name  string
		query string
		title string
		year  int
		want  float64 // minimum acceptable score
	}{
		{"exact", "sinners", "Sinners", 2025, 0.95},
		{"case and punctuation", "dune part two", "Dune: Part Two", 2024, 0.95},
		{"diacritics folded", "amelie", "Amélie", 2001, 0.95},
		{"prefix typed so far", "sinn", "Sinners", 2025, 0.75},
		{"year agrees", "sinners 2025", "Sinners", 2025, 0.95},
		{"loose word overlap", "the substance movie", "The Substance", 2024, 0.30},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Score(tt.query, tt.title, tt.year, 10); got < tt.want {
				t.Errorf("Score(%q, %q) = %.2f, want at least %.2f", tt.query, tt.title, got, tt.want)
			}
		})
	}
}

func TestScorePenalisesWrongYear(t *testing.T) {
	right := Score("sinners 2025", "Sinners", 2025, 10)
	wrong := Score("sinners 2025", "Sinners", 2006, 10)
	if wrong >= right {
		t.Errorf("wrong-year score %.2f is not below right-year score %.2f", wrong, right)
	}
	if wrong > 0.75 {
		t.Errorf("wrong-year score %.2f is high enough to auto-resolve", wrong)
	}
}

func TestScoreIgnoresUnrelatedTitles(t *testing.T) {
	if got := Score("sinners", "The Bear", 2022, 50); got > 0.2 {
		t.Errorf("unrelated title scored %.2f, want under 0.2", got)
	}
}

func TestDecide(t *testing.T) {
	tests := []struct {
		name      string
		scores    []float64
		confident bool
	}{
		{"clear winner", []float64{0.98, 0.42}, true},
		{"only candidate", []float64{0.91}, true},
		{"two plausible Sinners", []float64{0.95, 0.90}, false},
		{"nothing convincing", []float64{0.55, 0.20}, false},
		{"no candidates", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, confident := Decide(tt.scores)
			if confident != tt.confident {
				t.Errorf("Decide(%v) confident = %v, want %v", tt.scores, confident, tt.confident)
			}
		})
	}
}
