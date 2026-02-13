/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package db

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	qrzXMLLookupDefaultLimit = 40
	qrzXMLRequestTimeout     = 30 * time.Second
	qrzXMLDefaultAgent       = "Groundwave/1.0 (+https://huma.id)"
	qrzXMLBackgroundTimeout  = 15 * time.Minute
)

var qrzXMLServiceURL = "https://xmldata.qrz.com/xml/current/"

var qrzCallsignSyncState = struct {
	mu           sync.Mutex
	running      bool
	lastStarted  time.Time
	lastFinished time.Time
	lastResult   QRZCallsignSyncResult
	lastError    string
}{}

// QRZCallsignSyncResult summarizes profile refresh work done for callsigns.
type QRZCallsignSyncResult struct {
	Seeded   int
	Pending  int
	LookedUp int
	Updated  int
	Failed   int
}

// QRZCallsignBackgroundSyncStatus tracks asynchronous sync state.
type QRZCallsignBackgroundSyncStatus struct {
	Running      bool
	LastStarted  time.Time
	LastFinished time.Time
	LastResult   QRZCallsignSyncResult
	LastError    string
}

// QSLCallsignQSOItem is a compact QSO item for grouped callsign views.
type QSLCallsignQSOItem struct {
	ID      string    `db:"id"`
	QSODate time.Time `db:"qso_date"`
	TimeOn  time.Time `db:"time_on"`
	Band    *string   `db:"band"`
	Mode    string    `db:"mode"`
	Country *string   `db:"country"`
}

// FormatDate formats the QSO date as YYYY-MM-DD.
func (q QSLCallsignQSOItem) FormatDate() string {
	return q.QSODate.Format("2006-01-02")
}

// FormatTime formats the QSO time as HH:MM UTC.
func (q QSLCallsignQSOItem) FormatTime() string {
	return q.TimeOn.UTC().Format("15:04")
}

// QSLCallsignProfileListItem represents grouped callsign profile data for /qsl/callsigns.
type QSLCallsignProfileListItem struct {
	Callsign    string  `db:"callsign"`
	Call        *string `db:"call"`
	FName       *string `db:"fname"`
	Name        *string `db:"name"`
	NameFmt     *string `db:"name_fmt"`
	Attn        *string `db:"attn"`
	Addr1       *string `db:"addr1"`
	Addr2       *string `db:"addr2"`
	State       *string `db:"state"`
	Zip         *string `db:"zip"`
	Country     *string `db:"country"`
	ContactID   *string `db:"contact_id"`
	ContactName *string `db:"contact_name"`
	QSOs        []QSLCallsignQSOItem
}

// DisplayName returns the best available name from QRZ profile fields.
func (item QSLCallsignProfileListItem) DisplayName() string {
	if value := strings.TrimSpace(pointerString(item.NameFmt)); value != "" {
		return value
	}

	combined := strings.TrimSpace(strings.TrimSpace(pointerString(item.FName)) + " " + strings.TrimSpace(pointerString(item.Name)))
	if combined != "" {
		return combined
	}

	return ""
}

// AddressLines returns preformatted address lines suitable for template rendering.
func (item QSLCallsignProfileListItem) AddressLines() []string {
	return buildQRZAddressLines(item.Attn, item.Addr1, item.Addr2, item.State, item.Zip, item.Country)
}

// HasAddress returns true when at least one mailing-address line is available.
func (item QSLCallsignProfileListItem) HasAddress() bool {
	return len(item.AddressLines()) > 0
}

// HasContact returns true when a linked contact exists.
func (item QSLCallsignProfileListItem) HasContact() bool {
	return strings.TrimSpace(pointerString(item.ContactID)) != ""
}

// ContactIDValue returns contact_id for URL generation.
func (item QSLCallsignProfileListItem) ContactIDValue() string {
	return strings.TrimSpace(pointerString(item.ContactID))
}

// ContactLabel returns a readable label for a linked contact.
func (item QSLCallsignProfileListItem) ContactLabel() string {
	if name := strings.TrimSpace(pointerString(item.ContactName)); name != "" {
		return name
	}

	return item.Callsign
}

