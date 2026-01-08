/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/humaidq/groundwave/utils"
)

// QSOListItem represents a QSO in the list view
type QSOListItem struct {
	ID      string    `db:"id"`
	Call    string    `db:"call"`
	QSODate time.Time `db:"qso_date"`
	TimeOn  time.Time `db:"time_on"`
	Band    *string   `db:"band"`
	Mode    string    `db:"mode"`
	RSTSent *string   `db:"rst_sent"`
	RSTRcvd *string   `db:"rst_rcvd"`
	Country *string   `db:"country"`
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

// QSODetail represents a full QSO with all details
type QSODetail struct {
	*QSO                    // Embed pointer instead of value for proper method access
	ContactName     *string `db:"contact_name"`
	ContactCallSign *string `db:"contact_call_sign"`
}

// ListQSOs returns all QSOs sorted by date/time (most recent first)
func ListQSOs(ctx context.Context) ([]QSOListItem, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
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
			country
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
		return nil, fmt.Errorf("database connection not initialized")
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
			country
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
		return 0, fmt.Errorf("database connection not initialized")
	}

	imported := 0
	updated := 0

	for _, qso := range qsos {
		// Parse timestamp
		timestamp, err := parseADIFTimestamp(qso.QSODate, qso.TimeOn)
		if err != nil {
			// Skip QSOs with invalid timestamps
			continue
		}

		// Parse optional time_off
		var timeOff *time.Time
		if qso.QSODateOff != "" && qso.TimeOff != "" {
			t, err := parseADIFTimestamp(qso.QSODateOff, qso.TimeOff)
			if err == nil {
				timeOff = &t
			}
		}

		// Check if QSO already exists (by call, date, and time)
		exists, err := qsoExists(ctx, qso.Call, timestamp)
		if err != nil {
			return imported, fmt.Errorf("failed to check if QSO exists: %w", err)
		}

		if exists {
			// UPDATE existing QSO with merge logic
			// COALESCE keeps existing value if new value is NULL
			query := `
				UPDATE qsos SET
					band = COALESCE(NULLIF($1, ''), band),
					freq = CASE WHEN $2::text != '' THEN $2::double precision ELSE freq END,
					mode = COALESCE(NULLIF($3, ''), mode),
					time_off = COALESCE($4, time_off),
					qso_date_off = COALESCE($4, qso_date_off),
					rst_sent = COALESCE(NULLIF($5, ''), rst_sent),
					rst_rcvd = COALESCE(NULLIF($6, ''), rst_rcvd),
					qth = COALESCE(NULLIF($7, ''), qth),
					name = COALESCE(NULLIF($8, ''), name),
					comment = COALESCE(NULLIF($9, ''), comment),
					gridsquare = COALESCE(NULLIF($10, ''), gridsquare),
					country = COALESCE(NULLIF($11, ''), country),
					dxcc = CASE WHEN $12::text != '' THEN $12::integer ELSE dxcc END,
					my_gridsquare = COALESCE(NULLIF($13, ''), my_gridsquare),
					station_callsign = COALESCE(NULLIF($14, ''), station_callsign),
					my_rig = COALESCE(NULLIF($15, ''), my_rig),
					my_antenna = COALESCE(NULLIF($16, ''), my_antenna),
					tx_pwr = CASE WHEN $17::text != '' THEN $17::double precision ELSE tx_pwr END,
					qsl_sent = CASE WHEN $18 != '' THEN $18::qsl_sent_status ELSE qsl_sent END,
					qsl_rcvd = CASE WHEN $19 != '' THEN $19::qsl_status ELSE qsl_rcvd END,
					lotw_qsl_sent = CASE WHEN $20 != '' THEN $20::qsl_sent_status ELSE lotw_qsl_sent END,
					lotw_qsl_rcvd = CASE WHEN $21 != '' THEN $21::qsl_status ELSE lotw_qsl_rcvd END,
					eqsl_qsl_sent = CASE WHEN $22 != '' THEN $22::qsl_sent_status ELSE eqsl_qsl_sent END,
					eqsl_qsl_rcvd = CASE WHEN $23 != '' THEN $23::qsl_status ELSE eqsl_qsl_rcvd END,
					updated_at = NOW()
				WHERE call = $24 AND qso_date = $25 AND time_on = $26
			`
			_, err = pool.Exec(ctx, query,
				qso.Band,
				qso.Freq,
				qso.Mode,
				timeOff,
				qso.RSTSent,
				qso.RSTRcvd,
				qso.QTH,
				qso.Name,
				qso.Comment,
				qso.GridSquare,
				qso.Country,
				qso.DXCC,
				qso.MyGridSquare,
				qso.StationCall,
				qso.MyRig,
				qso.MyAntenna,
				qso.TxPwr,
				string(qso.QslSent),
				string(qso.QslRcvd),
				string(qso.LotwSent),
				string(qso.LotwRcvd),
				string(qso.EqslSent),
				string(qso.EqslRcvd),
				qso.Call,
				timestamp,
				timestamp,
			)
			if err != nil {
				return imported, fmt.Errorf("failed to update QSO: %w", err)
			}
			updated++
		} else {
			// INSERT new QSO
			query := `
				INSERT INTO qsos (
					call, qso_date, time_on, time_off, qso_date_off,
					band, freq, mode, rst_sent, rst_rcvd,
					qth, name, comment, gridsquare, country, dxcc,
					my_gridsquare, station_callsign, my_rig, my_antenna, tx_pwr,
					qsl_sent, qsl_rcvd, lotw_qsl_sent, lotw_qsl_rcvd,
					eqsl_qsl_sent, eqsl_qsl_rcvd
				) VALUES (
					$1, $2, $3, $4, $5,
					NULLIF($6, ''), CASE WHEN $7 != '' THEN $7::double precision ELSE NULL END, $8,
					NULLIF($9, ''), NULLIF($10, ''),
					NULLIF($11, ''), NULLIF($12, ''), NULLIF($13, ''),
					NULLIF($14, ''), NULLIF($15, ''),
					CASE WHEN $16 != '' THEN $16::integer ELSE NULL END,
					NULLIF($17, ''), NULLIF($18, ''), NULLIF($19, ''),
					NULLIF($20, ''), CASE WHEN $21 != '' THEN $21::double precision ELSE NULL END,
					CASE WHEN $22 != '' THEN $22::qsl_sent_status ELSE NULL END,
					CASE WHEN $23 != '' THEN $23::qsl_status ELSE NULL END,
					CASE WHEN $24 != '' THEN $24::qsl_sent_status ELSE NULL END,
					CASE WHEN $25 != '' THEN $25::qsl_status ELSE NULL END,
					CASE WHEN $26 != '' THEN $26::qsl_sent_status ELSE NULL END,
					CASE WHEN $27 != '' THEN $27::qsl_status ELSE NULL END
				)
			`
			_, err = pool.Exec(ctx, query,
				qso.Call,
				timestamp,
				timestamp,
				timeOff,
				timeOff,
				qso.Band,
				qso.Freq,
				qso.Mode,
				qso.RSTSent,
				qso.RSTRcvd,
				qso.QTH,
				qso.Name,
				qso.Comment,
				qso.GridSquare,
				qso.Country,
				qso.DXCC,
				qso.MyGridSquare,
				qso.StationCall,
				qso.MyRig,
				qso.MyAntenna,
				qso.TxPwr,
				string(qso.QslSent),
				string(qso.QslRcvd),
				string(qso.LotwSent),
				string(qso.LotwRcvd),
				string(qso.EqslSent),
				string(qso.EqslRcvd),
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

// qsoExists checks if a QSO with the same call, date, and time already exists
func qsoExists(ctx context.Context, call string, timestamp time.Time) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM qsos WHERE call = $1 AND qso_date = $2 AND time_on = $3)`
	var exists bool
	err := pool.QueryRow(ctx, query, call, timestamp, timestamp).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// parseADIFTimestamp parses ADIF date and time into a timestamp
func parseADIFTimestamp(date, timeOn string) (time.Time, error) {
	// ADIF date format: YYYYMMDD
	// ADIF time format: HHMMSS or HHMM
	if len(date) != 8 {
		return time.Time{}, fmt.Errorf("invalid date format: %s", date)
	}

	// Pad time if needed
	if len(timeOn) == 4 {
		timeOn = timeOn + "00"
	}
	if len(timeOn) != 6 {
		return time.Time{}, fmt.Errorf("invalid time format: %s", timeOn)
	}

	layout := "20060102150405"
	dateTime := date + timeOn
	return time.Parse(layout, dateTime)
}

// GetQSO returns a single QSO by ID with contact information if linked
func GetQSO(ctx context.Context, id string) (*QSODetail, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
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
		return nil, fmt.Errorf("database connection not initialized")
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
			country
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
		return 0, fmt.Errorf("database connection not initialized")
	}

	var count int
	query := `SELECT COUNT(*) FROM qsos`
	err := pool.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count QSOs: %w", err)
	}

	return count, nil
}
