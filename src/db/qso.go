/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package db

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/humaidq/groundwave/utils"
)

// QSOListItem represents a QSO in the list view
type QSOListItem struct {
	ID         string    `db:"id"`
	Call       string    `db:"call"`
	QSODate    time.Time `db:"qso_date"`
	TimeOn     time.Time `db:"time_on"`
	Band       *string   `db:"band"`
	Mode       string    `db:"mode"`
	RSTSent    *string   `db:"rst_sent"`
	RSTRcvd    *string   `db:"rst_rcvd"`
	Country    *string   `db:"country"`
	Name       *string   `db:"name"`
	QTH        *string   `db:"qth"`
	State      *string   `db:"state"`
	GridSquare *string   `db:"gridsquare"`
}

// FormatDate formats QSO date as YYYY-MM-DD
func (q *QSOListItem) FormatDate() string {
	return q.QSODate.Format("2006-01-02")
}

// FormatTime formats QSO time as HH:MM (UTC)
func (q *QSOListItem) FormatTime() string {
	return q.TimeOn.UTC().Format("15:04")
}

// FormatQSOTime formats QSO timestamp for display
func (q *QSOListItem) FormatQSOTime() string {
	return q.TimeOn.UTC().Format("2006-01-02 15:04:05 UTC")
}

// TimestampUTC returns the QSO date+time in UTC.
func (q *QSOListItem) TimestampUTC() time.Time {
	timeOn := q.TimeOn.UTC()

	return time.Date(
		q.QSODate.Year(),
		q.QSODate.Month(),
		q.QSODate.Day(),
		timeOn.Hour(),
		timeOn.Minute(),
		timeOn.Second(),
		0,
		time.UTC,
	)
}

// UnixTimestamp returns the QSO timestamp as Unix seconds.
func (q *QSOListItem) UnixTimestamp() int64 {
	return q.TimestampUTC().Unix()
}

// GetFlagCode returns the ISO 3166-1 alpha-2 country code for flagcdn.com.
func (q *QSOListItem) GetFlagCode() string {
	if q.Country == nil {
		return ""
	}

	return utils.CountryFlagCode(*q.Country)
}

// QSOHallOfFameItem represents a paper QSL hall of fame entry.
type QSOHallOfFameItem struct {
	Call    string  `db:"call"`
	Name    *string `db:"name"`
	Country *string `db:"country"`
}

// GetFlagCode returns the ISO 3166-1 alpha-2 country code for flagcdn.com.
func (q *QSOHallOfFameItem) GetFlagCode() string {
	if q.Country == nil {
		return ""
	}

	return utils.CountryFlagCode(*q.Country)
}

// QSODetail represents a full QSO with all details
type QSODetail struct {
	*QSO                    // Embed pointer instead of value for proper method access
	ContactName     *string `db:"contact_name"`
	ContactCallSign *string `db:"contact_call_sign"`
}

// ListQSOs returns all QSOs sorted by date/time (most recent first)
func ListQSOs(ctx context.Context) ([]QSOListItem, error) {
	if pool == nil {
		return nil, ErrDatabaseConnectionNotInitialized
	}

	query := `
		SELECT
			id,
			call,
			qso_date,
			time_on,
			band,
			mode,
			rst_sent,
			rst_rcvd,
			country,
			name,
			qth,
			state,
			gridsquare
		FROM qsos
		ORDER BY qso_date DESC, time_on DESC
	`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query QSOs: %w", err)
	}
	defer rows.Close()

	var qsos []QSOListItem

	for rows.Next() {
		var qso QSOListItem

		err := rows.Scan(
			&qso.ID,
			&qso.Call,
			&qso.QSODate,
			&qso.TimeOn,
			&qso.Band,
			&qso.Mode,
			&qso.RSTSent,
			&qso.RSTRcvd,
			&qso.Country,
			&qso.Name,
			&qso.QTH,
			&qso.State,
			&qso.GridSquare,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan QSO: %w", err)
		}

		qsos = append(qsos, qso)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating QSOs: %w", err)
	}

	return qsos, nil
}

// ListRecentQSOs returns the most recent N QSOs sorted by date/time
func ListRecentQSOs(ctx context.Context, limit int) ([]QSOListItem, error) {
	if pool == nil {
		return nil, ErrDatabaseConnectionNotInitialized
	}

	query := `
		SELECT
			id,
			call,
			qso_date,
			time_on,
			band,
			mode,
			rst_sent,
			rst_rcvd,
			country,
			name,
			qth,
			state,
			gridsquare
		FROM qsos
		ORDER BY qso_date DESC, time_on DESC
		LIMIT $1
	`

	rows, err := pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent QSOs: %w", err)
	}
	defer rows.Close()

	var qsos []QSOListItem

	for rows.Next() {
		var qso QSOListItem

		err := rows.Scan(
			&qso.ID,
			&qso.Call,
			&qso.QSODate,
			&qso.TimeOn,
			&qso.Band,
			&qso.Mode,
			&qso.RSTSent,
			&qso.RSTRcvd,
			&qso.Country,
			&qso.Name,
			&qso.QTH,
			&qso.State,
			&qso.GridSquare,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan QSO: %w", err)
		}

		qsos = append(qsos, qso)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating recent QSOs: %w", err)
	}

	return qsos, nil
}