// QSLCallsignProfileDetail represents detailed callsign profile data for /qrz/{callsign}.
type QSLCallsignProfileDetail struct {
	Callsign    string   `db:"callsign"`
	Call        *string  `db:"call"`
	FName       *string  `db:"fname"`
	Name        *string  `db:"name"`
	NameFmt     *string  `db:"name_fmt"`
	Nickname    *string  `db:"nickname"`
	Attn        *string  `db:"attn"`
	Addr1       *string  `db:"addr1"`
	Addr2       *string  `db:"addr2"`
	State       *string  `db:"state"`
	Zip         *string  `db:"zip"`
	Country     *string  `db:"country"`
	Grid        *string  `db:"grid"`
	County      *string  `db:"county"`
	DXCC        *int     `db:"dxcc"`
	QSLMgr      *string  `db:"qslmgr"`
	Email       *string  `db:"email"`
	QRZUser     *string  `db:"qrz_user"`
	Aliases     *string  `db:"aliases"`
	Xref        *string  `db:"xref"`
	Lat         *float64 `db:"lat"`
	Lon         *float64 `db:"lon"`
	ContactID   *string  `db:"contact_id"`
	ContactName *string  `db:"contact_name"`
}

// DisplayName returns the best available name from QRZ profile fields.
func (item QSLCallsignProfileDetail) DisplayName() string {
	if value := strings.TrimSpace(pointerString(item.NameFmt)); value != "" {
		return value
	}

	combined := strings.TrimSpace(strings.TrimSpace(pointerString(item.FName)) + " " + strings.TrimSpace(pointerString(item.Name)))
	if combined != "" {
		return combined
	}

	return ""
}

// AddressLines returns preformatted address lines suitable for template rendering.
func (item QSLCallsignProfileDetail) AddressLines() []string {
	return buildQRZAddressLines(item.Attn, item.Addr1, item.Addr2, item.State, item.Zip, item.Country)
}

// HasAddress returns true when at least one mailing-address line is available.
func (item QSLCallsignProfileDetail) HasAddress() bool {
	return len(item.AddressLines()) > 0
}

// HasContact returns true when a linked contact exists.
func (item QSLCallsignProfileDetail) HasContact() bool {
	return strings.TrimSpace(pointerString(item.ContactID)) != ""
}

// ContactIDValue returns contact_id for URL generation.
func (item QSLCallsignProfileDetail) ContactIDValue() string {
	return strings.TrimSpace(pointerString(item.ContactID))
}

// ContactLabel returns a readable label for a linked contact.
func (item QSLCallsignProfileDetail) ContactLabel() string {
	if name := strings.TrimSpace(pointerString(item.ContactName)); name != "" {
		return name
	}

	return item.Callsign
}

// DXCCString returns DXCC as a string for display.
func (item QSLCallsignProfileDetail) DXCCString() string {
	if item.DXCC == nil {
		return ""
	}

	return strconv.Itoa(*item.DXCC)
}

// CoordinateString returns formatted latitude/longitude if available.
func (item QSLCallsignProfileDetail) CoordinateString() string {
	if item.Lat == nil || item.Lon == nil {
		return ""
	}

	return fmt.Sprintf("%.5f, %.5f", *item.Lat, *item.Lon)
}

// HasProfileData returns true when at least one profile field is available.
func (item QSLCallsignProfileDetail) HasProfileData() bool {
	if item.DisplayName() != "" || item.HasAddress() {
		return true
	}

	for _, value := range []*string{item.Nickname, item.QSLMgr, item.Email, item.Grid, item.County, item.Aliases, item.Xref, item.QRZUser} {
		if strings.TrimSpace(pointerString(value)) != "" {
			return true
		}
	}

	if item.DXCC != nil {
		return true
	}

	if item.Lat != nil || item.Lon != nil {
		return true
	}

	return false
}

type qrzXMLField struct {
	XMLName xml.Name
	Value   string `xml:",chardata"`
}

type qrzXMLNode struct {
	Fields []qrzXMLField `xml:",any"`
}

type qrzXMLResponseEnvelope struct {
	XMLName  xml.Name   `xml:"QRZDatabase"`
	Version  string     `xml:"version,attr"`
	Session  qrzXMLNode `xml:"Session"`
	Callsign qrzXMLNode `xml:"Callsign"`
}

type qrzXMLResponse struct {
	Version  string
	Session  map[string]string
	Callsign map[string]string
	RawXML   string
}

type qrzXMLCredentials struct {
	Username string
	Password string
	Agent    string
}

