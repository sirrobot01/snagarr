// Package media holds the vocabulary every integration client shares. It has no
// dependencies so that the HTTP clients never pull in storage.
package media

// Type is the kind of title Snagarr tracks. TMDB, Radarr, Sonarr and every
// library provider split on this one axis.
type Type string

const (
	Movie Type = "movie"
	TV    Type = "tv"
)

func (t Type) Valid() bool { return t == Movie || t == TV }
