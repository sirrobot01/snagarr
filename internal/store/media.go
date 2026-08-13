package store

// MediaType is the kind of title Snagarr tracks. TMDB, Radarr, Sonarr and every
// library provider split on this one axis.
type MediaType string

const (
	Movie MediaType = "movie"
	TV    MediaType = "tv"
)

func (t MediaType) Valid() bool { return t == Movie || t == TV }
