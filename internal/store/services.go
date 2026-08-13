package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ServiceKind names the integration a service record configures. Snagarr can
// build a client for each of them and for nothing else.
type ServiceKind string

const (
	KindPlex      ServiceKind = "plex"
	KindEmby      ServiceKind = "emby"
	KindJellyfin  ServiceKind = "jellyfin"
	KindRadarr    ServiceKind = "radarr"
	KindSonarr    ServiceKind = "sonarr"
	KindOverseerr ServiceKind = "overseerr"
	KindNtfy      ServiceKind = "ntfy"
)

// Valid reports whether k is a kind Snagarr knows.
func (k ServiceKind) Valid() bool {
	switch k {
	case KindPlex, KindEmby, KindJellyfin, KindRadarr, KindSonarr, KindOverseerr, KindNtfy:
		return true
	}
	return false
}

// Library reports whether k is a media server. The three media servers differ
// only in how their client is built; everything downstream treats them alike.
func (k ServiceKind) Library() bool {
	return k == KindPlex || k == KindEmby || k == KindJellyfin
}

// Service is one member's connection to one external service. Config carries
// the kind's own JSON document — clear text here, encrypted at rest with the
// same key as the settings blob.
type Service struct {
	ID        int64
	UserID    int64
	Kind      ServiceKind
	Name      string
	Config    []byte
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CreateService stores a new service and fills in its ID and timestamps.
func (s *Store) CreateService(ctx context.Context, svc *Service) error {
	sealed, err := s.encrypt(svc.Config)
	if err != nil {
		return fmt.Errorf("encrypt service config: %w", err)
	}
	svc.CreatedAt = time.Now().UTC()
	svc.UpdatedAt = svc.CreatedAt

	res, err := s.db.ExecContext(ctx,
		`INSERT INTO services (user_id, kind, name, config, enabled, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		svc.UserID, svc.Kind, svc.Name, sealed, svc.Enabled, svc.CreatedAt, svc.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	svc.ID, err = res.LastInsertId()
	return err
}

// UpdateService writes back the whole record. The owner never changes: a
// service belongs to whoever created it.
func (s *Store) UpdateService(ctx context.Context, svc *Service) error {
	sealed, err := s.encrypt(svc.Config)
	if err != nil {
		return fmt.Errorf("encrypt service config: %w", err)
	}
	svc.UpdatedAt = time.Now().UTC()

	res, err := s.db.ExecContext(ctx,
		`UPDATE services SET name = ?, config = ?, enabled = ?, updated_at = ? WHERE id = ?`,
		svc.Name, sealed, svc.Enabled, svc.UpdatedAt, svc.ID)
	if err != nil {
		return fmt.Errorf("update service: %w", err)
	}
	return affected(res)
}

// DeleteService removes a service. Its index rows go with it, so the household
// state stops counting a server nobody can reach any more.
func (s *Store) DeleteService(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM services WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete service: %w", err)
	}
	return affected(res)
}

const serviceColumns = `id, user_id, kind, name, config, enabled, created_at, updated_at`

func (s *Store) scanService(row interface{ Scan(...any) error }) (*Service, error) {
	var svc Service
	var sealed []byte
	if err := row.Scan(&svc.ID, &svc.UserID, &svc.Kind, &svc.Name, &sealed,
		&svc.Enabled, &svc.CreatedAt, &svc.UpdatedAt); err != nil {
		return nil, err
	}
	config, err := s.decrypt(sealed)
	if err != nil {
		return nil, fmt.Errorf("decrypt service %d config: %w", svc.ID, err)
	}
	svc.Config = config
	return &svc, nil
}

// Service returns one service by ID, enabled or not.
func (s *Store) Service(ctx context.Context, id int64) (*Service, error) {
	svc, err := s.scanService(s.db.QueryRowContext(ctx,
		`SELECT `+serviceColumns+` FROM services WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get service: %w", err)
	}
	return svc, nil
}

// Services returns every enabled service in the household. State is the union
// of what all of them report, so the reconcile loop needs all of them at once.
func (s *Store) Services(ctx context.Context) ([]Service, error) {
	return s.services(ctx, `SELECT `+serviceColumns+` FROM services WHERE enabled = 1 ORDER BY id`)
}

// UserServices returns one member's services, disabled ones included: the
// settings UI has to show a service that was switched off.
func (s *Store) UserServices(ctx context.Context, userID int64) ([]Service, error) {
	return s.services(ctx,
		`SELECT `+serviceColumns+` FROM services WHERE user_id = ? ORDER BY kind, name, id`, userID)
}

func (s *Store) services(ctx context.Context, query string, args ...any) ([]Service, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	defer rows.Close()

	var services []Service
	for rows.Next() {
		svc, err := s.scanService(rows)
		if err != nil {
			return nil, fmt.Errorf("list services: %w", err)
		}
		services = append(services, *svc)
	}
	return services, rows.Err()
}