// SyncMissingQRZCallsignProfiles seeds profile rows for QSO/contact callsigns and refreshes missing records.
func SyncMissingQRZCallsignProfiles(ctx context.Context, limit int) (QRZCallsignSyncResult, error) {
	if pool == nil {
		return QRZCallsignSyncResult{}, ErrDatabaseConnectionNotInitialized
	}

	seeded, err := seedQRZCallsignProfilesFromQSOs(ctx)
	if err != nil {
		return QRZCallsignSyncResult{}, fmt.Errorf("failed to seed qrz callsign profiles: %w", err)
	}

	result := QRZCallsignSyncResult{Seeded: seeded}

	if limit <= 0 {
		limit = qrzXMLLookupDefaultLimit
	}

	pending, err := listCallsignsPendingQRZLookup(ctx, limit)
	if err != nil {
		return result, fmt.Errorf("failed to list callsigns pending qrz lookup: %w", err)
	}

	result.Pending = len(pending)
	if len(pending) == 0 {
		return result, nil
	}

	credentials, err := loadQRZXMLCredentials()
	if err != nil {
		return result, err
	}

	client := &http.Client{Timeout: qrzXMLRequestTimeout}

	sessionKey, err := loginQRZXML(ctx, client, credentials)
	if err != nil {
		return result, err
	}

	for _, callsign := range pending {
		if err := ctx.Err(); err != nil {
			return result, fmt.Errorf("qrz callsign sync canceled: %w", err)
		}

		result.LookedUp++

		response, err := lookupQRZCallsign(ctx, client, credentials.Agent, sessionKey, callsign)
		if err != nil {
			if isContextDoneError(err) {
				return result, err
			}

			result.Failed++

			if storeErr := upsertQRZCallsignLookupFailure(ctx, callsign, nil, "", err); storeErr != nil {
				if isContextDoneError(storeErr) {
					return result, storeErr
				}

				logger.Warn("Failed storing QRZ lookup failure", "callsign", callsign, "error", storeErr)
			}

			continue
		}

		sessionError := qrzXMLFieldValue(response.Session, "Error")
		if sessionError != "" {
			result.Failed++

			payload := buildQRZPayload(response, callsign)

			lookupErr := fmt.Errorf("%w: %s", ErrQRZXMLLookupFailed, sessionError)
			if storeErr := upsertQRZCallsignLookupFailure(ctx, callsign, payload, response.RawXML, lookupErr); storeErr != nil {
				if isContextDoneError(storeErr) {
					return result, storeErr
				}

				logger.Warn("Failed storing QRZ session error", "callsign", callsign, "error", storeErr)
			}

			continue
		}

		if err := upsertQRZCallsignLookupSuccess(ctx, callsign, response); err != nil {
			if isContextDoneError(err) {
				return result, err
			}

			result.Failed++

			logger.Error("Failed storing QRZ lookup success", "callsign", callsign, "error", err)

			continue
		}

		result.Updated++
	}

	return result, nil
}

// SyncQRZCallsignProfile refreshes a single callsign profile from QRZ XML.
func SyncQRZCallsignProfile(ctx context.Context, callsign string) error {
	if pool == nil {
		return ErrDatabaseConnectionNotInitialized
	}

	normalizedCallsign := strings.ToUpper(strings.TrimSpace(callsign))
	if normalizedCallsign == "" {
		return ErrCallsignRequired
	}

	credentials, err := loadQRZXMLCredentials()
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: qrzXMLRequestTimeout}

	sessionKey, err := loginQRZXML(ctx, client, credentials)
	if err != nil {
		return err
	}

	response, err := lookupQRZCallsign(ctx, client, credentials.Agent, sessionKey, normalizedCallsign)
	if err != nil {
		if storeErr := upsertQRZCallsignLookupFailure(ctx, normalizedCallsign, nil, "", err); storeErr != nil {
			if isContextDoneError(storeErr) {
				return storeErr
			}

			logger.Warn("Failed storing QRZ lookup failure", "callsign", normalizedCallsign, "error", storeErr)
		}

		return err
	}

	sessionError := qrzXMLFieldValue(response.Session, "Error")
	if sessionError != "" {
		payload := buildQRZPayload(response, normalizedCallsign)

		lookupErr := fmt.Errorf("%w: %s", ErrQRZXMLLookupFailed, sessionError)
		if storeErr := upsertQRZCallsignLookupFailure(ctx, normalizedCallsign, payload, response.RawXML, lookupErr); storeErr != nil {
			if isContextDoneError(storeErr) {
				return storeErr
			}

			logger.Warn("Failed storing QRZ session error", "callsign", normalizedCallsign, "error", storeErr)
		}

		return lookupErr
	}

	if err := upsertQRZCallsignLookupSuccess(ctx, normalizedCallsign, response); err != nil {
		return err
	}

	return nil
}

