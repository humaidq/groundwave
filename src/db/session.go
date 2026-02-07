/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package db

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/flamego/session"
	"github.com/jackc/pgx/v5"
)

// PostgresSessionConfig contains options for the PostgreSQL session store
type PostgresSessionConfig struct {
	// Lifetime is the duration to have no access to a session before being recycled.
	// Default is 30 days (720 hours).
	Lifetime time.Duration
	// TableName is the name of the session table. Default is "flamego_sessions".
	TableName string
	// Encoder is the encoder to encode session data. Default is session.GobEncoder.
	Encoder session.Encoder
	// Decoder is the decoder to decode session data. Default is session.GobDecoder.
	Decoder session.Decoder
}

// PostgresSessionStore implements session.Store interface for PostgreSQL
type PostgresSessionStore struct {
	config  PostgresSessionConfig
	encoder session.Encoder
	decoder session.Decoder
}

// PostgresSessionIniter returns the Initer for the PostgreSQL session store
func PostgresSessionIniter() session.Initer {
	return func(ctx context.Context, args ...interface{}) (session.Store, error) {
		var config PostgresSessionConfig
		if len(args) > 0 {
			var ok bool
			config, ok = args[0].(PostgresSessionConfig)
			if !ok {
				return nil, errors.New("invalid PostgresSessionConfig")
			}
		}

		// Set defaults
		if config.Lifetime == 0 {
			config.Lifetime = 30 * 24 * time.Hour // 30 days
		}
		if config.TableName == "" {
			config.TableName = "flamego_sessions"
		}
		if config.Encoder == nil {
			config.Encoder = session.GobEncoder
		}
		if config.Decoder == nil {
			config.Decoder = session.GobDecoder
		}

		store := &PostgresSessionStore{
			config:  config,
			encoder: config.Encoder,
			decoder: config.Decoder,
		}

		return store, nil
	}
}

