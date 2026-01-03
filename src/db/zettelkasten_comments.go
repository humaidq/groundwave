/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package db

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/humaidq/groundwave/utils"
)

// GetCommentsForZettel fetches all comments for a specific zettel
func GetCommentsForZettel(ctx context.Context, zettelID string) ([]ZettelComment, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	// Validate zettel ID format
	if err := utils.ValidateUUID(zettelID); err != nil {
		return nil, fmt.Errorf("invalid zettel ID: %w", err)
	}

	query := `
		SELECT id, zettel_id, content, created_at, updated_at
		FROM zettel_comments
		WHERE zettel_id = $1
		ORDER BY created_at ASC
	`

	rows, err := pool.Query(ctx, query, zettelID)
	if err != nil {
		return nil, fmt.Errorf("failed to query zettel comments: %w", err)
	}
	defer rows.Close()

	var comments []ZettelComment
	for rows.Next() {
		var comment ZettelComment
		err := rows.Scan(
			&comment.ID,
			&comment.ZettelID,
			&comment.Content,
			&comment.CreatedAt,
			&comment.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan comment: %w", err)
		}
		comments = append(comments, comment)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating comments: %w", err)
	}

	return comments, nil
}

// CreateZettelComment creates a new comment for a zettel
func CreateZettelComment(ctx context.Context, zettelID string, content string) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	// Validate zettel ID format
	if err := utils.ValidateUUID(zettelID); err != nil {
		return fmt.Errorf("invalid zettel ID: %w", err)
	}

	// Validate content is not empty
	if content == "" {
		return fmt.Errorf("comment content cannot be empty")
	}

	query := `
		INSERT INTO zettel_comments (zettel_id, content)
		VALUES ($1, $2)
	`

	_, err := pool.Exec(ctx, query, zettelID, content)
	if err != nil {
		return fmt.Errorf("failed to create zettel comment: %w", err)
	}

	return nil
}

// DeleteZettelComment deletes a comment by ID
func DeleteZettelComment(ctx context.Context, commentID uuid.UUID) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	query := `DELETE FROM zettel_comments WHERE id = $1`

	result, err := pool.Exec(ctx, query, commentID)
	if err != nil {
		return fmt.Errorf("failed to delete zettel comment: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("comment not found: %s", commentID)
	}

	return nil
}

// GetAllZettelComments fetches all comments grouped by zettel for the inbox view
// Returns comments with zettel metadata (title, filename, orphaned status)
func GetAllZettelComments(ctx context.Context) ([]ZettelCommentWithNote, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	// Get all comments ordered by zettel_id and creation time
	query := `
		SELECT id, zettel_id, content, created_at, updated_at
		FROM zettel_comments
		ORDER BY zettel_id, created_at ASC
	`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query all zettel comments: %w", err)
	}
	defer rows.Close()

	var comments []ZettelComment
	for rows.Next() {
		var comment ZettelComment
		err := rows.Scan(
			&comment.ID,
			&comment.ZettelID,
			&comment.Content,
			&comment.CreatedAt,
			&comment.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan comment: %w", err)
		}
		comments = append(comments, comment)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating comments: %w", err)
	}

	// Enrich comments with zettel metadata
	var enrichedComments []ZettelCommentWithNote
	zettelCache := make(map[string]*ZKNote) // Cache to avoid fetching same zettel multiple times

	for _, comment := range comments {
		enriched := ZettelCommentWithNote{
			ZettelComment: comment,
		}

		// Check cache first
		if note, exists := zettelCache[comment.ZettelID]; exists {
			if note == nil {
				// Cached as orphaned
				enriched.OrphanedNote = true
				enriched.ZettelTitle = "[Note Not Found]"
				enriched.ZettelFilename = ""
			} else {
				enriched.ZettelTitle = note.Title
				enriched.ZettelFilename = note.Filename
				enriched.OrphanedNote = false
			}
		} else {
			// Fetch zettel metadata
			note, err := GetNoteByID(ctx, comment.ZettelID)
			if err != nil {
				// Zettel not found (orphaned)
				log.Printf("Warning: zettel %s not found (orphaned comment): %v", comment.ZettelID, err)
				enriched.OrphanedNote = true
				enriched.ZettelTitle = "[Note Not Found]"
				enriched.ZettelFilename = ""
				zettelCache[comment.ZettelID] = nil // Cache as orphaned
			} else {
				enriched.ZettelTitle = note.Title
				enriched.ZettelFilename = note.Filename
				enriched.OrphanedNote = false
				zettelCache[comment.ZettelID] = note
			}
		}

		enrichedComments = append(enrichedComments, enriched)
	}

	return enrichedComments, nil
}

// GetZettelCommentCount returns the total number of comments across all zettels
// Useful for displaying a badge in navigation
func GetZettelCommentCount(ctx context.Context) (int, error) {
	if pool == nil {
		return 0, fmt.Errorf("database connection not initialized")
	}

	var count int
	query := `SELECT COUNT(*) FROM zettel_comments`

	err := pool.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count zettel comments: %w", err)
	}

	return count, nil
}
