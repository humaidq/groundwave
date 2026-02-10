/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// CreateQSLCardRequestInput is the payload for creating a physical QSL card request.
type CreateQSLCardRequestInput struct {
	QSOID          string
	RequesterName  string
	MailingAddress string
	Note           string
}

// QSLCardRequestListItem is a pending QSL card request enriched with QSO details.
type QSLCardRequestListItem struct {
	ID             uuid.UUID `db:"id"`
	QSOID          uuid.UUID `db:"qso_id"`
	Call           string    `db:"call"`
	QSODate        time.Time `db:"qso_date"`
	TimeOn         time.Time `db:"time_on"`
	Mode           string    `db:"mode"`
	Band           *string   `db:"band"`
	Country        *string   `db:"country"`
	RequesterName  *string   `db:"requester_name"`
	MailingAddress string    `db:"mailing_address"`
	Note           *string   `db:"note"`
	CreatedAt      time.Time `db:"created_at"`
}

// CreateQSLCardRequest stores a new physical QSL card request.
func CreateQSLCardRequest(ctx context.Context, input CreateQSLCardRequestInput) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	qsoID := strings.TrimSpace(input.QSOID)
	if qsoID == "" {
		return fmt.Errorf("qso id is required")
	}

	mailingAddress := strings.TrimSpace(input.MailingAddress)
	if mailingAddress == "" {
		return fmt.Errorf("mailing address is required")
	}

	var requesterName *string
	if name := strings.TrimSpace(input.RequesterName); name != "" {
		requesterName = &name
	}

	var note *string
	if value := strings.TrimSpace(input.Note); value != "" {
		note = &value
	}

	query := `
		INSERT INTO qsl_card_requests (qso_id, requester_name, mailing_address, note)
		VALUES ($1, $2, $3, $4)
	`

	_, err := pool.Exec(ctx, query, qsoID, requesterName, mailingAddress, note)
	if err != nil {
		return fmt.Errorf("failed to create qsl card request: %w", err)
	}

	return nil
}

// ListOpenQSLCardRequests returns pending (not dismissed) physical QSL card requests.
func ListOpenQSLCardRequests(ctx context.Context) ([]QSLCardRequestListItem, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := `
		SELECT
			r.id,
			r.qso_id,
			q.call,
			q.qso_date,
			q.time_on,
			q.mode,
			q.band,
			q.country,
			r.requester_name,
			r.mailing_address,
			r.note,
			r.created_at
		FROM qsl_card_requests r
		JOIN qsos q ON q.id = r.qso_id
		WHERE r.dismissed_at IS NULL
		ORDER BY r.created_at DESC
	`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query qsl card requests: %w", err)
	}
	defer rows.Close()

	var requests []QSLCardRequestListItem
	for rows.Next() {
		var request QSLCardRequestListItem
		if err := rows.Scan(
			&request.ID,
			&request.QSOID,
			&request.Call,
			&request.QSODate,
			&request.TimeOn,
			&request.Mode,
			&request.Band,
			&request.Country,
			&request.RequesterName,
			&request.MailingAddress,
			&request.Note,
			&request.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan qsl card request: %w", err)
		}
		requests = append(requests, request)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating qsl card requests: %w", err)
	}

	return requests, nil
}

// HasOpenQSLCardRequestForQSO returns true if the QSO has at least one open request.
func HasOpenQSLCardRequestForQSO(ctx context.Context, qsoID string) (bool, error) {
	if pool == nil {
		return false, fmt.Errorf("database connection not initialized")
	}

	id := strings.TrimSpace(qsoID)
	if id == "" {
		return false, fmt.Errorf("qso id is required")
	}

	query := `
		SELECT EXISTS (
			SELECT 1
			FROM qsl_card_requests
			WHERE qso_id = $1
			  AND dismissed_at IS NULL
		)
	`

	var hasOpenRequest bool
	if err := pool.QueryRow(ctx, query, id).Scan(&hasOpenRequest); err != nil {
		return false, fmt.Errorf("failed to check open qsl card request: %w", err)
	}

	return hasOpenRequest, nil
}

// DismissQSLCardRequest hides a request from the pending queue.
func DismissQSLCardRequest(ctx context.Context, requestID string) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	id := strings.TrimSpace(requestID)
	if id == "" {
		return fmt.Errorf("request id is required")
	}

	query := `
		UPDATE qsl_card_requests
		SET dismissed_at = NOW()
		WHERE id = $1 AND dismissed_at IS NULL
	`

	result, err := pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to dismiss qsl card request: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("qsl card request not found")
	}

	return nil
}