// Exist returns true if the session with given ID exists and hasn't expired
func (s *PostgresSessionStore) Exist(ctx context.Context, sid string) bool {
	var exists bool
	err := pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM `+s.config.TableName+` WHERE id = $1 AND expires_at > NOW())`,
		sid,
	).Scan(&exists)
	return err == nil && exists
}

// Read returns the session with given ID. If a session with the ID does not exist,
// a new session with the same ID is created and returned.
func (s *PostgresSessionStore) Read(ctx context.Context, sid string) (session.Session, error) {
	var data []byte
	err := pool.QueryRow(ctx,
		`SELECT data FROM `+s.config.TableName+` WHERE id = $1 AND expires_at > NOW()`,
		sid,
	).Scan(&data)

	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	// Create custom IDWriter that writes to cookie
	idWriter := func(w http.ResponseWriter, r *http.Request, newSid string) {
		// This is handled by Flamego's session middleware
		// We don't need to do anything here
	}

	// If session doesn't exist, create a new one
	if errors.Is(err, pgx.ErrNoRows) || len(data) == 0 {
		return session.NewBaseSession(sid, s.encoder, idWriter), nil
	}

	// Decode session data
	sessionData, err := s.decoder(data)
	if err != nil {
		// If we can't decode, create a new session
		return session.NewBaseSession(sid, s.encoder, idWriter), nil
	}

	return session.NewBaseSessionWithData(sid, s.encoder, idWriter, sessionData), nil
}

// Destroy deletes session with given ID from the session store completely
func (s *PostgresSessionStore) Destroy(ctx context.Context, sid string) error {
	_, err := pool.Exec(ctx,
		`DELETE FROM `+s.config.TableName+` WHERE id = $1`,
		sid,
	)
	return err
}

// Touch updates the expiry time of the session with given ID
func (s *PostgresSessionStore) Touch(ctx context.Context, sid string) error {
	expiresAt := time.Now().Add(s.config.Lifetime)
	_, err := pool.Exec(ctx,
		`UPDATE `+s.config.TableName+` SET expires_at = $1 WHERE id = $2`,
		expiresAt,
		sid,
	)
	return err
}

// Save persists session data to the session store
func (s *PostgresSessionStore) Save(ctx context.Context, sess session.Session) error {
	// Encode session data
	data, err := sess.Encode()
	if err != nil {
		return err
	}

	expiresAt := time.Now().Add(s.config.Lifetime)

	// Use UPSERT (INSERT ... ON CONFLICT UPDATE)
	_, err = pool.Exec(ctx,
		`INSERT INTO `+s.config.TableName+` (id, data, expires_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (id) DO UPDATE SET
			data = EXCLUDED.data,
			expires_at = EXCLUDED.expires_at`,
		sess.ID(),
		data,
		expiresAt,
	)

	return err
}

// SessionData represents a decoded session for the security page
type SessionData struct {
	ID            string
	ExpiresAt     time.Time
	DeviceLabel   string
	DeviceIP      string
	Authenticated bool
	UserID        string
	UserDisplay   string
}

// GC performs a garbage collection operation on the session store
func (s *PostgresSessionStore) GC(ctx context.Context) error {
	_, err := pool.Exec(ctx,
		`DELETE FROM `+s.config.TableName+` WHERE expires_at < NOW()`,
	)
	return err
}

// ListValidSessions returns all valid authenticated sessions for security page
func (s *PostgresSessionStore) ListValidSessions(ctx context.Context) ([]SessionData, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, data, expires_at FROM `+s.config.TableName+` WHERE expires_at > NOW() ORDER BY expires_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []SessionData
	for rows.Next() {
		var id string
		var data []byte
		var expiresAt time.Time

		if err := rows.Scan(&id, &data, &expiresAt); err != nil {
			return nil, err
		}

		// Decode session data
		sessionData, err := s.decoder(data)
		if err != nil {
			// Skip sessions that can't be decoded
			continue
		}

		// Extract device info
		deviceLabel := "Unknown device"
		if val, ok := sessionData["device_label"]; ok && val != nil {
			if str, ok := val.(string); ok && str != "" {
				deviceLabel = str
			}
		}

		deviceIP := "Unknown IP"
		if val, ok := sessionData["device_ip"]; ok && val != nil {
			if str, ok := val.(string); ok && str != "" {
				deviceIP = str
			}
		}

		// Check authentication status
		authenticated := false
		if val, ok := sessionData["authenticated"]; ok && val != nil {
			if auth, ok := val.(bool); ok {
				authenticated = auth
			}
		}

		userID := ""
		if val, ok := sessionData["user_id"]; ok && val != nil {
			if str, ok := val.(string); ok {
				userID = str
			}
		}

		userDisplay := ""
		if val, ok := sessionData["user_display_name"]; ok && val != nil {
			if str, ok := val.(string); ok {
				userDisplay = str
			}
		}

		// Only include authenticated sessions
		if !authenticated {
			continue
		}

		sessions = append(sessions, SessionData{
			ID:            id,
			ExpiresAt:     expiresAt,
			DeviceLabel:   deviceLabel,
			DeviceIP:      deviceIP,
			Authenticated: authenticated,
			UserID:        userID,
			UserDisplay:   userDisplay,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return sessions, nil
}

// InvalidateOtherSessions deletes all authenticated sessions except the current one.
func (s *PostgresSessionStore) InvalidateOtherSessions(ctx context.Context, currentID string, userID string) (int, error) {
	sessions, err := s.ListValidSessions(ctx)
	if err != nil {
		return 0, err
	}

	deleted := 0
	for _, sess := range sessions {
		if sess.ID == currentID {
			continue
		}
		if userID != "" && sess.UserID != "" && sess.UserID != userID {
			continue
		}
		if err := s.Destroy(ctx, sess.ID); err != nil {
			return deleted, err
		}
		deleted++
	}

	return deleted, nil
}