// StartQRZCallsignProfileSyncInBackground launches QRZ callsign profile sync asynchronously.
// It returns true when a new background run starts and false when a run is already in progress.
func StartQRZCallsignProfileSyncInBackground(limit int) bool {
	if limit <= 0 {
		limit = qrzXMLLookupDefaultLimit
	}

	now := time.Now().UTC()

	qrzCallsignSyncState.mu.Lock()

	if qrzCallsignSyncState.running {
		qrzCallsignSyncState.mu.Unlock()

		return false
	}

	qrzCallsignSyncState.running = true
	qrzCallsignSyncState.lastStarted = now
	qrzCallsignSyncState.lastError = ""
	qrzCallsignSyncState.mu.Unlock()

	go func(syncLimit int) {
		ctx, cancel := context.WithTimeout(context.Background(), qrzXMLBackgroundTimeout)
		defer cancel()

		result, err := SyncMissingQRZCallsignProfiles(ctx, syncLimit)

		qrzCallsignSyncState.mu.Lock()
		qrzCallsignSyncState.running = false
		qrzCallsignSyncState.lastFinished = time.Now().UTC()
		qrzCallsignSyncState.lastResult = result

		if err != nil {
			qrzCallsignSyncState.lastError = err.Error()
		} else {
			qrzCallsignSyncState.lastError = ""
		}

		qrzCallsignSyncState.mu.Unlock()

		if err != nil {
			if isContextDoneError(err) {
				logger.Warn("QRZ callsign background sync canceled", "error", err)

				return
			}

			logger.Error("QRZ callsign background sync failed", "error", err)

			return
		}

		logger.Info(
			"QRZ callsign background sync completed",
			"seeded", result.Seeded,
			"pending", result.Pending,
			"looked_up", result.LookedUp,
			"updated", result.Updated,
			"failed", result.Failed,
		)
	}(limit)

	return true
}

// GetQRZCallsignBackgroundSyncStatus returns current async sync status.
func GetQRZCallsignBackgroundSyncStatus() QRZCallsignBackgroundSyncStatus {
	qrzCallsignSyncState.mu.Lock()
	defer qrzCallsignSyncState.mu.Unlock()

	return QRZCallsignBackgroundSyncStatus{
		Running:      qrzCallsignSyncState.running,
		LastStarted:  qrzCallsignSyncState.lastStarted,
		LastFinished: qrzCallsignSyncState.lastFinished,
		LastResult:   qrzCallsignSyncState.lastResult,
		LastError:    qrzCallsignSyncState.lastError,
	}
}

