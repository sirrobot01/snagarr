package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/sirrobot01/snagarr/internal/arr"
	"github.com/sirrobot01/snagarr/internal/clients"
	"github.com/sirrobot01/snagarr/internal/config"
	"github.com/sirrobot01/snagarr/internal/store"
)

// defaultServiceName matches the name the settings migration and the
// environment seeding both use, so an install that never opens this API still
// reads the same as one that does.
const defaultServiceName = "Default"

type serviceDTO struct {
	ID        int64             `json:"id"`
	UserID    int64             `json:"user_id"`
	Kind      store.ServiceKind `json:"kind"`
	Name      string            `json:"name"`
	Config    map[string]any    `json:"config"`
	Enabled   bool              `json:"enabled"`
	Locked    bool              `json:"locked"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

func (s *Server) newServiceDTO(svc store.Service) (serviceDTO, error) {
	masked, err := maskedConfig(svc.Config)
	if err != nil {
		return serviceDTO{}, err
	}
	return serviceDTO{
		ID: svc.ID, UserID: svc.UserID, Kind: svc.Kind, Name: svc.Name,
		Config: masked, Enabled: svc.Enabled, Locked: s.settings.LockedServices()[svc.ID],
		CreatedAt: svc.CreatedAt, UpdatedAt: svc.UpdatedAt,
	}, nil
}

// maskedConfig hides the credentials in a service config. The client echoes the
// mask back unchanged and mergeConfig restores the real value, so a secret only
// ever leaves the process once.
func maskedConfig(raw []byte) (map[string]any, error) {
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, fmt.Errorf("decode service config: %w", err)
	}
	for _, name := range config.ServiceSecrets {
		if secret, ok := fields[name].(string); ok {
			fields[name] = config.Mask(secret)
		}
	}
	return fields, nil
}

// mergeConfig lays a client's document over the stored one: a field the client
// omits keeps what it had, and an echoed mask leaves the secret alone.
func mergeConfig(stored, patch []byte) ([]byte, error) {
	fields := map[string]any{}
	if err := json.Unmarshal(stored, &fields); err != nil {
		return nil, fmt.Errorf("decode stored config: %w", err)
	}
	var incoming map[string]any
	if err := json.Unmarshal(patch, &incoming); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	for name, value := range incoming {
		if secret, ok := value.(string); ok && config.IsMasked(secret) {
			continue
		}
		fields[name] = value
	}
	return json.Marshal(fields)
}

func (s *Server) listServices(w http.ResponseWriter, r *http.Request) {
	s.writeServices(w, r, userFrom(r).ID)
}

// listUserServices is the admin view of somebody else's stack.
func (s *Server) listUserServices(w http.ResponseWriter, r *http.Request) {
	u, ok := s.loadUser(w, r)
	if !ok {
		return
	}
	s.writeServices(w, r, u.ID)
}

func (s *Server) writeServices(w http.ResponseWriter, r *http.Request, userID int64) {
	services, err := s.store.UserServices(r.Context(), userID)
	if err != nil {
		s.writeStoreError(w, err, "services")
		return
	}
	dtos := make([]serviceDTO, 0, len(services))
	for _, svc := range services {
		dto, err := s.newServiceDTO(svc)
		if err != nil {
			s.log.Error("could not encode service", "service", svc.ID, "error", err)
			writeError(w, http.StatusInternalServerError, codeInternal, "services could not be read")
			return
		}
		dtos = append(dtos, dto)
	}
	writeJSON(w, http.StatusOK, map[string]any{"services": dtos})
}

func (s *Server) createService(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Kind    store.ServiceKind `json:"kind"`
		Name    string            `json:"name"`
		Config  json.RawMessage   `json:"config"`
		Enabled *bool             `json:"enabled"`
	}
	if !decode(w, r, &req) {
		return
	}
	if !req.Kind.Valid() {
		writeError(w, http.StatusBadRequest, codeBadRequest, "kind %q is not a service Snagarr knows", req.Kind)
		return
	}
	if req.Name == "" {
		req.Name = defaultServiceName
	}
	if len(req.Config) == 0 {
		req.Config = json.RawMessage("{}")
	}
	// A new service starts from its kind's defaults, so the client never has to
	// know the collection is called Snagged.
	merged, err := mergeConfig(config.DefaultServiceConfig(req.Kind), req.Config)
	if err != nil {
		writeError(w, http.StatusBadRequest, codeBadRequest, "%v", err)
		return
	}

	svc := &store.Service{
		UserID: userFrom(r).ID, Kind: req.Kind, Name: req.Name,
		Config: merged, Enabled: req.Enabled == nil || *req.Enabled,
	}
	if !s.nameAvailable(w, r, *svc) {
		return
	}
	if err := s.store.CreateService(r.Context(), svc); err != nil {
		s.writeStoreError(w, err, "service")
		return
	}
	// New credentials usually mean new indexes to build.
	s.engine.Trigger()
	s.respondWithService(w, *svc, http.StatusCreated)
}

func (s *Server) updateService(w http.ResponseWriter, r *http.Request) {
	svc, ok := s.loadService(w, r)
	if !ok {
		return
	}
	var req struct {
		Name    *string         `json:"name"`
		Config  json.RawMessage `json:"config"`
		Enabled *bool           `json:"enabled"`
	}
	if !decode(w, r, &req) {
		return
	}

	if req.Name != nil {
		if *req.Name == "" {
			writeError(w, http.StatusBadRequest, codeBadRequest, "name cannot be empty")
			return
		}
		svc.Name = *req.Name
	}
	if req.Enabled != nil {
		svc.Enabled = *req.Enabled
	}
	if len(req.Config) > 0 {
		merged, err := mergeConfig(svc.Config, req.Config)
		if err != nil {
			writeError(w, http.StatusBadRequest, codeBadRequest, "%v", err)
			return
		}
		svc.Config = merged
	}
	if !s.nameAvailable(w, r, *svc) {
		return
	}
	if err := s.store.UpdateService(r.Context(), svc); err != nil {
		s.writeStoreError(w, err, "service")
		return
	}
	s.engine.Trigger()
	s.respondWithService(w, *svc, http.StatusOK)
}

func (s *Server) deleteService(w http.ResponseWriter, r *http.Request) {
	svc, ok := s.loadService(w, r)
	if !ok {
		return
	}
	if err := s.store.DeleteService(r.Context(), svc.ID); err != nil {
		s.writeStoreError(w, err, "service")
		return
	}
	// The index rows went with it, so the household state has to be recomputed.
	s.engine.Trigger()
	w.WriteHeader(http.StatusNoContent)
}

// testService reports a service's reachability verbatim, so the settings card
// can show the upstream failure rather than a generic error.
func (s *Server) testService(w http.ResponseWriter, r *http.Request) {
	svc, ok := s.loadService(w, r)
	if !ok {
		return
	}
	message, err := ping(r.Context(), *svc)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": message})
}

func ping(ctx context.Context, svc store.Service) (string, error) {
	switch {
	case svc.Kind.Library():
		built, err := clients.BuildLibrary(svc)
		if err != nil {
			return "", err
		}
		return built.Client.Ping(ctx)
	case svc.Kind == store.KindRadarr:
		built, err := clients.BuildRadarr(svc)
		if err != nil {
			return "", err
		}
		return built.Client.Ping(ctx)
	case svc.Kind == store.KindSonarr:
		built, err := clients.BuildSonarr(svc)
		if err != nil {
			return "", err
		}
		return built.Client.Ping(ctx)
	case svc.Kind == store.KindOverseerr:
		built, err := clients.BuildOverseerr(svc)
		if err != nil {
			return "", err
		}
		return built.Client.Ping(ctx)
	case svc.Kind == store.KindNtfy:
		built, err := clients.BuildNtfy(svc)
		if err != nil {
			return "", err
		}
		return "OK", built.Client.Ping(ctx)
	default:
		return "", fmt.Errorf("kind %q cannot be tested", svc.Kind)
	}
}

// serviceOptions lets the settings UI offer real quality profiles, root folders
// and library sections instead of asking for numeric IDs.
func (s *Server) serviceOptions(w http.ResponseWriter, r *http.Request) {
	svc, ok := s.loadService(w, r)
	if !ok {
		return
	}
	ctx := r.Context()

	switch {
	case svc.Kind == store.KindRadarr:
		built, err := clients.BuildRadarr(*svc)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, codeNotConfigured, "%v", err)
			return
		}
		s.writeArrOptions(ctx, w, built.Client)
	case svc.Kind == store.KindSonarr:
		built, err := clients.BuildSonarr(*svc)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, codeNotConfigured, "%v", err)
			return
		}
		s.writeArrOptions(ctx, w, built.Client)
	case svc.Kind.Library():
		built, err := clients.BuildLibrary(*svc)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, codeNotConfigured, "%v", err)
			return
		}
		sections, err := built.Client.Sections(ctx)
		if err != nil {
			writeError(w, http.StatusBadGateway, codeUpstreamError, "%v", err)
			return
		}
		rows := make([]map[string]any, 0, len(sections))
		for _, sec := range sections {
			rows = append(rows, map[string]any{"id": sec.ID, "title": sec.Title, "type": sec.Type})
		}
		writeJSON(w, http.StatusOK, map[string]any{"sections": rows})
	default:
		writeError(w, http.StatusBadRequest, codeBadRequest, "a %s service has no options", svc.Kind)
	}
}

// arrOptions is satisfied by both *arr.Radarr and *arr.Sonarr, which expose the
// same two lookups.
type arrOptions interface {
	QualityProfiles(ctx context.Context) ([]arr.Profile, error)
	RootFolders(ctx context.Context) ([]arr.RootFolder, error)
}

func (s *Server) writeArrOptions(ctx context.Context, w http.ResponseWriter, c arrOptions) {
	profiles, err := c.QualityProfiles(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, codeUpstreamError, "%v", err)
		return
	}
	folders, err := c.RootFolders(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, codeUpstreamError, "%v", err)
		return
	}

	profileRows := make([]map[string]any, 0, len(profiles))
	for _, p := range profiles {
		profileRows = append(profileRows, map[string]any{"id": p.ID, "name": p.Name})
	}
	folderRows := make([]map[string]any, 0, len(folders))
	for _, f := range folders {
		folderRows = append(folderRows, map[string]any{"path": f.Path, "free_space": f.FreeSpace})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"quality_profiles": profileRows,
		"root_folders":     folderRows,
	})
}

// loadService reads the service named in the path and applies the ownership
// rule: a member reaches only their own services, an admin reaches everybody's.
func (s *Server) loadService(w http.ResponseWriter, r *http.Request) (*store.Service, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, codeBadRequest, "service id must be a number")
		return nil, false
	}
	svc, err := s.store.Service(r.Context(), id)
	if err != nil {
		s.writeStoreError(w, err, "service")
		return nil, false
	}
	u := userFrom(r)
	if u.Role != store.RoleAdmin && svc.UserID != u.ID {
		writeError(w, http.StatusForbidden, codeForbidden, "only an admin can reach another member's service")
		return nil, false
	}
	return svc, true
}

// nameAvailable enforces one name per kind per member before SQLite does, so a
// duplicate reads as a conflict rather than an internal error.
func (s *Server) nameAvailable(w http.ResponseWriter, r *http.Request, svc store.Service) bool {
	owned, err := s.store.UserServices(r.Context(), svc.UserID)
	if err != nil {
		s.writeStoreError(w, err, "services")
		return false
	}
	for _, other := range owned {
		if other.ID != svc.ID && other.Kind == svc.Kind && other.Name == svc.Name {
			writeError(w, http.StatusConflict, codeConflict,
				"a %s service called %q already exists", svc.Kind, svc.Name)
			return false
		}
	}
	return true
}

func (s *Server) respondWithService(w http.ResponseWriter, svc store.Service, status int) {
	dto, err := s.newServiceDTO(svc)
	if err != nil {
		s.log.Error("could not encode service", "service", svc.ID, "error", err)
		writeError(w, http.StatusInternalServerError, codeInternal, "service could not be read")
		return
	}
	writeJSON(w, status, dto)
}

// household builds every enabled service in the install. State is the union of
// what all of them report, and a personal action falls back through them, so
// both need the whole set.
func (s *Server) household(ctx context.Context) (clients.Household, error) {
	services, err := s.store.Services(ctx)
	if err != nil {
		return clients.Household{}, err
	}
	house, err := clients.BuildHousehold(services)
	if err != nil {
		s.log.Warn("some services could not be built", "error", err)
	}
	return house, nil
}

// serviceOwners is the order a personal action searches in: the caller's own
// service first, then the item capturer's, then any admin's. An action must not
// fail for want of a service somebody in the household already has.
func (s *Server) serviceOwners(ctx context.Context, caller, capturer int64) []int64 {
	owners := []int64{caller}
	if capturer != 0 && capturer != caller {
		owners = append(owners, capturer)
	}
	users, err := s.store.Users(ctx)
	if err != nil {
		s.log.Warn("could not list household members", "error", err)
		return owners
	}
	for _, u := range users {
		if u.Role == store.RoleAdmin {
			owners = append(owners, u.ID)
		}
	}
	return owners
}
