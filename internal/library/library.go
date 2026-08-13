// Package library reads media server contents and keeps the Snagged collection
// in sync. Plex and Emby/Jellyfin expose the same method set.
package library

import (
	"time"

	"github.com/sirrobot01/snagarr/internal/media"
)

// Item is one title in a media server library.
type Item struct {
	ProviderItemID string
	TMDBID         int
	IMDBID         string
	TVDBID         int
	Type           media.Type
	Title          string
	Year           int
	AddedAt        time.Time
}

// Section is a movie or show library on the server.
type Section struct {
	ID, Title string
	Type      media.Type
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
