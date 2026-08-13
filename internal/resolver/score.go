// Package resolver turns a raw capture into a TMDB entry, or into a set of
// candidates when it cannot decide. It never discards a capture.
package resolver

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// A capture resolves on its own only when the leader is both strong and clearly
// ahead. Anything else goes to Needs Review, where one tap settles it — that is
// cheaper for the user than silently picking the wrong Sinners.
const (
	autoResolveScore = 0.80
	ambiguityMargin  = 0.12
)

var yearPattern = regexp.MustCompile(`\b(19|20)\d{2}\b`)

// Score rates a candidate title against the captured text, from 0 to 1.
func Score(query, title string, year int, popularity float64) float64 {
	queryYear := extractYear(query)
	q := normalize(stripYear(query))
	t := normalize(title)
	if q == "" || t == "" {
		return 0
	}

	var score float64
	switch {
	case q == t:
		score = 1.0
	case strings.HasPrefix(t, q):
		score = 0.85
	case strings.HasPrefix(q, t):
		score = 0.80
	case strings.Contains(t, q) || strings.Contains(q, t):
		score = 0.70
	default:
		score = 0.65 * tokenOverlap(q, t)
	}

	switch {
	case queryYear == 0 || year == 0:
		// No year to compare; leave the title score alone.
	case queryYear == year:
		score += 0.10
	default:
		score -= 0.35
	}

	// Popularity only breaks ties between titles that already match.
	score += math.Min(0.05, math.Log1p(popularity)/200)

	return math.Max(0, math.Min(1, score))
}

// Decide reports the winner of a ranked candidate list, and whether it is safe
// to resolve without asking. Candidates must be sorted by descending score.
func Decide(scores []float64) (index int, confident bool) {
	if len(scores) == 0 {
		return -1, false
	}
	if scores[0] < autoResolveScore {
		return 0, false
	}
	if len(scores) > 1 && scores[0]-scores[1] < ambiguityMargin {
		return 0, false
	}
	return 0, true
}

func tokenOverlap(a, b string) float64 {
	fieldsA, fieldsB := strings.Fields(a), strings.Fields(b)
	if len(fieldsA) == 0 || len(fieldsB) == 0 {
		return 0
	}
	inB := make(map[string]bool, len(fieldsB))
	for _, f := range fieldsB {
		inB[f] = true
	}
	var shared int
	for _, f := range fieldsA {
		if inB[f] {
			shared++
			delete(inB, f)
		}
	}
	union := len(fieldsA) + len(fieldsB) - shared
	return float64(shared) / float64(union)
}

// normalize folds a title down to comparable text: diacritics removed, case
// dropped, punctuation gone. "Amélie" and "amelie" must match.
func normalize(s string) string {
	folded, _, err := transform.String(
		transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC), s)
	if err != nil {
		folded = s
	}
	var b strings.Builder
	var space bool
	for _, r := range strings.ToLower(folded) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if space && b.Len() > 0 {
				b.WriteByte(' ')
			}
			space = false
			b.WriteRune(r)
		default:
			space = true
		}
	}
	return b.String()
}

func extractYear(s string) int {
	match := yearPattern.FindString(s)
	if match == "" {
		return 0
	}
	year, _ := strconv.Atoi(match)
	return year
}

func stripYear(s string) string { return yearPattern.ReplaceAllString(s, " ") }
