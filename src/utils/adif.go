/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package utils

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// QslStatus represents ADIF QSL status values.
type QslStatus string

// QslStatus values represent QSL confirmation states.
const (
	QslYes       QslStatus = "Y" // Yes
	QslNo        QslStatus = "N" // No
	QslRequested QslStatus = "R" // Requested
	QslInvalid   QslStatus = "I" // Invalid/Ignore
	QslEmpty     QslStatus = ""  // Empty/Unknown
)

// QSO represents one ADIF contact record.
type QSO struct {
	Call         string
	QSODate      string // YYYYMMDD format
	TimeOn       string // HHMMSS format
	QSODateOff   string // YYYYMMDD format (optional)
	TimeOff      string // HHMMSS format (optional)
	Band         string
	Mode         string
	Freq         string
	RSTSent      string
	RSTRcvd      string
	QTH          string
	Name         string
	Comment      string
	GridSquare   string
	Country      string
	DXCC         string
	MyGridSquare string
	StationCall  string
	MyRig        string
	MyAntenna    string
	TxPwr        string
	QslSent      QslStatus
	QslRcvd      QslStatus
	LotwSent     QslStatus
	LotwRcvd     QslStatus
	EqslSent     QslStatus
	EqslRcvd     QslStatus
	Timestamp    time.Time // Parsed datetime for easier searching
}

// ADIFParser parses ADIF content and stores parsed QSOs.
type ADIFParser struct {
	QSOs []QSO
}

// NewADIFParser creates an empty ADIF parser.
func NewADIFParser() *ADIFParser {
	return &ADIFParser{
		QSOs: make([]QSO, 0),
	}
}

// ParseFile reads and parses ADIF data from the provided reader.
func (p *ADIFParser) ParseFile(reader io.Reader) error {
	content, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read ADIF file: %w", err)
	}

	return p.parseContent(string(content))
}

func (p *ADIFParser) parseContent(content string) error {
	// Remove header if present (everything before <EOH>)
	eohIndex := strings.Index(strings.ToUpper(content), "<EOH>")
	if eohIndex != -1 {
		content = content[eohIndex+5:]
	}

	// Split into records using <eor> delimiter (case insensitive)
	records := regexp.MustCompile(`(?i)<eor>`).Split(content, -1)

	for _, record := range records {
		record = strings.TrimSpace(record)
		if record == "" {
			continue
		}

		qso, err := p.parseRecord(record)
		if err != nil {
			// Skip malformed records but continue parsing
			continue
		}

		p.QSOs = append(p.QSOs, qso)
	}

	return nil
}

