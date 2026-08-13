package integration

import (
	"time"

	"github.com/sirrobot01/snagarr/internal/store"
)

// The media server clients read library contents and keep the Snagged
// collection in sync. Plex and Emby/Jellyfin expose the same method set.

// LibraryItem is one title in a media server library.
type LibraryItem struct {
	ProviderItemID string
	TMDBID         int
	IMDBID         string
	TVDBID         int
	Type           store.MediaType
	Title          string
	Year           int
	AddedAt        time.Time
}

// Section is a movie or show library on the server.
type Section struct {
	ID, Title string
	Type      store.MediaType
}

// diffMembers compares the collection membership a server reports against the
// membership Snagarr wants.
func diffMembers(current, desired []string) (add, remove []string) {
	have := make(map[string]bool, len(current))
	for _, id := range current {
		have[id] = true
	}
	want := make(map[string]bool, len(desired))
	for _, id := range desired {
		if want[id] {
			continue
		}
		want[id] = true
		if !have[id] {
			add = append(add, id)
		}
	}
	for _, id := range current {
		if !want[id] {
			remove = append(remove, id)
		}
	}
	return add, remove
}