// ListQSLCallsignProfiles returns all worked callsigns ordered alphabetically with grouped QSO lists.
func ListQSLCallsignProfiles(ctx context.Context) ([]QSLCallsignProfileListItem, error) {
	if pool == nil {
		return nil, ErrDatabaseConnectionNotInitialized
	}

	const profileQuery = `
		WITH worked_calls AS (
			SELECT DISTINCT upper(btrim(call)) AS callsign
			FROM qsos
			WHERE btrim(call) <> ''
			UNION
			SELECT DISTINCT upper(btrim(call_sign)) AS callsign
			FROM contacts
			WHERE call_sign IS NOT NULL AND btrim(call_sign) <> ''
		)
		SELECT
			worked_calls.callsign,
			p.call,
			p.fname,
			p.name,
			p.name_fmt,
			p.attn,
			p.addr1,
			p.addr2,
			p.state,
			p.zip,
			p.country,
			COALESCE(call_sign_contact.id::text, qso_contact.id) AS contact_id,
			COALESCE(call_sign_contact.name_display, qso_contact.name_display) AS contact_name
		FROM worked_calls
		LEFT JOIN qrz_callsign_profiles p ON p.callsign = worked_calls.callsign
		LEFT JOIN contacts call_sign_contact ON call_sign_contact.call_sign = worked_calls.callsign
		LEFT JOIN LATERAL (
			SELECT c.id::text AS id, c.name_display
			FROM qsos q
			JOIN contacts c ON c.id = q.contact_id
			WHERE upper(q.call) = worked_calls.callsign
			ORDER BY q.qso_date DESC, q.time_on DESC
			LIMIT 1
		) qso_contact ON true
		ORDER BY worked_calls.callsign ASC
	`

	rows, err := pool.Query(ctx, profileQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to query qsl callsign profiles: %w", err)
	}
	defer rows.Close()

	profiles := []QSLCallsignProfileListItem{}

	for rows.Next() {
		var item QSLCallsignProfileListItem

		if err := rows.Scan(
			&item.Callsign,
			&item.Call,
			&item.FName,
			&item.Name,
			&item.NameFmt,
			&item.Attn,
			&item.Addr1,
			&item.Addr2,
			&item.State,
			&item.Zip,
			&item.Country,
			&item.ContactID,
			&item.ContactName,
		); err != nil {
			return nil, fmt.Errorf("failed to scan qsl callsign profile row: %w", err)
		}

		item.QSOs = []QSLCallsignQSOItem{}
		profiles = append(profiles, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating qsl callsign profile rows: %w", err)
	}

	qsoByCallsign, err := listQSOItemsByCallsign(ctx)
	if err != nil {
		return nil, err
	}

	for idx := range profiles {
		profiles[idx].QSOs = qsoByCallsign[profiles[idx].Callsign]
		if profiles[idx].QSOs == nil {
			profiles[idx].QSOs = []QSLCallsignQSOItem{}
		}
	}

	return profiles, nil
}

func listQSOItemsByCallsign(ctx context.Context) (map[string][]QSLCallsignQSOItem, error) {
	const qsoQuery = `
		SELECT
			upper(call) AS callsign,
			id,
			qso_date,
			time_on,
			band,
			mode,
			country
		FROM qsos
		WHERE btrim(call) <> ''
		ORDER BY upper(call) ASC, qso_date DESC, time_on DESC
	`

	rows, err := pool.Query(ctx, qsoQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to query qso items by callsign: %w", err)
	}
	defer rows.Close()

	grouped := make(map[string][]QSLCallsignQSOItem)

	for rows.Next() {
		var (
			callsign string
			item     QSLCallsignQSOItem
		)

		if err := rows.Scan(
			&callsign,
			&item.ID,
			&item.QSODate,
			&item.TimeOn,
			&item.Band,
			&item.Mode,
			&item.Country,
		); err != nil {
			return nil, fmt.Errorf("failed to scan grouped qso item: %w", err)
		}

		grouped[callsign] = append(grouped[callsign], item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating grouped qso rows: %w", err)
	}

	return grouped, nil
}

// GetQSLCallsignProfileDetail returns detailed QRZ profile data for a callsign.
func GetQSLCallsignProfileDetail(ctx context.Context, callsign string) (*QSLCallsignProfileDetail, error) {
	if pool == nil {
		return nil, ErrDatabaseConnectionNotInitialized
	}

	normalizedCallsign := strings.ToUpper(strings.TrimSpace(callsign))
	if normalizedCallsign == "" {
		return nil, ErrCallsignRequired
	}

	const query = `
		WITH requested_call AS (
			SELECT $1::text AS callsign
		)
		SELECT
			requested_call.callsign,
			p.call,
			p.fname,
			p.name,
			p.name_fmt,
			p.nickname,
			p.attn,
			p.addr1,
			p.addr2,
			p.state,
			p.zip,
			p.country,
			p.grid,
			p.county,
			p.dxcc,
			p.qslmgr,
			p.email,
			p.qrz_user,
			p.aliases,
			p.xref,
			p.lat,
			p.lon,
			COALESCE(call_sign_contact.id::text, qso_contact.id) AS contact_id,
			COALESCE(call_sign_contact.name_display, qso_contact.name_display) AS contact_name
		FROM requested_call
		LEFT JOIN qrz_callsign_profiles p ON p.callsign = requested_call.callsign
		LEFT JOIN contacts call_sign_contact ON call_sign_contact.call_sign = requested_call.callsign
		LEFT JOIN LATERAL (
			SELECT c.id::text AS id, c.name_display
			FROM qsos q
			JOIN contacts c ON c.id = q.contact_id
			WHERE upper(q.call) = requested_call.callsign
			ORDER BY q.qso_date DESC, q.time_on DESC
			LIMIT 1
		) qso_contact ON true
	`

	var detail QSLCallsignProfileDetail

	err := pool.QueryRow(ctx, query, normalizedCallsign).Scan(
		&detail.Callsign,
		&detail.Call,
		&detail.FName,
		&detail.Name,
		&detail.NameFmt,
		&detail.Nickname,
		&detail.Attn,
		&detail.Addr1,
		&detail.Addr2,
		&detail.State,
		&detail.Zip,
		&detail.Country,
		&detail.Grid,
		&detail.County,
		&detail.DXCC,
		&detail.QSLMgr,
		&detail.Email,
		&detail.QRZUser,
		&detail.Aliases,
		&detail.Xref,
		&detail.Lat,
		&detail.Lon,
		&detail.ContactID,
		&detail.ContactName,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query qrz callsign profile detail: %w", err)
	}

	return &detail, nil
}

func seedQRZCallsignProfilesFromQSOs(ctx context.Context) (int, error) {
	const query = `
		INSERT INTO qrz_callsign_profiles (callsign)
		SELECT DISTINCT callsign
		FROM (
			SELECT upper(btrim(call)) AS callsign
			FROM qsos
			WHERE btrim(call) <> ''
			UNION
			SELECT upper(btrim(call_sign)) AS callsign
			FROM contacts
			WHERE call_sign IS NOT NULL AND btrim(call_sign) <> ''
		) AS combined_calls
		ON CONFLICT (callsign) DO NOTHING
	`

	result, err := pool.Exec(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to seed qrz callsign profiles: %w", err)
	}

	return int(result.RowsAffected()), nil
}

func buildQRZAddressLines(attn, addr1, addr2, state, zip, country *string) []string {
	lines := []string{}

	for _, part := range []*string{attn, addr1} {
		if value := strings.TrimSpace(pointerString(part)); value != "" {
			lines = append(lines, value)
		}
	}

	city := strings.TrimSpace(pointerString(addr2))
	region := strings.TrimSpace(pointerString(state))
	postalCode := strings.TrimSpace(pointerString(zip))

	locationParts := []string{}
	if city != "" {
		locationParts = append(locationParts, city)
	}

	if region != "" {
		locationParts = append(locationParts, region)
	}

	locationLine := strings.Join(locationParts, ", ")
	if postalCode != "" {
		if locationLine == "" {
			locationLine = postalCode
		} else {
			locationLine = locationLine + " " + postalCode
		}
	}

	if locationLine != "" {
		lines = append(lines, locationLine)
	}

	if country := strings.TrimSpace(pointerString(country)); country != "" {
		lines = append(lines, country)
	}

	return lines
}

func listCallsignsPendingQRZLookup(ctx context.Context, limit int) ([]string, error) {
	const query = `
		SELECT callsign
		FROM qrz_callsign_profiles
		WHERE payload_json IS NULL
			OR (
				last_lookup_error IS NOT NULL
				AND (
					last_lookup_at IS NULL
					OR last_lookup_at < NOW() - INTERVAL '24 hours'
				)
			)
		ORDER BY callsign ASC
		LIMIT $1
	`

	rows, err := pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list pending callsigns: %w", err)
	}
	defer rows.Close()

	pending := []string{}

	for rows.Next() {
		var callsign string

		if err := rows.Scan(&callsign); err != nil {
			return nil, fmt.Errorf("failed to scan pending callsign: %w", err)
		}

		pending = append(pending, callsign)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating pending callsigns: %w", err)
	}

	return pending, nil
}

func loadQRZXMLCredentials() (qrzXMLCredentials, error) {
	username := firstNonEmptyEnv("QRZ_XML_USERNAME", "QRZ_USERNAME")
	password := firstNonEmptyEnv("QRZ_XML_PASSWORD", "QRZ_PASSWORD")

	if username == "" || password == "" {
		return qrzXMLCredentials{}, ErrQRZXMLCredentialsNotConfigured
	}

	agent := strings.TrimSpace(os.Getenv("QRZ_XML_AGENT"))
	if agent == "" {
		agent = qrzXMLDefaultAgent
	}

	return qrzXMLCredentials{
		Username: username,
		Password: password,
		Agent:    agent,
	}, nil
}

func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}

	return ""
}

