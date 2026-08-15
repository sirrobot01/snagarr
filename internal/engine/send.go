package engine

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/sirrobot01/snagarr/internal/config"
	"github.com/sirrobot01/snagarr/internal/integration"
	"github.com/sirrobot01/snagarr/internal/store"
)

// Sender hands resolved titles to the household's services. The HTTP API, the
// automatic send after a capture resolves, and the Telegram bot all go through
// it, so every path reaches the same service with the same options.
type Sender struct {
	store      *store.Store
	settings   *config.Manager
	reconciler *Reconciler
	log        *slog.Logger
}

func NewSender(s *store.Store, settings *config.Manager, reconciler *Reconciler, log *slog.Logger) *Sender {
	return &Sender{store: s, settings: settings, reconciler: reconciler, log: log}
}

// ErrNoTarget means nobody in the owner list has the requested service.
var ErrNoTarget = errors.New("nobody has that service configured")

// ErrUnknownTarget means the target is not radarr, sonarr or overseerr.
var ErrUnknownTarget = errors.New("unknown send target")

// Household builds every enabled service in the install. A record that cannot
// be decoded is reported and left out, so one bad row never blocks an action.
func (s *Sender) Household(ctx context.Context) (integration.Household, error) {
	services, err := s.store.Services(ctx)
	if err != nil {
		return integration.Household{}, err
	}
	house, err := integration.BuildHousehold(services)
	if err != nil {
		s.log.Warn("some services could not be built", "error", err)
	}
	return house, nil
}

// Owners is the order a personal action searches in: the caller's own service
// first, then the item capturer's, then any admin's. An action must not fail
// for want of a service somebody in the household already has.
func (s *Sender) Owners(ctx context.Context, caller *store.User, capturer int64) []int64 {
	// A member may only spend their own services. Falling back to an admin's
	// Radarr would let anyone with a token push to it, which is the permission
	// the admin-only route used to protect.
	if caller.Role != store.RoleAdmin {
		return []int64{caller.ID}
	}
	owners := []int64{caller.ID}
	if capturer != 0 && capturer != caller.ID {
		owners = append(owners, capturer)
	}
	users, err := s.store.Users(ctx)
	if err != nil {
		s.log.Warn("could not list household members", "error", err)
		return owners
	}
	for _, u := range users {
		if u.Role == store.RoleAdmin && u.ID != caller.ID {
			owners = append(owners, u.ID)
		}
	}
	return owners
}

// Send hands one title to one service and reports the state that leaves the
// item in.
func (s *Sender) Send(ctx context.Context, house integration.Household, it store.Item,
	target string, owners []int64) (store.Status, error) {
	var err error
	status := store.StatusMonitored

	switch target {
	case "radarr":
		radarr := house.RadarrFor(owners...)
		if radarr == nil {
			return "", ErrNoTarget
		}
		_, err = radarr.Client.Add(ctx, int(it.TMDBID), integration.AddOptions{
			QualityProfileID: radarr.Config.QualityProfileID,
			RootFolder:       radarr.Config.RootFolder,
			Monitor:          true,
			SearchOnAdd:      radarr.Config.SearchOnAdd,
		})

	case "sonarr":
		sonarr := house.SonarrFor(owners...)
		if sonarr == nil {
			return "", ErrNoTarget
		}
		ids := integration.ArrExternalIDs{TMDBID: int(it.TMDBID), Title: it.Title, Year: it.Year}
		// Sonarr keys on TVDB, so the TMDB ID has to be translated first.
		if catalogue := s.tmdb(); catalogue != nil {
			if external, idErr := catalogue.ExternalIDs(ctx, it.MediaType, int(it.TMDBID)); idErr == nil {
				ids.TVDBID, ids.IMDBID = external.TVDBID, external.IMDBID
			}
		}
		_, err = sonarr.Client.Add(ctx, ids, integration.AddOptions{
			QualityProfileID: sonarr.Config.QualityProfileID,
			RootFolder:       sonarr.Config.RootFolder,
			Monitor:          true,
			SearchOnAdd:      sonarr.Config.SearchOnAdd,
			SeasonFolder:     sonarr.Config.SeasonFolder,
		})

	case "overseerr":
		overseerr := house.OverseerrFor(owners...)
		if overseerr == nil {
			return "", ErrNoTarget
		}
		_, err = overseerr.Client.Create(ctx, int(it.TMDBID), it.MediaType)
		status = store.StatusRequested

	default:
		return "", ErrUnknownTarget
	}

	// A title the service already tracks is the outcome the user wanted.
	if err != nil && !errors.Is(err, integration.ErrAlreadyAdded) {
		return "", err
	}
	return status, nil
}

// AutoSend is "add to Radarr or Sonarr by default": a title that resolves to
// something nobody owns yet goes straight to the capturer's own download
// manager. It spends nobody else's service, and it never fails a capture — an
// unreachable Radarr leaves the item exactly where the Send button can retry.
func (s *Sender) AutoSend(ctx context.Context, itemID int64) {
	if !s.settings.Get().General.AutoSend {
		return
	}
	it, err := s.store.Item(ctx, itemID)
	if err != nil || it.TMDBID == 0 {
		return
	}
	// Anything already in the library, monitored or requested needs nothing.
	if it.Status != store.StatusNew || it.Archived() {
		return
	}

	target := "radarr"
	if it.MediaType == store.TV {
		target = "sonarr"
	}
	house, err := s.Household(ctx)
	if err != nil {
		s.log.Warn("automatic send could not read services", "item", itemID, "error", err)
		return
	}

	status, err := s.Send(ctx, house, *it, target, []int64{it.CapturedBy})
	if errors.Is(err, ErrNoTarget) {
		return
	}
	if err != nil {
		s.log.Warn("automatic send failed", "item", itemID, "target", target, "error", err)
		return
	}
	if err := s.store.SetStatus(ctx, it.ID, status, time.Time{}); err != nil {
		s.log.Warn("automatic send could not record the state", "item", itemID, "error", err)
		return
	}
	s.log.Info("sent automatically", "item", itemID, "title", it.Title, "target", target)
	s.reconciler.Trigger()
}

func (s *Sender) tmdb() *integration.TMDBClient {
	return integration.TMDB(s.settings.Get(), s.store.HTTPCache())
}