func (p *ADIFParser) parseRecord(record string) (QSO, error) {
	qso := QSO{}

	// Regex to match ADIF fields: <FIELDNAME:LENGTH>DATA
	fieldRegex := regexp.MustCompile(`<([^:>]+):(\d+)>([^<]*)`)
	matches := fieldRegex.FindAllStringSubmatch(record, -1)

	for _, match := range matches {
		fieldName := strings.ToLower(strings.TrimSpace(match[1]))
		lengthStr := match[2]
		data := match[3]

		// Parse field length and extract the correct amount of data
		length, err := strconv.Atoi(lengthStr)
		if err != nil {
			continue
		}

		if len(data) < length {
			continue
		}

		fieldValue := strings.TrimSpace(data[:length])

		// Map fields to QSO struct
		switch fieldName {
		case "call":
			qso.Call = strings.ToUpper(fieldValue)
		case "qso_date":
			qso.QSODate = fieldValue
		case "time_on":
			qso.TimeOn = fieldValue
		case "qso_date_off":
			qso.QSODateOff = fieldValue
		case "time_off":
			qso.TimeOff = fieldValue
		case "band":
			qso.Band = fieldValue
		case "mode":
			qso.Mode = fieldValue
		case "freq":
			qso.Freq = fieldValue
		case "rst_sent":
			qso.RSTSent = fieldValue
		case "rst_rcvd":
			qso.RSTRcvd = fieldValue
		case "qth":
			qso.QTH = fieldValue
		case "name":
			qso.Name = fieldValue
		case "comment":
			qso.Comment = fieldValue
		case "gridsquare":
			qso.GridSquare = fieldValue
		case "country":
			qso.Country = fieldValue
		case "dxcc":
			qso.DXCC = fieldValue
		case "my_gridsquare":
			qso.MyGridSquare = fieldValue
		case "station_callsign":
			qso.StationCall = fieldValue
		case "my_rig":
			qso.MyRig = fieldValue
		case "my_antenna":
			qso.MyAntenna = fieldValue
		case "tx_pwr":
			qso.TxPwr = fieldValue
		case "qsl_sent":
			qso.QslSent = QslStatus(fieldValue)
		case "qsl_rcvd":
			qso.QslRcvd = QslStatus(fieldValue)
		case "lotw_qsl_sent":
			qso.LotwSent = QslStatus(fieldValue)
		case "lotw_qsl_rcvd":
			qso.LotwRcvd = QslStatus(fieldValue)
		case "eqsl_qsl_sent":
			qso.EqslSent = QslStatus(fieldValue)
		case "eqsl_qsl_rcvd":
			qso.EqslRcvd = QslStatus(fieldValue)
		}
	}

	// Parse timestamp for easier searching
	if qso.QSODate != "" && qso.TimeOn != "" {
		timestamp, err := p.parseTimestamp(qso.QSODate, qso.TimeOn)
		if err == nil {
			qso.Timestamp = timestamp
		}
	}

	// Validate required fields
	if qso.Call == "" || qso.QSODate == "" {
		return qso, errMissingRequiredADIFFields
	}

	return qso, nil
}

func (p *ADIFParser) parseTimestamp(date, timeOn string) (time.Time, error) {
	// ADIF date format: YYYYMMDD
	// ADIF time format: HHMMSS
	if len(date) != 8 || len(timeOn) != 6 {
		return time.Time{}, errInvalidDateTimeFormat
	}

	// Parse components
	year, err := strconv.Atoi(date[0:4])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid adif year %q: %w", date[0:4], err)
	}

	month, err := strconv.Atoi(date[4:6])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid adif month %q: %w", date[4:6], err)
	}

	day, err := strconv.Atoi(date[6:8])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid adif day %q: %w", date[6:8], err)
	}

	hour, err := strconv.Atoi(timeOn[0:2])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid adif hour %q: %w", timeOn[0:2], err)
	}

	minute, err := strconv.Atoi(timeOn[2:4])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid adif minute %q: %w", timeOn[2:4], err)
	}

	second, err := strconv.Atoi(timeOn[4:6])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid adif second %q: %w", timeOn[4:6], err)
	}

	return time.Date(year, time.Month(month), day, hour, minute, second, 0, time.UTC), nil
}

// SearchQSO finds the closest QSO matching call sign and time with fuzzy matching
func (p *ADIFParser) SearchQSO(callSign string, searchTime time.Time, toleranceMinutes int) []QSO {
	callSign = strings.ToUpper(strings.TrimSpace(callSign))

	tolerance := time.Duration(toleranceMinutes) * time.Minute

	var (
		bestMatch    QSO
		bestTimeDiff time.Duration
	)

	found := false

	for _, qso := range p.QSOs {
		// Match call sign (exact match)
		if qso.Call != callSign {
			continue
		}

		// Check if QSO timestamp is within tolerance
		if !qso.Timestamp.IsZero() {
			timeDiff := qso.Timestamp.Sub(searchTime)
			if timeDiff < 0 {
				timeDiff = -timeDiff
			}

			if timeDiff <= tolerance {
				// If this is the first match or closer than previous best match
				if !found || timeDiff < bestTimeDiff {
					bestMatch = qso
					bestTimeDiff = timeDiff
					found = true
				}
			}
		}
	}

	if found {
		return []QSO{bestMatch}
	}

	return []QSO{}
}