func loginQRZXML(ctx context.Context, client *http.Client, credentials qrzXMLCredentials) (string, error) {
	params := url.Values{}
	params.Set("username", credentials.Username)
	params.Set("password", credentials.Password)
	params.Set("agent", credentials.Agent)

	response, err := requestQRZXML(ctx, client, credentials.Agent, params)
	if err != nil {
		return "", err
	}

	if sessionError := qrzXMLFieldValue(response.Session, "Error"); sessionError != "" {
		return "", fmt.Errorf("%w: %s", ErrQRZXMLLoginFailed, sessionError)
	}

	sessionKey := qrzXMLFieldValue(response.Session, "Key")
	if sessionKey == "" {
		return "", ErrQRZXMLSessionKeyMissing
	}

	return sessionKey, nil
}

func lookupQRZCallsign(ctx context.Context, client *http.Client, agent string, sessionKey string, callsign string) (*qrzXMLResponse, error) {
	params := url.Values{}
	params.Set("s", sessionKey)
	params.Set("callsign", callsign)

	response, err := requestQRZXML(ctx, client, agent, params)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func requestQRZXML(ctx context.Context, client *http.Client, agent string, params url.Values) (*qrzXMLResponse, error) {
	endpoint, err := url.Parse(qrzXMLServiceURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrQRZXMLServiceURLInvalid, err)
	}

	query := endpoint.Query()

	for key, values := range params {
		for _, value := range values {
			query.Set(key, value)
		}
	}

	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create qrz xml request: %w", err)
	}

	req.Header.Set("User-Agent", agent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call qrz xml api: %w", err)
	}

	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Warn("Failed to close QRZ XML response body", "error", closeErr)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read qrz xml response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %d", ErrQRZXMLAPIReturnedStatus, resp.StatusCode)
	}

	parsed, err := parseQRZXMLResponse(body)
	if err != nil {
		return nil, err
	}

	return parsed, nil
}