// ImportADIFQSOs imports QSOs from parsed ADIF data with merge logic
// Merge logic: file values override DB values, but DB values are kept if file field is empty
func ImportADIFQSOs(ctx context.Context, qsos []utils.QSO) (int, error) {
	if pool == nil {
		return 0, ErrDatabaseConnectionNotInitialized
	}

	imported := 0
	updated := 0

	for _, qso := range qsos {
		timestamp, err := parseADIFTimestamp(qso.QSODate, qso.TimeOn)
		if err != nil {
			continue
		}

		mode := trimOptional(qso.Mode)
		if mode == nil {
			continue
		}

		timeOff := parseOptionalADIFTimestamp(qso.QSODateOff, qso.TimeOff)
		if timeOff == nil {
			timeOff = parseOptionalADIFTimestamp(qso.QSODate, qso.TimeOff)
		}

		qsoDateOff := parseOptionalADIFDate(qso.QSODateOff)
		if qsoDateOff == nil && timeOff != nil {
			qsoDateOff = parseOptionalADIFDate(qso.QSODate)
		}

		band := trimOptional(qso.Band)
		freq := parseOptionalADIFFloat(qso.Freq)
		bandRx := trimOptional(qso.BandRx)
		freqRx := parseOptionalADIFFloat(qso.FreqRx)
		submode := trimOptional(qso.Submode)
		rstSent := trimOptional(qso.RSTSent)
		rstRcvd := trimOptional(qso.RSTRcvd)
		qth := trimOptional(qso.QTH)
		name := trimOptional(qso.Name)
		comment := trimOptional(qso.Comment)
		notes := trimOptional(qso.Notes)
		gridSquare := trimOptional(qso.GridSquare)
		country := trimOptional(qso.Country)
		dxcc := parseOptionalADIFInt(qso.DXCC)
		cqz := parseOptionalADIFInt(qso.CQZ)
		ituz := parseOptionalADIFInt(qso.ITUZ)
		cont := trimOptional(qso.Cont)
		state := trimOptional(qso.State)
		cnty := normalizeCNTY(qso.Cnty, country)
		pfx := trimOptional(qso.Pfx)
		iota := trimOptional(qso.IOTA)
		distance := parseOptionalADIFFloat(qso.Distance)
		aIndex := parseOptionalADIFInt(qso.AIndex)
		kIndex := parseOptionalADIFInt(qso.KIndex)
		sfi := parseOptionalADIFInt(qso.SFI)
		myName := trimOptional(qso.MyName)
		myCity := trimOptional(qso.MyCity)
		myCountry := trimOptional(qso.MyCountry)
		myCQZone := parseOptionalADIFInt(qso.MyCQZone)
		myITUZone := parseOptionalADIFInt(qso.MyITUZone)
		myDXCC := parseOptionalADIFInt(qso.MyDXCC)
		myGridSquare := trimOptional(qso.MyGridSquare)
		stationCall := trimOptional(qso.StationCall)
		operator := trimOptional(qso.Operator)
		myRig := trimOptional(qso.MyRig)
		myAntenna := trimOptional(qso.MyAntenna)
		txPwr := parseOptionalADIFFloat(qso.TxPwr)

		qslSent := normalizeQSLSentStatus(string(qso.QslSent))
		qslRcvd := normalizeQSLStatus(string(qso.QslRcvd))
		qslsDate := parseOptionalADIFDate(qso.QSLSDate)
		qslrDate := parseOptionalADIFDate(qso.QSLRDate)
		qslSentVia := normalizeQSLVia(qso.QSLSentVia)
		qslRcvdVia := normalizeQSLVia(qso.QSLRcvdVia)
		qslVia := trimOptional(qso.QSLVia)
		qslMsg := trimOptional(qso.QSLMsg)
		qslMsgRcvd := trimOptional(qso.QSLMsgRcvd)

		lotwSent := normalizeQSLSentStatus(string(qso.LotwSent))
		lotwRcvd := normalizeQSLStatus(string(qso.LotwRcvd))
		lotwQSLSDate := parseOptionalADIFDate(qso.LotwQSLSDate)
		lotwQSLRDate := parseOptionalADIFDate(qso.LotwQSLRDate)

		eqslSent := normalizeQSLSentStatus(string(qso.EqslSent))
		eqslRcvd := normalizeQSLStatus(string(qso.EqslRcvd))
		eqslQSLSDate := parseOptionalADIFDate(qso.EqslQSLSDate)
		eqslQSLRDate := parseOptionalADIFDate(qso.EqslQSLRDate)
		eqslAG := parseOptionalADIFBool(qso.EqslAG)

		clublogUploadDate := parseOptionalADIFDate(qso.ClublogQSOUploadDate)
		clublogUploadStatus := normalizeQSOUploadStatus(qso.ClublogQSOUploadStatus)
		hrdlogUploadDate := parseOptionalADIFDate(qso.HRDLogQSOUploadDate)
		hrdlogUploadStatus := normalizeQSOUploadStatus(qso.HRDLogQSOUploadStatus)

		appFields := normalizeJSONFields(qso.AppFields)
		userFields := normalizeJSONFields(qso.UserFields)

		// Check if QSO already exists (by call, date, and time)
		exists, err := qsoExists(ctx, qso.Call, timestamp)
		if err != nil {
			return imported, fmt.Errorf("failed to check if QSO exists: %w", err)
		}

		if exists {
			query := `
				UPDATE qsos SET
					band = COALESCE($1, band),
					freq = COALESCE($2, freq),
					mode = COALESCE($3, mode),
					time_off = COALESCE($4, time_off),
					qso_date_off = COALESCE($5, qso_date_off),
					band_rx = COALESCE($6, band_rx),
					freq_rx = COALESCE($7, freq_rx),
					submode = COALESCE($8, submode),
					rst_sent = COALESCE($9, rst_sent),
					rst_rcvd = COALESCE($10, rst_rcvd),
					qth = COALESCE($11, qth),
					name = COALESCE($12, name),
					comment = COALESCE($13, comment),
					notes = COALESCE($14, notes),
					gridsquare = COALESCE($15, gridsquare),
					country = COALESCE($16, country),
					dxcc = COALESCE($17, dxcc),
					cqz = COALESCE($18, cqz),
					ituz = COALESCE($19, ituz),
					cont = COALESCE($20, cont),
					state = COALESCE($21, state),
					cnty = COALESCE($22, cnty),
					pfx = COALESCE($23, pfx),
					iota = COALESCE($24, iota),
					distance = COALESCE($25, distance),
					a_index = COALESCE($26, a_index),
					k_index = COALESCE($27, k_index),
					sfi = COALESCE($28, sfi),
					my_name = COALESCE($29, my_name),
					my_city = COALESCE($30, my_city),
					my_country = COALESCE($31, my_country),
					my_cq_zone = COALESCE($32, my_cq_zone),
					my_itu_zone = COALESCE($33, my_itu_zone),
					my_dxcc = COALESCE($34, my_dxcc),
					my_gridsquare = COALESCE($35, my_gridsquare),
					station_callsign = COALESCE($36, station_callsign),
					operator = COALESCE($37, operator),
					my_rig = COALESCE($38, my_rig),
					my_antenna = COALESCE($39, my_antenna),
					tx_pwr = COALESCE($40, tx_pwr),
					qsl_sent = COALESCE($41::qsl_sent_status, qsl_sent),
					qsl_rcvd = COALESCE($42::qsl_status, qsl_rcvd),
					qslsdate = COALESCE($43, qslsdate),
					qslrdate = COALESCE($44, qslrdate),
					qsl_sent_via = COALESCE($45::qsl_via, qsl_sent_via),
					qsl_rcvd_via = COALESCE($46::qsl_via, qsl_rcvd_via),
					qsl_via = COALESCE($47, qsl_via),
					qslmsg = COALESCE($48, qslmsg),
					qslmsg_rcvd = COALESCE($49, qslmsg_rcvd),
					lotw_qsl_sent = COALESCE($50::qsl_sent_status, lotw_qsl_sent),
					lotw_qsl_rcvd = COALESCE($51::qsl_status, lotw_qsl_rcvd),
					lotw_qslsdate = COALESCE($52, lotw_qslsdate),
					lotw_qslrdate = COALESCE($53, lotw_qslrdate),
					eqsl_qsl_sent = COALESCE($54::qsl_sent_status, eqsl_qsl_sent),
					eqsl_qsl_rcvd = COALESCE($55::qsl_status, eqsl_qsl_rcvd),
					eqsl_qslsdate = COALESCE($56, eqsl_qslsdate),
					eqsl_qslrdate = COALESCE($57, eqsl_qslrdate),
					eqsl_ag = COALESCE($58, eqsl_ag),
					clublog_qso_upload_date = COALESCE($59, clublog_qso_upload_date),
					clublog_qso_upload_status = COALESCE($60::qso_upload_status, clublog_qso_upload_status),
					hrdlog_qso_upload_date = COALESCE($61, hrdlog_qso_upload_date),
					hrdlog_qso_upload_status = COALESCE($62::qso_upload_status, hrdlog_qso_upload_status),
					app_fields = COALESCE(app_fields, '{}'::jsonb) || COALESCE($63::jsonb, '{}'::jsonb),
					user_fields = COALESCE(user_fields, '{}'::jsonb) || COALESCE($64::jsonb, '{}'::jsonb),
					updated_at = NOW()
				WHERE call = $65 AND qso_date = $66 AND time_on = $67
			`

			_, err = pool.Exec(ctx, query,
				nullableValue(band),
				nullableValue(freq),
				nullableValue(mode),
				timeOff,
				qsoDateOff,
				nullableValue(bandRx),
				nullableValue(freqRx),
				nullableValue(submode),
				nullableValue(rstSent),
				nullableValue(rstRcvd),
				nullableValue(qth),
				nullableValue(name),
				nullableValue(comment),
				nullableValue(notes),
				nullableValue(gridSquare),
				nullableValue(country),
				nullableValue(dxcc),
				nullableValue(cqz),
				nullableValue(ituz),
				nullableValue(cont),
				nullableValue(state),
				nullableValue(cnty),
				nullableValue(pfx),
				nullableValue(iota),
				nullableValue(distance),
				nullableValue(aIndex),
				nullableValue(kIndex),
				nullableValue(sfi),
				nullableValue(myName),
				nullableValue(myCity),
				nullableValue(myCountry),
				nullableValue(myCQZone),
				nullableValue(myITUZone),
				nullableValue(myDXCC),
				nullableValue(myGridSquare),
				nullableValue(stationCall),
				nullableValue(operator),
				nullableValue(myRig),
				nullableValue(myAntenna),
				nullableValue(txPwr),
				nullableValue(qslSent),
				nullableValue(qslRcvd),
				qslsDate,
				qslrDate,
				nullableValue(qslSentVia),
				nullableValue(qslRcvdVia),
				nullableValue(qslVia),
				nullableValue(qslMsg),
				nullableValue(qslMsgRcvd),
				nullableValue(lotwSent),
				nullableValue(lotwRcvd),
				lotwQSLSDate,
				lotwQSLRDate,
				nullableValue(eqslSent),
				nullableValue(eqslRcvd),
				eqslQSLSDate,
				eqslQSLRDate,
				nullableValue(eqslAG),
				clublogUploadDate,
				nullableValue(clublogUploadStatus),
				hrdlogUploadDate,
				nullableValue(hrdlogUploadStatus),
				appFields,
				userFields,
				qso.Call,
				timestamp,
				timestamp,
			)
			if err != nil {
				return imported, fmt.Errorf("failed to update QSO: %w", err)
			}

			updated++
		} else {
			query := `
				INSERT INTO qsos (
					call, qso_date, time_on, time_off, qso_date_off,
					band, freq, mode, band_rx, freq_rx, submode,
					rst_sent, rst_rcvd, qth, name, comment, notes,
					gridsquare, country, dxcc, cqz, ituz, cont, state, cnty, pfx, iota,
					distance, a_index, k_index, sfi,
					my_name, my_city, my_country, my_cq_zone, my_itu_zone, my_dxcc,
					my_gridsquare, station_callsign, operator, my_rig, my_antenna, tx_pwr,
					qsl_sent, qsl_rcvd, qslsdate, qslrdate, qsl_sent_via, qsl_rcvd_via, qsl_via, qslmsg, qslmsg_rcvd,
					lotw_qsl_sent, lotw_qsl_rcvd, lotw_qslsdate, lotw_qslrdate,
					eqsl_qsl_sent, eqsl_qsl_rcvd, eqsl_qslsdate, eqsl_qslrdate, eqsl_ag,
					clublog_qso_upload_date, clublog_qso_upload_status, hrdlog_qso_upload_date, hrdlog_qso_upload_status,
					app_fields, user_fields
				) VALUES (
					$1, $2, $3, $4, $5,
					$6, $7, $8, $9, $10, $11,
					$12, $13, $14, $15, $16, $17,
					$18, $19, $20, $21, $22, $23, $24, $25, $26, $27,
					$28, $29, $30, $31,
					$32, $33, $34, $35, $36, $37,
					$38, $39, $40, $41, $42, $43,
					$44::qsl_sent_status, $45::qsl_status, $46, $47, $48::qsl_via, $49::qsl_via, $50, $51, $52,
					$53::qsl_sent_status, $54::qsl_status, $55, $56,
					$57::qsl_sent_status, $58::qsl_status, $59, $60, $61,
					$62, $63::qso_upload_status, $64, $65::qso_upload_status,
					COALESCE($66::jsonb, '{}'::jsonb), COALESCE($67::jsonb, '{}'::jsonb)
				)
			`

			_, err = pool.Exec(ctx, query,
				qso.Call,
				timestamp,
				timestamp,
				timeOff,
				qsoDateOff,
				nullableValue(band),
				nullableValue(freq),
				nullableValue(mode),
				nullableValue(bandRx),
				nullableValue(freqRx),
				nullableValue(submode),
				nullableValue(rstSent),
				nullableValue(rstRcvd),
				nullableValue(qth),
				nullableValue(name),
				nullableValue(comment),
				nullableValue(notes),
				nullableValue(gridSquare),
				nullableValue(country),
				nullableValue(dxcc),
				nullableValue(cqz),
				nullableValue(ituz),
				nullableValue(cont),
				nullableValue(state),
				nullableValue(cnty),
				nullableValue(pfx),
				nullableValue(iota),
				nullableValue(distance),
				nullableValue(aIndex),
				nullableValue(kIndex),
				nullableValue(sfi),
				nullableValue(myName),
				nullableValue(myCity),
				nullableValue(myCountry),
				nullableValue(myCQZone),
				nullableValue(myITUZone),
				nullableValue(myDXCC),
				nullableValue(myGridSquare),
				nullableValue(stationCall),
				nullableValue(operator),
				nullableValue(myRig),
				nullableValue(myAntenna),
				nullableValue(txPwr),
				nullableValue(qslSent),
				nullableValue(qslRcvd),
				qslsDate,
				qslrDate,
				nullableValue(qslSentVia),
				nullableValue(qslRcvdVia),
				nullableValue(qslVia),
				nullableValue(qslMsg),
				nullableValue(qslMsgRcvd),
				nullableValue(lotwSent),
				nullableValue(lotwRcvd),
				lotwQSLSDate,
				lotwQSLRDate,
				nullableValue(eqslSent),
				nullableValue(eqslRcvd),
				eqslQSLSDate,
				eqslQSLRDate,
				nullableValue(eqslAG),
				clublogUploadDate,
				nullableValue(clublogUploadStatus),
				hrdlogUploadDate,
				nullableValue(hrdlogUploadStatus),
				appFields,
				userFields,
			)
			if err != nil {
				return imported, fmt.Errorf("failed to insert QSO: %w", err)
			}

			imported++
		}
	}

	// After importing, try to link QSOs to contacts by matching call signs
	linkedQuery := `SELECT link_all_qsos_to_contacts()`

	var linkedCount int

	err := pool.QueryRow(ctx, linkedQuery).Scan(&linkedCount)
	if err != nil {
		// Log error but don't fail the import
		fmt.Printf("Warning: failed to auto-link QSOs to contacts: %v\n", err)
	} else if linkedCount > 0 {
		fmt.Printf("Auto-linked %d QSOs to contacts by call sign\n", linkedCount)
	}

	return imported + updated, nil
}