// GetQSOsByCallsign returns all QSOs for a specific call sign
func (p *ADIFParser) GetQSOsByCallsign(callSign string) []QSO {
	callSign = strings.ToUpper(strings.TrimSpace(callSign))

	var results []QSO

	for _, qso := range p.QSOs {
		if qso.Call == callSign {
			results = append(results, qso)
		}
	}

	return results
}

// GetTotalQSOCount returns the total number of QSOs
func (p *ADIFParser) GetTotalQSOCount() int {
	return len(p.QSOs)
}

// GetUniqueCountriesCount returns the number of unique countries worked
func (p *ADIFParser) GetUniqueCountriesCount() int {
	countries := make(map[string]bool)

	for _, qso := range p.QSOs {
		if qso.Country != "" {
			countries[qso.Country] = true
		}
	}

	return len(countries)
}

// GetLatestQSOs returns the most recent QSOs, sorted by timestamp
func (p *ADIFParser) GetLatestQSOs(limit int) []QSO {
	if len(p.QSOs) == 0 {
		return []QSO{}
	}

	// Create a copy and sort by timestamp (newest first)
	qsos := make([]QSO, len(p.QSOs))
	copy(qsos, p.QSOs)

	// Simple bubble sort by timestamp (newest first)
	for i := range len(qsos) - 1 {
		for j := range len(qsos) - i - 1 {
			if qsos[j].Timestamp.Before(qsos[j+1].Timestamp) {
				qsos[j], qsos[j+1] = qsos[j+1], qsos[j]
			}
		}
	}

	if len(qsos) < limit {
		return qsos
	}

	return qsos[:limit]
}

// GetQSOs returns all parsed QSOs
func (p *ADIFParser) GetQSOs() []QSO {
	return p.QSOs
}

// GetLatestQSO returns the most recent QSO by timestamp
func (p *ADIFParser) GetLatestQSO() *QSO {
	if len(p.QSOs) == 0 {
		return nil
	}

	var latest *QSO

	for i := range p.QSOs {
		if p.QSOs[i].Timestamp.IsZero() {
			continue
		}

		if latest == nil || p.QSOs[i].Timestamp.After(latest.Timestamp) {
			latest = &p.QSOs[i]
		}
	}

	return latest
}

// GetPaperQSLHallOfFame returns deduplicated QSOs where paper QSL was received
func (p *ADIFParser) GetPaperQSLHallOfFame() []QSO {
	seen := make(map[string]QSO)

	for _, qso := range p.QSOs {
		// Only include QSOs where paper QSL was received
		if qso.QslRcvd == QslYes {
			// Use callsign as the key for deduplication
			if existing, exists := seen[qso.Call]; !exists {
				seen[qso.Call] = qso
			} else {
				// If we already have this callsign, prefer the one with a name
				if qso.Name != "" && existing.Name == "" {
					seen[qso.Call] = qso
				}
			}
		}
	}

	// Convert map to slice and sort by callsign
	result := make([]QSO, 0, len(seen))
	for _, qso := range seen {
		result = append(result, qso)
	}

	// Simple bubble sort by callsign
	sort.Slice(result, func(i, j int) bool {
		return result[i].Call < result[j].Call
	})

	return result
}

// FormatQSOTime formats QSO timestamp for display
func (qso QSO) FormatQSOTime() string {
	if !qso.Timestamp.IsZero() {
		return qso.Timestamp.UTC().Format("2006-01-02 15:04:05 UTC")
	}

	return fmt.Sprintf("%s %s UTC", qso.QSODate, qso.TimeOn)
}

// FormatDate formats QSO date with dashes (YYYY-MM-DD)
func (qso QSO) FormatDate() string {
	if len(qso.QSODate) == 8 {
		return fmt.Sprintf("%s-%s-%s", qso.QSODate[0:4], qso.QSODate[4:6], qso.QSODate[6:8])
	}

	return qso.QSODate
}

