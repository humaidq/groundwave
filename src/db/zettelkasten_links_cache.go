/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package db

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/humaidq/groundwave/utils"
)

// Global cache for backlinks
var (
	backlinkCache    = make(map[string][]string) // target ID -> slice of source IDs
	forwardLinkCache = make(map[string][]string) // source ID -> slice of target IDs
	backlinkMutex    sync.RWMutex
	lastCacheBuild   time.Time
)

// ExtractLinksFromContent extracts all org-roam ID links from content
// Returns a slice of target IDs found in the content
// Handles both [[id:uuid][title]] and [[id:uuid]] formats
func ExtractLinksFromContent(content string) []string {
	// Match [[id:uuid][title]] or [[id:uuid]]
	// Captures the UUID in group 1
	re := regexp.MustCompile(`\[\[id:([a-f0-9\-]+)\](?:\[([^\]]+)\])?\]`)
	matches := re.FindAllStringSubmatch(content, -1)

	targetIDs := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			targetID := match[1] // The UUID
			targetIDs = append(targetIDs, targetID)
		}
	}

	return targetIDs
}

// BuildBacklinkCache scans all .org files and builds the backlink index
// This function is designed to be called periodically by a background worker
func BuildBacklinkCache(ctx context.Context) error {
	log.Println("Building backlink cache...")
	startTime := time.Now()

	// List all .org files
	files, err := ListOrgFiles(ctx)
	if err != nil {
		return fmt.Errorf("failed to list org files: %w", err)
	}

	// Build temporary cache
	tempBacklinkCache := make(map[string][]string)
	tempForwardCache := make(map[string][]string)
	filesProcessed := 0
	filesSkipped := 0

	// Scan each file
	for _, file := range files {
		// Fetch file content
		content, err := FetchOrgFile(ctx, file)
		if err != nil {
			log.Printf("Skipping unreadable file %s: %v", file, err)
			filesSkipped++
			continue
		}

		// Extract source ID (the note's own ID)
		sourceID, err := utils.ExtractIDProperty(content)
		if err != nil {
			// File doesn't have an ID property, skip it
			filesSkipped++
			continue
		}

		// Extract all link targets from this note
		targetIDs := ExtractLinksFromContent(content)

		seenTargets := make(map[string]struct{}, len(targetIDs))
		for _, targetID := range targetIDs {
			if targetID == "" {
				continue
			}
			if _, exists := seenTargets[targetID]; exists {
				continue
			}
			seenTargets[targetID] = struct{}{}
			tempBacklinkCache[targetID] = append(tempBacklinkCache[targetID], sourceID)
		}

		forwardLinks := make([]string, 0, len(seenTargets))
		for targetID := range seenTargets {
			forwardLinks = append(forwardLinks, targetID)
		}
		sort.Strings(forwardLinks)
		tempForwardCache[sourceID] = forwardLinks

		filesProcessed++
	}

	// Update global cache with write lock
	backlinkMutex.Lock()
	backlinkCache = tempBacklinkCache
	forwardLinkCache = tempForwardCache
	lastCacheBuild = time.Now()
	backlinkMutex.Unlock()

	duration := time.Since(startTime)
	log.Printf("Backlink cache built: %d files processed, %d skipped, %d backlink entries, took %v",
		filesProcessed, filesSkipped, len(tempBacklinkCache), duration)

	return nil
}

// GetBacklinksFromCache retrieves backlinks for a given target ID
// Returns a slice of source IDs (notes that link to the target)
// Returns empty slice if no backlinks found
func GetBacklinksFromCache(targetID string) []string {
	backlinkMutex.RLock()
	defer backlinkMutex.RUnlock()

	backlinks, exists := backlinkCache[targetID]
	if !exists {
		return []string{}
	}

	// Return a copy to prevent external modification
	result := make([]string, len(backlinks))
	copy(result, backlinks)
	return result
}

// GetForwardLinksFromCache retrieves forward links for a given source ID.
func GetForwardLinksFromCache(sourceID string) []string {
	backlinkMutex.RLock()
	defer backlinkMutex.RUnlock()

	links, exists := forwardLinkCache[sourceID]
	if !exists {
		return []string{}
	}

	result := make([]string, len(links))
	copy(result, links)
	return result
}

// GetLastCacheBuildTime returns the timestamp of the last cache build
func GetLastCacheBuildTime() time.Time {
	backlinkMutex.RLock()
	defer backlinkMutex.RUnlock()
	return lastCacheBuild
}

// StartBacklinkRefreshWorker starts a background goroutine that periodically
// refreshes the backlink cache
func StartBacklinkRefreshWorker(ctx context.Context) {
	go func() {
		// Initial delay to let the application start up
		log.Println("Backlink refresh worker starting in 30 seconds...")
		time.Sleep(30 * time.Second)

		// Initial cache build
		if err := BuildBacklinkCache(ctx); err != nil {
			log.Printf("Error building initial backlink cache: %v", err)
		}

		// Periodic refresh every 10 minutes
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Println("Backlink refresh worker shutting down")
				return
			case <-ticker.C:
				if err := BuildBacklinkCache(ctx); err != nil {
					log.Printf("Error refreshing backlink cache: %v", err)
				}
			}
		}
	}()
}