func parseQRZXMLResponse(raw []byte) (*qrzXMLResponse, error) {
	var envelope qrzXMLResponseEnvelope

	if err := xml.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrQRZXMLResponseMalformed, err)
	}

	return &qrzXMLResponse{
		Version:  strings.TrimSpace(envelope.Version),
		Session:  qrzXMLFieldsToMap(envelope.Session.Fields),
		Callsign: qrzXMLFieldsToMap(envelope.Callsign.Fields),
		RawXML:   string(raw),
	}, nil
}

func qrzXMLFieldsToMap(fields []qrzXMLField) map[string]string {
	result := make(map[string]string, len(fields))

	for _, field := range fields {
		key := strings.TrimSpace(field.XMLName.Local)
		if key == "" {
			continue
		}

		result[key] = strings.TrimSpace(field.Value)
	}

	return result
}

func qrzXMLFieldValue(values map[string]string, key string) string {
	for candidate, value := range values {
		if strings.EqualFold(candidate, key) {
			return strings.TrimSpace(value)
		}
	}

	return ""
}

func buildQRZPayload(response *qrzXMLResponse, queriedCallsign string) map[string]any {
	payload := map[string]any{
		"query_callsign": strings.ToUpper(strings.TrimSpace(queriedCallsign)),
		"xml_version":    response.Version,
		"session":        response.Session,
		"callsign":       response.Callsign,
	}

	return payload
}

