/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package db

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/humaidq/groundwave/utils"
)

const (
	qrzLogbookAPIEndpoint     = "https://logbook.qrz.com/api"
	qrzLogbookUserAgentEnvVar = "QRZ_USERAGENT"
	qrzLogbookFetchPageSize   = 250
	qrzLogbookFetchMaxPages   = 200
	qrzLogbookModSinceOffset  = -7
)

var qrzLogbookIDFieldRegex = regexp.MustCompile(`(?i)<APP_QRZLOG_LOGID:\d+>(\d+)`)

// QRZLogbookSyncResult summarizes a QRZ logbook sync run.
type QRZLogbookSyncResult struct {
	RequestedLogbooks int
	SyncedLogbooks    int
	FailedLogbooks    int
	ProcessedQSOs     int
}

type qrzLogbookFetchResponse struct {
	Result string
	Reason string
	Count  int
	ADIF   string
}

// SyncQRZLogbooks fetches and imports the latest QSOs from one or more QRZ logbooks.
func SyncQRZLogbooks(ctx context.Context, apiKeys []string) (QRZLogbookSyncResult, error) {
	result := QRZLogbookSyncResult{}

	keys := normalizeQRZAPIKeys(apiKeys)
	if len(keys) == 0 {
		return result, ErrNoQRZAPIKeysProvided
	}

	userAgent, err := qrzLogbookUserAgentHeader()
	if err != nil {
		return result, err
	}

	result.RequestedLogbooks = len(keys)

	latestQSOTime, err := GetLatestQSOTime(ctx)
	if err != nil {
		return result, fmt.Errorf("failed to determine latest QSO time: %w", err)
	}

	modSince := ""
	if latestQSOTime != nil {
		modSince = latestQSOTime.AddDate(0, 0, qrzLogbookModSinceOffset).UTC().Format("2006-01-02")
	}

	client := &http.Client{Timeout: 45 * time.Second}

	for idx, apiKey := range keys {
		processed, syncErr := syncSingleQRZLogbook(ctx, client, apiKey, modSince, userAgent)
		if syncErr != nil {
			result.FailedLogbooks++

			logger.Error("QRZ logbook sync failed", "logbook_index", idx+1, "error", syncErr)

			continue
		}

		result.SyncedLogbooks++
		result.ProcessedQSOs += processed
	}

	if result.SyncedLogbooks == 0 {
		return result, ErrSyncAllQRZLogbooksFailed
	}

	return result, nil
}

func normalizeQRZAPIKeys(apiKeys []string) []string {
	normalized := make([]string, 0, len(apiKeys))
	seen := make(map[string]struct{}, len(apiKeys))

	for _, apiKey := range apiKeys {
		value := strings.TrimSpace(apiKey)
		if value == "" {
			continue
		}

		if _, exists := seen[value]; exists {
			continue
		}

		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}

	return normalized
}

func syncSingleQRZLogbook(ctx context.Context, client *http.Client, apiKey string, modSince string, userAgent string) (int, error) {
	afterLogID := 0
	processedTotal := 0

	for range qrzLogbookFetchMaxPages {
		option := buildQRZFetchOption(modSince, afterLogID)

		response, err := fetchQRZLogbookPage(ctx, client, apiKey, option, userAgent)
		if err != nil {
			return processedTotal, err
		}

		if strings.TrimSpace(response.ADIF) == "" {
			return processedTotal, nil
		}

		parser := utils.NewADIFParser()
		if err := parser.ParseFile(bytes.NewReader([]byte(response.ADIF))); err != nil {
			return processedTotal, fmt.Errorf("failed to parse QRZ ADIF response: %w", err)
		}

		if len(parser.QSOs) == 0 {
			return processedTotal, nil
		}

		processed, err := ImportADIFQSOs(ctx, parser.QSOs)
		if err != nil {
			return processedTotal, fmt.Errorf("failed to import QRZ QSOs: %w", err)
		}

		processedTotal += processed

		if len(parser.QSOs) < qrzLogbookFetchPageSize {
			return processedTotal, nil
		}

		maxLogID := maxQRZLogbookID(response.ADIF)
		if maxLogID <= afterLogID {
			return processedTotal, ErrQRZPaginationMissingLogID
		}

		afterLogID = maxLogID + 1
	}

	return processedTotal, fmt.Errorf("%w (%d pages)", ErrQRZPaginationLimitReached, qrzLogbookFetchMaxPages)
}