// ExportADIF exports QSOs in ADIF format, optionally filtered by date range.
func ExportADIF(ctx context.Context, fromDate *time.Time, toDate *time.Time) (string, error) {
	if pool == nil {
		return "", ErrDatabaseConnectionNotInitialized
	}

	var fromArg any
	if fromDate != nil {
		fromArg = fromDate.Format("2006-01-02")
	}

	var toArg any
	if toDate != nil {
		toArg = toDate.Format("2006-01-02")
	}

	const query = `SELECT export_adif($1::date, $2::date)`

	var adif string

	err := pool.QueryRow(ctx, query, fromArg, toArg).Scan(&adif)
	if err != nil {
		return "", fmt.Errorf("failed to export adif: %w", err)
	}

	return adif, nil
}

// qsoExists checks if a QSO with the same call, date, and time already exists
func qsoExists(ctx context.Context, call string, timestamp time.Time) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM qsos WHERE call = $1 AND qso_date = $2 AND time_on = $3)`

	var exists bool

	err := pool.QueryRow(ctx, query, call, timestamp, timestamp).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check existing qso: %w", err)
	}

	return exists, nil
}

// parseADIFTimestamp parses ADIF date and time into a timestamp
func parseADIFTimestamp(date, timeOn string) (time.Time, error) {
	// ADIF date format: YYYYMMDD
	// ADIF time format: HHMMSS or HHMM
	if len(date) != 8 {
		return time.Time{}, fmt.Errorf("%w: %s", ErrInvalidDateFormat, date)
	}

	// Pad time if needed
	if len(timeOn) == 4 {
		timeOn = timeOn + "00"
	}

	if len(timeOn) != 6 {
		return time.Time{}, fmt.Errorf("%w: %s", ErrInvalidTimeFormat, timeOn)
	}

	layout := "20060102150405"
	dateTime := date + timeOn

	parsed, err := time.Parse(layout, dateTime)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse adif timestamp %q: %w", dateTime, err)
	}

	return parsed, nil
}

// parseADIFDate parses ADIF date (YYYYMMDD) into a time.Time
func parseADIFDate(date string) (time.Time, error) {
	if len(date) != 8 {
		return time.Time{}, fmt.Errorf("%w: %s", ErrInvalidDateFormat, date)
	}

	parsed, err := time.Parse("20060102", date)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse adif date %q: %w", date, err)
	}

	return parsed, nil
}

func trimOptional(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}

	return &trimmed
}

func normalizeCNTY(cnty string, country *string) *string {
	normalized := trimOptional(cnty)
	if normalized == nil {
		return nil
	}

	if country == nil {
		return normalized
	}

	trimmedCountry := strings.TrimSpace(*country)
	if trimmedCountry == "" {
		return normalized
	}

	if strings.EqualFold(*normalized, trimmedCountry) {
		return nil
	}

	if isLettersAndSpaces(*normalized) {
		normalizedCNTYPhrase := normalizeAlnumPhrase(*normalized)
		normalizedCountryPhrase := normalizeAlnumPhrase(trimmedCountry)

		if normalizedCNTYPhrase != "" && normalizedCountryPhrase != "" {
			if phraseContainsPhrase(normalizedCountryPhrase, normalizedCNTYPhrase) || phraseContainsPhrase(normalizedCNTYPhrase, normalizedCountryPhrase) {
				return nil
			}
		}
	}

	return normalized
}

func isLettersAndSpaces(value string) bool {
	for _, r := range value {
		if r == ' ' {
			continue
		}

		if (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') {
			return false
		}
	}

	return true
}

func normalizeAlnumPhrase(value string) string {
	fields := strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	})

	return strings.Join(fields, " ")
}

func phraseContainsPhrase(haystack, needle string) bool {
	return strings.Contains(" "+haystack+" ", " "+needle+" ")
}

func parseOptionalADIFFloat(value string) *float64 {
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

func parseOptionalADIFInt(value string) *int {
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

func parseOptionalADIFDate(value string) *time.Time {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}

	parsed, err := parseADIFDate(trimmed)
	if err != nil {
		return nil
	}

	return &parsed
}

func parseOptionalADIFTimestamp(date string, timeOn string) *time.Time {
	trimmedDate := strings.TrimSpace(date)

	trimmedTime := strings.TrimSpace(timeOn)
	if trimmedDate == "" || trimmedTime == "" {
		return nil
	}

	parsed, err := parseADIFTimestamp(trimmedDate, trimmedTime)
	if err != nil {
		return nil
	}

	return &parsed
}

func parseOptionalADIFBool(value string) *bool {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "Y", "T", "TRUE", "1":
		parsed := true

		return &parsed
	case "N", "F", "FALSE", "0":
		parsed := false

		return &parsed
	default:
		return nil
	}
}

func normalizeQSLStatus(value string) *string {
	return normalizeADIFEnum(value, "Y", "N", "R", "I", "V")
}

func normalizeQSLSentStatus(value string) *string {
	return normalizeADIFEnum(value, "Y", "N", "R", "Q", "I")
}

func normalizeQSLVia(value string) *string {
	return normalizeADIFEnum(value, "B", "D", "E", "M")
}

func normalizeQSOUploadStatus(value string) *string {
	return normalizeADIFEnum(value, "Y", "N", "M")
}

func normalizeADIFEnum(value string, allowed ...string) *string {
	trimmed := strings.ToUpper(strings.TrimSpace(value))
	if trimmed == "" {
		return nil
	}

	for _, candidate := range allowed {
		if trimmed == candidate {
			return &trimmed
		}
	}

	return nil
}

func normalizeJSONFields(fields map[string]any) map[string]any {
	if len(fields) == 0 {
		return nil
	}

	normalized := make(map[string]any, len(fields))

	for key, value := range fields {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" || value == nil {
			continue
		}

		switch castValue := value.(type) {
		case string:
			trimmedValue := strings.TrimSpace(castValue)
			if trimmedValue == "" {
				continue
			}

			normalized[normalizedKey] = trimmedValue
		default:
			normalized[normalizedKey] = castValue
		}
	}

	if len(normalized) == 0 {
		return nil
	}

	return normalized
}

func nullableValue[T any](value *T) any {
	if value == nil {
		return nil
	}

	return *value
}

// GetQSO returns a single QSO by ID with contact information if linked
func GetQSO(ctx context.Context, id string) (*QSODetail, error) {
	if pool == nil {
		return nil, ErrDatabaseConnectionNotInitialized
	}

	query := `
		SELECT
			q.*,
			c.name_display as contact_name,
			c.call_sign as contact_call_sign
		FROM qsos q
		LEFT JOIN contacts c ON c.id = q.contact_id
		WHERE q.id = $1
	`

	var detail QSODetail

	detail.QSO = &QSO{} // Initialize the embedded pointer

	err := pool.QueryRow(ctx, query, id).Scan(
		&detail.ID,
		&detail.ContactID,
		&detail.Call,
		&detail.QSODate,
		&detail.TimeOn,
		&detail.Band,
		&detail.Freq,
		&detail.Mode,
		&detail.TimeOff,
		&detail.QSODateOff,
		&detail.BandRx,
		&detail.FreqRx,
		&detail.Submode,
		&detail.RSTSent,
		&detail.RSTRcvd,
		&detail.TxPwr,
		&detail.RxPwr,
		&detail.Name,
		&detail.QTH,
		&detail.GridSquare,
		&detail.GridSquareExt,
		&detail.VUCCGrids,
		&detail.Lat,
		&detail.Lon,
		&detail.Altitude,
		&detail.Country,
		&detail.DXCC,
		&detail.CQZ,
		&detail.ITUZ,
		&detail.Cont,
		&detail.State,
		&detail.Cnty,
		&detail.CntyAlt,
		&detail.Pfx,
		&detail.Age,
		&detail.Email,
		&detail.Web,
		&detail.Address,
		&detail.IOTA,
		&detail.IOTAIslandID,
		&detail.SOTARef,
		&detail.POTARef,
		&detail.WWFFRef,
		&detail.Sig,
		&detail.SigInfo,
		&detail.DARCDOK,
		&detail.FISTS,
		&detail.FISTSCC,
		&detail.SKCC,
		&detail.TenTen,
		&detail.UKSMG,
		&detail.USACACounties,
		&detail.ContactedOp,
		&detail.EQCall,
		&detail.GuestOp,
		&detail.SilentKey,
		&detail.StationCallsign,
		&detail.Operator,
		&detail.OwnerCallsign,
		&detail.MyName,
		&detail.MyGridSquare,
		&detail.MyGridSquareExt,
		&detail.MyVUCCGrids,
		&detail.MyLat,
		&detail.MyLon,
		&detail.MyAltitude,
		&detail.MyCity,
		&detail.MyStreet,
		&detail.MyPostalCode,
		&detail.MyState,
		&detail.MyCnty,
		&detail.MyCntyAlt,
		&detail.MyCountry,
		&detail.MyDXCC,
		&detail.MyCQZone,
		&detail.MyITUZone,
		&detail.MyIOTA,
		&detail.MyIOTAIslandID,
		&detail.MySOTARef,
		&detail.MyPOTARef,
		&detail.MyWWFFRef,
		&detail.MySig,
		&detail.MySigInfo,
		&detail.MyARRLSect,
		&detail.MyDARCDOK,
		&detail.MyFISTS,
		&detail.MyUSACACounties,
		&detail.MyRig,
		&detail.MyAntenna,
		&detail.Rig,
		&detail.MorseKeyType,
		&detail.MorseKeyInfo,
		&detail.MyMorseKeyType,
		&detail.MyMorseKeyInfo,
		&detail.PropMode,
		&detail.AntPath,
		&detail.AntAz,
		&detail.AntEl,
		&detail.Distance,
		&detail.AIndex,
		&detail.KIndex,
		&detail.SFI,
		&detail.SatName,
		&detail.SatMode,
		&detail.MSShower,
		&detail.MaxBursts,
		&detail.NrBursts,
		&detail.NrPings,
		&detail.ContestID,
		&detail.SRX,
		&detail.SRXString,
		&detail.STX,
		&detail.STXString,
		&detail.Class,
		&detail.ARRLSect,
		&detail.CheckField,
		&detail.Precedence,
		&detail.Region,
		&detail.QSOComplete,
		&detail.QSORandom,
		&detail.ForceInit,
		&detail.SWL,
		&detail.QSLSent,
		&detail.QSLSDate,
		&detail.QSLSentVia,
		&detail.QSLRcvd,
		&detail.QSLRDate,
		&detail.QSLRcvdVia,
		&detail.QSLVia,
		&detail.QSLMsg,
		&detail.QSLMsgRcvd,
		&detail.LoTWQSLSent,
		&detail.LoTWQSLSDate,
		&detail.LoTWQSLRcvd,
		&detail.LoTWQSLRDate,
		&detail.EQSLQSLSent,
		&detail.EQSLQSLSDate,
		&detail.EQSLQSLRcvd,
		&detail.EQSLQSLRDate,
		&detail.EQSLAG,
		&detail.ClublogQSOUploadDate,
		&detail.ClublogQSOUploadStatus,
		&detail.QRZComQSOUploadDate,
		&detail.QRZComQSOUploadStatus,
		&detail.QRZComQSODownloadDate,
		&detail.QRZComQSODownloadStatus,
		&detail.HRDLogQSOUploadDate,
		&detail.HRDLogQSOUploadStatus,
		&detail.HamLogEUQSOUploadDate,
		&detail.HamLogEUQSOUploadStatus,
		&detail.HamQTHQSOUploadDate,
		&detail.HamQTHQSOUploadStatus,
		&detail.DCLQSLSent,
		&detail.DCLQSLSDate,
		&detail.DCLQSLRcvd,
		&detail.DCLQSLRDate,
		&detail.AwardSubmitted,
		&detail.AwardGranted,
		&detail.CreditSubmitted,
		&detail.CreditGranted,
		&detail.Comment,
		&detail.Notes,
		&detail.PublicKey,
		&detail.AppFields,
		&detail.UserFields,
		&detail.CreatedAt,
		&detail.UpdatedAt,
		&detail.ContactName,
		&detail.ContactCallSign,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get QSO: %w", err)
	}

	return &detail, nil
}

// GetQSOsByCallSign returns all QSOs for a specific call sign
func GetQSOsByCallSign(ctx context.Context, callSign string) ([]QSOListItem, error) {
	if pool == nil {
		return nil, ErrDatabaseConnectionNotInitialized
	}

	query := `
		SELECT
			id,
			call,
			qso_date,
			time_on,
			band,
			mode,
			rst_sent,
			rst_rcvd,
			country,
			name,
			qth,
			state,
			gridsquare
		FROM qsos
		WHERE UPPER(call) = UPPER($1)
		ORDER BY qso_date DESC, time_on DESC
	`

	rows, err := pool.Query(ctx, query, callSign)
	if err != nil {
		return nil, fmt.Errorf("failed to query QSOs by call sign: %w", err)
	}
	defer rows.Close()

	var qsos []QSOListItem

	for rows.Next() {
		var qso QSOListItem

		err := rows.Scan(
			&qso.ID,
			&qso.Call,
			&qso.QSODate,
			&qso.TimeOn,
			&qso.Band,
			&qso.Mode,
			&qso.RSTSent,
			&qso.RSTRcvd,
			&qso.Country,
			&qso.Name,
			&qso.QTH,
			&qso.State,
			&qso.GridSquare,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan QSO: %w", err)
		}

		qsos = append(qsos, qso)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating QSOs: %w", err)
	}

	return qsos, nil
}

// GetQSOCount returns the total number of QSOs
func GetQSOCount(ctx context.Context) (int, error) {
	if pool == nil {
		return 0, ErrDatabaseConnectionNotInitialized
	}

	var count int

	query := `SELECT COUNT(*) FROM qsos`

	err := pool.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count QSOs: %w", err)
	}

	return count, nil
}

// GetUniqueCountriesCount returns the number of unique countries worked.
func GetUniqueCountriesCount(ctx context.Context) (int, error) {
	if pool == nil {
		return 0, ErrDatabaseConnectionNotInitialized
	}

	var count int

	query := `SELECT COUNT(DISTINCT country) FROM qsos WHERE country IS NOT NULL AND country <> ''`

	err := pool.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count unique countries: %w", err)
	}

	return count, nil
}

// GetLatestQSOTime returns the most recent QSO timestamp.
func GetLatestQSOTime(ctx context.Context) (*time.Time, error) {
	if pool == nil {
		return nil, ErrDatabaseConnectionNotInitialized
	}

	query := `SELECT MAX(qso_date + time_on) FROM qsos`

	var latest *time.Time

	err := pool.QueryRow(ctx, query).Scan(&latest)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest QSO time: %w", err)
	}

	return latest, nil
}

// GetPaperQSLHallOfFame returns deduplicated QSOs where paper QSL was received.
func GetPaperQSLHallOfFame(ctx context.Context) ([]QSOHallOfFameItem, error) {
	if pool == nil {
		return nil, ErrDatabaseConnectionNotInitialized
	}

	query := `
		SELECT DISTINCT ON (UPPER(call))
			call,
			name,
			country
		FROM qsos
		WHERE qsl_rcvd = 'Y'
		ORDER BY UPPER(call), (name IS NULL OR name = '') ASC, qso_date DESC, time_on DESC
	`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query paper QSL hall of fame: %w", err)
	}
	defer rows.Close()

	var entries []QSOHallOfFameItem

	for rows.Next() {
		var entry QSOHallOfFameItem

		err := rows.Scan(
			&entry.Call,
			&entry.Name,
			&entry.Country,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan hall of fame entry: %w", err)
		}

		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating hall of fame entries: %w", err)
	}

	return entries, nil
}

// FindClosestQSOByCallAndTime finds the closest matching QSO within a tolerance window.
func FindClosestQSOByCallAndTime(ctx context.Context, callSign string, searchTime time.Time, toleranceMinutes int) (*QSODetail, error) {
	if pool == nil {
		return nil, ErrDatabaseConnectionNotInitialized
	}

	query := `
		SELECT id
		FROM qsos
		WHERE UPPER(call) = UPPER($1)
		  AND abs(extract(epoch FROM ((qso_date + time_on) - $2::timestamp))) <= $3 * 60
		ORDER BY abs(extract(epoch FROM ((qso_date + time_on) - $2::timestamp))) ASC
		LIMIT 1
	`

	var id string

	err := pool.QueryRow(ctx, query, callSign, searchTime.UTC(), toleranceMinutes).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil //nolint:nilnil // No nearby QSO is a valid, non-error search result.
		}

		return nil, fmt.Errorf("failed to search QSO: %w", err)
	}

	return GetQSO(ctx, id)
}