func upsertQRZCallsignLookupSuccess(ctx context.Context, callsign string, response *qrzXMLResponse) error {
	normalizedCallsign := strings.ToUpper(strings.TrimSpace(callsign))
	if normalizedCallsign == "" {
		return ErrCallsignRequired
	}

	payload := buildQRZPayload(response, normalizedCallsign)
	lookupTime := time.Now().UTC()

	query := `
		INSERT INTO qrz_callsign_profiles (
			callsign,
			call,
			xref,
			aliases,
			dxcc,
			fname,
			name,
			name_fmt,
			nickname,
			attn,
			addr1,
			addr2,
			state,
			zip,
			country,
			ccode,
			lat,
			lon,
			grid,
			county,
			qslmgr,
			email,
			qrz_user,
			payload_json,
			raw_xml,
			last_lookup_at,
			last_success_at,
			last_lookup_error
		)
		VALUES (
			$1,
			$2,
			$3,
			$4,
			$5,
			$6,
			$7,
			$8,
			$9,
			$10,
			$11,
			$12,
			$13,
			$14,
			$15,
			$16,
			$17,
			$18,
			$19,
			$20,
			$21,
			$22,
			$23,
			$24::jsonb,
			$25,
			$26,
			$27,
			$28
		)
		ON CONFLICT (callsign) DO UPDATE SET
			call = EXCLUDED.call,
			xref = EXCLUDED.xref,
			aliases = EXCLUDED.aliases,
			dxcc = EXCLUDED.dxcc,
			fname = EXCLUDED.fname,
			name = EXCLUDED.name,
			name_fmt = EXCLUDED.name_fmt,
			nickname = EXCLUDED.nickname,
			attn = EXCLUDED.attn,
			addr1 = EXCLUDED.addr1,
			addr2 = EXCLUDED.addr2,
			state = EXCLUDED.state,
			zip = EXCLUDED.zip,
			country = EXCLUDED.country,
			ccode = EXCLUDED.ccode,
			lat = EXCLUDED.lat,
			lon = EXCLUDED.lon,
			grid = EXCLUDED.grid,
			county = EXCLUDED.county,
			qslmgr = EXCLUDED.qslmgr,
			email = EXCLUDED.email,
			qrz_user = EXCLUDED.qrz_user,
			payload_json = EXCLUDED.payload_json,
			raw_xml = EXCLUDED.raw_xml,
			last_lookup_at = EXCLUDED.last_lookup_at,
			last_success_at = EXCLUDED.last_success_at,
			last_lookup_error = EXCLUDED.last_lookup_error
	`

	_, err := pool.Exec(ctx, query,
		normalizedCallsign,
		nullableValue(trimOptional(qrzXMLFieldValue(response.Callsign, "call"))),
		nullableValue(trimOptional(qrzXMLFieldValue(response.Callsign, "xref"))),
		nullableValue(trimOptional(qrzXMLFieldValue(response.Callsign, "aliases"))),
		nullableValue(parseQRZOptionalInt(qrzXMLFieldValue(response.Callsign, "dxcc"))),
		nullableValue(trimOptional(qrzXMLFieldValue(response.Callsign, "fname"))),
		nullableValue(trimOptional(qrzXMLFieldValue(response.Callsign, "name"))),
		nullableValue(trimOptional(qrzXMLFieldValue(response.Callsign, "name_fmt"))),
		nullableValue(trimOptional(qrzXMLFieldValue(response.Callsign, "nickname"))),
		nullableValue(trimOptional(qrzXMLFieldValue(response.Callsign, "attn"))),
		nullableValue(trimOptional(qrzXMLFieldValue(response.Callsign, "addr1"))),
		nullableValue(trimOptional(qrzXMLFieldValue(response.Callsign, "addr2"))),
		nullableValue(trimOptional(qrzXMLFieldValue(response.Callsign, "state"))),
		nullableValue(trimOptional(qrzXMLFieldValue(response.Callsign, "zip"))),
		nullableValue(trimOptional(qrzXMLFieldValue(response.Callsign, "country"))),
		nullableValue(parseQRZOptionalInt(qrzXMLFieldValue(response.Callsign, "ccode"))),
		nullableValue(parseQRZOptionalFloat(qrzXMLFieldValue(response.Callsign, "lat"))),
		nullableValue(parseQRZOptionalFloat(qrzXMLFieldValue(response.Callsign, "lon"))),
		nullableValue(trimOptional(qrzXMLFieldValue(response.Callsign, "grid"))),
		nullableValue(trimOptional(qrzXMLFieldValue(response.Callsign, "county"))),
		nullableValue(trimOptional(qrzXMLFieldValue(response.Callsign, "qslmgr"))),
		nullableValue(trimOptional(qrzXMLFieldValue(response.Callsign, "email"))),
		nullableValue(trimOptional(qrzXMLFieldValue(response.Callsign, "user"))),
		payload,
		response.RawXML,
		lookupTime,
		lookupTime,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert qrz callsign profile success: %w", err)
	}

	return nil
}

func upsertQRZCallsignLookupFailure(ctx context.Context, callsign string, payload map[string]any, rawXML string, lookupErr error) error {
	normalizedCallsign := strings.ToUpper(strings.TrimSpace(callsign))
	if normalizedCallsign == "" {
		return ErrCallsignRequired
	}

	lookupTime := time.Now().UTC()

	errorMessage := "QRZ lookup failed"
	if lookupErr != nil {
		errorMessage = strings.TrimSpace(lookupErr.Error())
	}

	query := `
		INSERT INTO qrz_callsign_profiles (
			callsign,
			payload_json,
			raw_xml,
			last_lookup_at,
			last_lookup_error
		)
		VALUES ($1, $2::jsonb, NULLIF($3, ''), $4, $5)
		ON CONFLICT (callsign) DO UPDATE SET
			payload_json = COALESCE(EXCLUDED.payload_json, qrz_callsign_profiles.payload_json),
			raw_xml = COALESCE(EXCLUDED.raw_xml, qrz_callsign_profiles.raw_xml),
			last_lookup_at = EXCLUDED.last_lookup_at,
			last_lookup_error = EXCLUDED.last_lookup_error
	`

	_, err := pool.Exec(ctx, query,
		normalizedCallsign,
		payload,
		rawXML,
		lookupTime,
		errorMessage,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert qrz callsign profile failure: %w", err)
	}

	return nil
}

func parseQRZOptionalInt(value string) *int {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}

	parsed, err := strconv.Atoi(trimmed)
	if err != nil {
		return nil
	}

	return &parsed
}

func parseQRZOptionalFloat(value string) *float64 {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}

	parsed, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return nil
	}

	return &parsed
}

func pointerString(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}

func isContextDoneError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
