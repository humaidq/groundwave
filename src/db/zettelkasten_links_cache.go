/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package db

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"regexp"
	"sort"
	"strings"
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

// JournalEntry represents a daily journal note.
type JournalEntry struct {
	Date       time.Time
	Filename   string
	Title      string
	HTMLBody   template.HTML
	Preview    template.HTML
	HasMore    bool
	UpdatedAt  time.Time
	DateString string
}

// ZKTimelineNote represents a zettelkasten note in the timeline.
type ZKTimelineNote struct {
	ID         string
	Title      string
	Filename   string
	Timestamp  time.Time
	DateString string
}

var (
	journalCache      = make(map[string]JournalEntry) // date string -> entry
	journalMutex      sync.RWMutex
	lastJournalBuild  time.Time
	journalFileFormat = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}\.org$`)

	zkNoteCache       = make(map[string][]ZKTimelineNote) // date string -> notes
	zkNoteMutex       sync.RWMutex
	lastZKNoteBuild   time.Time
	zkNoteFileFormat  = regexp.MustCompile(`^\d{14}-.*\.org$`)
	zkTimestampFormat = regexp.MustCompile(`^(\d{14})-`)
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

// BuildJournalCache scans daily journal entries and caches them for the timeline.
func BuildJournalCache(ctx context.Context) error {
	log.Println("Building journal cache...")
	startTime := time.Now()

	files, err := ListDailyOrgFiles(ctx)
	if err != nil {
		return fmt.Errorf("failed to list daily org files: %w", err)
	}

	tempCache := make(map[string]JournalEntry)
	filesProcessed := 0
	filesSkipped := 0

	for _, file := range files {
		if !journalFileFormat.MatchString(file) {
			continue
		}

		dateString := strings.TrimSuffix(file, ".org")
		parsedDate, err := time.Parse("2006-01-02", dateString)
		if err != nil {
			filesSkipped++
			continue
		}

		content, err := FetchDailyOrgFile(ctx, file)
		if err != nil {
			log.Printf("Skipping unreadable journal file %s: %v", file, err)
			filesSkipped++
			continue
		}

		htmlBody, err := utils.ParseOrgToHTML(content)
		if err != nil {
			log.Printf("Skipping journal file %s due to parse error: %v", file, err)
			filesSkipped++
			continue
		}

		previewContent, hasMore := buildJournalPreview(content, 2, 480)
		previewHTML := ""
		if previewContent != "" {
			previewHTML, err = utils.ParseOrgToHTML(previewContent)
			if err != nil {
				log.Printf("Failed to parse journal preview %s: %v", file, err)
				previewHTML = ""
			}
		}

		title := utils.ExtractTitle(content)
		if title == "Untitled Note" {
			title = dateString
		}

		tempCache[dateString] = JournalEntry{
			Date:       parsedDate,
			Filename:   file,
			Title:      title,
			HTMLBody:   template.HTML(htmlBody),
			Preview:    template.HTML(previewHTML),
			HasMore:    hasMore,
			UpdatedAt:  time.Now(),
			DateString: dateString,
		}
		filesProcessed++
	}

	journalMutex.Lock()
	journalCache = tempCache
	lastJournalBuild = time.Now()
	journalMutex.Unlock()

	duration := time.Since(startTime)
	log.Printf("Journal cache built: %d files processed, %d skipped, %d entries, took %v",
		filesProcessed, filesSkipped, len(tempCache), duration)

	return nil
}

// BuildZKTimelineNotesCache scans zettelkasten notes and caches them for timeline display.
func BuildZKTimelineNotesCache(ctx context.Context) error {
	log.Println("Building zettelkasten timeline note cache...")
	startTime := time.Now()

	files, err := ListOrgFiles(ctx)
	if err != nil {
		return fmt.Errorf("failed to list org files: %w", err)
	}

	config, err := GetZKConfig()
	if err != nil {
		return fmt.Errorf("failed to load zettelkasten config: %w", err)
	}

	tempCache := make(map[string][]ZKTimelineNote)
	filesProcessed := 0
	filesSkipped := 0

	for _, file := range files {
		if file == config.IndexFile {
			continue
		}
		if !zkNoteFileFormat.MatchString(file) {
			continue
		}

		matches := zkTimestampFormat.FindStringSubmatch(file)
		if len(matches) < 2 {
			filesSkipped++
			continue
		}

		parsedTimestamp, err := time.Parse("20060102150405", matches[1])
		if err != nil {
			filesSkipped++
			continue
		}

		content, err := FetchOrgFile(ctx, file)
		if err != nil {
			log.Printf("Skipping unreadable note file %s: %v", file, err)
			filesSkipped++
			continue
		}

		noteID, err := utils.ExtractIDProperty(content)
		if err != nil {
			filesSkipped++
			continue
		}

		title := utils.ExtractTitle(content)
		if title == "Untitled Note" {
			title = strings.TrimSuffix(file, ".org")
		}

		dateString := parsedTimestamp.Format("2006-01-02")
		overrideDate, hasOverride := utils.ExtractDateDirective(content)
		if hasOverride {
			overrideDateString := overrideDate.Format("2006-01-02")
			if overrideDateString != dateString {
				parsedTimestamp = time.Date(
					overrideDate.Year(),
					overrideDate.Month(),
					overrideDate.Day(),
					0, 0, 0, 0,
					parsedTimestamp.Location(),
				)
				dateString = overrideDateString
			}
		}

		tempCache[dateString] = append(tempCache[dateString], ZKTimelineNote{
			ID:         noteID,
			Title:      title,
			Filename:   file,
			Timestamp:  parsedTimestamp,
			DateString: dateString,
		})
		filesProcessed++
	}

	for date := range tempCache {
		sort.Slice(tempCache[date], func(i, j int) bool {
			return tempCache[date][i].Timestamp.After(tempCache[date][j].Timestamp)
		})
	}

	zkNoteMutex.Lock()
	zkNoteCache = tempCache
	lastZKNoteBuild = time.Now()
	zkNoteMutex.Unlock()

	duration := time.Since(startTime)
	log.Printf("Zettelkasten note cache built: %d files processed, %d skipped, %d dates, took %v",
		filesProcessed, filesSkipped, len(tempCache), duration)

	return nil
}

func buildJournalPreview(content string, maxParagraphs, maxChars int) (string, bool) {
	lines := strings.Split(content, "\n")
	paragraphs := make([]string, 0, maxParagraphs)
	current := make([]string, 0)
	inProperties := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.EqualFold(trimmed, ":PROPERTIES:") {
			inProperties = true
			continue
		}
		if inProperties {
			if strings.EqualFold(trimmed, ":END:") {
				inProperties = false
			}
			continue
		}
		if strings.HasPrefix(strings.ToUpper(trimmed), "#+TITLE:") {
			continue
		}

		if trimmed == "" {
			if len(current) > 0 {
				paragraphs = append(paragraphs, strings.Join(current, "\n"))
				current = current[:0]
			}
			continue
		}

		current = append(current, line)
	}

	if len(current) > 0 {
		paragraphs = append(paragraphs, strings.Join(current, "\n"))
	}

	hasMore := false
	if len(paragraphs) > maxParagraphs {
		paragraphs = paragraphs[:maxParagraphs]
		hasMore = true
	}

	preview := strings.TrimSpace(strings.Join(paragraphs, "\n\n"))
	if preview == "" {
		return "", false
	}

	if len(preview) > maxChars {
		preview = strings.TrimSpace(preview[:maxChars])
		hasMore = true
	}

	return preview, hasMore
}

// GetJournalEntriesFromCache retrieves all cached journal entries sorted by date.
func GetJournalEntriesFromCache() []JournalEntry {
	journalMutex.RLock()
	entries := make([]JournalEntry, 0, len(journalCache))
	for _, entry := range journalCache {
		entries = append(entries, entry)
	}
	journalMutex.RUnlock()

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Date.After(entries[j].Date)
	})

	return entries
}

// GetJournalEntryByDate fetches a single cached entry by date string.
func GetJournalEntryByDate(date string) (JournalEntry, bool) {
	if date == "" {
		return JournalEntry{}, false
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return JournalEntry{}, false
	}

	journalMutex.RLock()
	entry, exists := journalCache[date]
	journalMutex.RUnlock()

	return entry, exists
}

// GetLastJournalCacheBuildTime returns the last journal cache build time.
func GetLastJournalCacheBuildTime() time.Time {
	journalMutex.RLock()
	defer journalMutex.RUnlock()
	return lastJournalBuild
}

// GetZKTimelineNotesByDate returns cached zettelkasten notes grouped by date.
func GetZKTimelineNotesByDate() map[string][]ZKTimelineNote {
	zkNoteMutex.RLock()
	defer zkNoteMutex.RUnlock()

	result := make(map[string][]ZKTimelineNote, len(zkNoteCache))
	for date, notes := range zkNoteCache {
		copied := make([]ZKTimelineNote, len(notes))
		copy(copied, notes)
		result[date] = copied
	}

	return result
}

// GetLastZKNoteCacheBuildTime returns the last zettelkasten note cache build time.
func GetLastZKNoteCacheBuildTime() time.Time {
	zkNoteMutex.RLock()
	defer zkNoteMutex.RUnlock()
	return lastZKNoteBuild
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
		if err := BuildJournalCache(ctx); err != nil {
			log.Printf("Error building initial journal cache: %v", err)
		}
		if err := BuildZKTimelineNotesCache(ctx); err != nil {
			log.Printf("Error building initial zettelkasten note cache: %v", err)
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
				if err := BuildJournalCache(ctx); err != nil {
					log.Printf("Error refreshing journal cache: %v", err)
				}
				if err := BuildZKTimelineNotesCache(ctx); err != nil {
					log.Printf("Error refreshing zettelkasten note cache: %v", err)
				}
			}
		}
	}()
}