func buildQRZFetchOption(modSince string, afterLogID int) string {
	options := []string{
		fmt.Sprintf("MAX:%d", qrzLogbookFetchPageSize),
		"TYPE:ADIF",
		fmt.Sprintf("AFTERLOGID:%d", afterLogID),
	}
	if modSince != "" {
		options = append(options, "MODSINCE:"+modSince)
	}

	return strings.Join(options, ",")
}

func fetchQRZLogbookPage(ctx context.Context, client *http.Client, apiKey string, option string, userAgent string) (*qrzLogbookFetchResponse, error) {
	form := url.Values{}
	form.Set("KEY", apiKey)
	form.Set("ACTION", "FETCH")
	form.Set("OPTION", option)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, qrzLogbookAPIEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create QRZ logbook request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call QRZ logbook API: %w", err)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warn("Failed to close QRZ logbook response body", "error", err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read QRZ logbook response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %d", ErrQRZLogbookAPIReturnedStatus, resp.StatusCode)
	}

	parsed, err := parseQRZLogbookResponse(body)
	if err != nil {
		return nil, err
	}

	if parsed.Result != "OK" {
		reason := parsed.Reason
		if reason == "" {
			reason = "unknown error"
		}

		return nil, fmt.Errorf("%w %s: %s", ErrQRZLogbookAPIError, strings.ToLower(parsed.Result), reason)
	}

	return parsed, nil
}

func qrzLogbookUserAgentHeader() (string, error) {
	userAgent := strings.TrimSpace(os.Getenv(qrzLogbookUserAgentEnvVar))
	if userAgent == "" {
		return "", ErrQRZUserAgentNotConfigured
	}

	return userAgent, nil
}

func parseQRZLogbookResponse(raw []byte) (*qrzLogbookFetchResponse, error) {
	fields := parseQRZResponseFields(string(raw))

	result := strings.ToUpper(strings.TrimSpace(fields["RESULT"]))
	if result == "" {
		result = strings.ToUpper(strings.TrimSpace(fields["STATUS"]))
	}

	if result == "" {
		return nil, ErrQRZLogbookResponseMissing
	}

	count := 0

	if countValue := strings.TrimSpace(fields["COUNT"]); countValue != "" {
		parsedCount, err := strconv.Atoi(countValue)
		if err == nil {
			count = parsedCount
		}
	}

	return &qrzLogbookFetchResponse{
		Result: result,
		Reason: strings.TrimSpace(fields["REASON"]),
		Count:  count,
		ADIF:   fields["ADIF"],
	}, nil
}

func parseQRZResponseFields(raw string) map[string]string {
	fields := make(map[string]string)

	for _, part := range strings.Split(strings.TrimSpace(raw), "&") {
		if part == "" {
			continue
		}

		pair := strings.SplitN(part, "=", 2)

		key := strings.ToUpper(strings.TrimSpace(pair[0]))
		if key == "" {
			continue
		}

		value := ""

		if len(pair) == 2 {
			decodedValue, err := url.QueryUnescape(pair[1])
			if err != nil {
				value = pair[1]
			} else {
				value = decodedValue
			}
		}

		fields[key] = value
	}

	return fields
}

func maxQRZLogbookID(adif string) int {
	maxLogID := 0

	matches := qrzLogbookIDFieldRegex.FindAllStringSubmatch(adif, -1)
	for _, match := range matches {
		if len(match) != 2 {
			continue
		}

		logID, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}

		if logID > maxLogID {
			maxLogID = logID
		}
	}

	return maxLogID
}