// FormatTime formats QSO time with colons and no seconds (HH:MM)
func (qso QSO) FormatTime() string {
	if len(qso.TimeOn) >= 4 {
		return fmt.Sprintf("%s:%s", qso.TimeOn[0:2], qso.TimeOn[2:4])
	}

	return qso.TimeOn
}

// GetFlagCode returns the ISO 3166-1 alpha-2 country code for flagcdn.com
func (qso QSO) GetFlagCode() string {
	return CountryFlagCode(qso.Country)
}

// CountryFlagCode returns the ISO 3166-1 alpha-2 country code for flagcdn.com.
func CountryFlagCode(country string) string {
	if country == "" {
		return ""
	}

	countryMap := map[string]string{
		// From ADIF data analysis
		"Albania":              "al",
		"Armenia":              "am",
		"Asiatic Russia":       "ru",
		"Asiatic Turkey":       "tr",
		"Australia":            "au",
		"Austria":              "at",
		"Bahrain":              "bh",
		"Belarus":              "by",
		"Belgium":              "be",
		"Bosnia-Herzegovina":   "ba",
		"Brazil":               "br",
		"Brunei Darussalam":    "bn",
		"Bulgaria":             "bg",
		"Canary Islands":       "es", // Part of Spain
		"Chile":                "cl",
		"China":                "cn",
		"Comoros":              "km",
		"Crete":                "gr", // Part of Greece
		"Croatia":              "hr",
		"Cyprus":               "cy",
		"Czech Republic":       "cz",
		"Denmark":              "dk",
		"Dodecanese":           "gr", // Part of Greece
		"England":              "gb",
		"Estonia":              "ee",
		"European Russia":      "ru",
		"Fed. Rep. of Germany": "de",
		"Finland":              "fi",
		"France":               "fr",
		"Georgia":              "ge",
		"Greece":               "gr",
		"Hungary":              "hu",
		"India":                "in",
		"Indonesia":            "id",
		"Iraq":                 "iq",
		"Israel":               "il",
		"Italy":                "it",
		"Japan":                "jp",
		"Jersey":               "je",
		"Kazakhstan":           "kz",
		"Kyrgyzstan":           "kg",
		"Laos":                 "la",
		"Latvia":               "lv",
		"Lebanon":              "lb",
		"Lithuania":            "lt",
		"Madeira Islands":      "pt", // Part of Portugal
		"Malawi":               "mw",
		"Montenegro":           "me",
		"Namibia":              "na",
		"Netherlands":          "nl",
		"Northern Ireland":     "gb",
		"Norway":               "no",
		"Pakistan":             "pk",
		"Poland":               "pl",
		"Portugal":             "pt",
		"Puerto Rico":          "pr",
		"Qatar":                "qa",
		"Republic of Korea":    "kr",
		"Romania":              "ro",
		"Sardinia":             "it", // Part of Italy
		"Saudi Arabia":         "sa",
		"Scotland":             "gb",
		"Serbia":               "rs",
		"Singapore":            "sg",
		"Slovak Republic":      "sk",
		"Slovenia":             "si",
		"South Africa":         "za",
		"Spain":                "es",
		"Sri Lanka":            "lk",
		"Sweden":               "se",
		"Switzerland":          "ch",
		"Taiwan":               "tw",
		"Thailand":             "th",
		"Ukraine":              "ua",
		"United Arab Emirates": "ae",
		"United States":        "us",
		"Uzbekistan":           "uz",
		"Wales":                "gb",
		"West Malaysia":        "my",

		// Additional common mappings
		"Germany":        "de",
		"United Kingdom": "gb",
		"Russia":         "ru",
		"Turkey":         "tr",
		"South Korea":    "kr",
		"Malaysia":       "my",
	}

	if code, exists := countryMap[country]; exists {
		return code
	}

	return ""
}
