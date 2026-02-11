// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var errTestReadFailed = errors.New("read failed")

func TestADIFParserParseFileSkipsMalformedRecords(t *testing.T) {
	t.Parallel()

	parser := NewADIFParser()
	content := strings.Join([]string{
		"Some header line",
		"<EOH>",
		"<call:5>ab1cd<qso_date:8>20240102<time_on:6>130501<band:3>20m<country:5>Japan<eor>",
		"<qso_date:8>20240103<time_on:6>130502<EOR>",
	}, "\n")

	err := parser.ParseFile(strings.NewReader(content))
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if len(parser.QSOs) != 1 {
		t.Fatalf("expected 1 parsed QSO, got %d", len(parser.QSOs))
	}

	qso := parser.QSOs[0]
	if qso.Call != "AB1CD" {
		t.Fatalf("expected CALL to be uppercased, got %q", qso.Call)
	}

	if qso.Band != "20m" {
		t.Fatalf("expected band 20m, got %q", qso.Band)
	}

	if qso.Country != "Japan" {
		t.Fatalf("expected country Japan, got %q", qso.Country)
	}

	expected := time.Date(2024, time.January, 2, 13, 5, 1, 0, time.UTC)
	if !qso.Timestamp.Equal(expected) {
		t.Fatalf("expected timestamp %v, got %v", expected, qso.Timestamp)
	}
}

func TestADIFParserParseRecordMissingRequiredFields(t *testing.T) {
	t.Parallel()

	parser := NewADIFParser()

	if _, err := parser.parseRecord("<call:3>ABC"); err == nil {
		t.Fatalf("expected error when QSO_DATE is missing")
	}

	if _, err := parser.parseRecord("<qso_date:8>20240102"); err == nil {
		t.Fatalf("expected error when CALL is missing")
	}
}

func TestADIFParserParseRecordInvalidFieldLength(t *testing.T) {
	t.Parallel()

	parser := NewADIFParser()

	_, err := parser.parseRecord("<call:6>ABC<qso_date:8>20240102")
	if err == nil {
		t.Fatalf("expected error when CALL length exceeds data")
	}
}

func TestADIFParserParseRecordAllFields(t *testing.T) {
	t.Parallel()

	parser := NewADIFParser()

	record := strings.Join([]string{
		"<call:5>K1ABC",
		"<qso_date:8>20240102",
		"<time_on:6>010203",
		"<qso_date_off:8>20240102",
		"<time_off:6>010204",
		"<band:3>20m",
		"<mode:3>SSB",
		"<freq:4>14.2",
		"<rst_sent:2>59",
		"<rst_rcvd:2>58",
		"<qth:6>Boston",
		"<name:4>John",
		"<comment:4>Test",
		"<gridsquare:4>FN31",
		"<country:5>Japan",
		"<dxcc:3>291",
		"<my_gridsquare:4>FN42",
		"<station_callsign:5>AB1CD",
		"<my_rig:5>IC730",
		"<my_antenna:3>Dip",
		"<tx_pwr:3>100",
		"<qsl_sent:1>Y",
		"<qsl_rcvd:1>N",
		"<lotw_qsl_sent:1>R",
		"<lotw_qsl_rcvd:1>Y",
		"<eqsl_qsl_sent:1>N",
		"<eqsl_qsl_rcvd:1>Y",
	}, "")

	qso, err := parser.parseRecord(record)
	if err != nil {
		t.Fatalf("parseRecord failed: %v", err)
	}

	if qso.Call != "K1ABC" || qso.QSODate != "20240102" || qso.TimeOn != "010203" {
		t.Fatalf("unexpected core fields: %+v", qso)
	}

	if qso.QSODateOff != "20240102" || qso.TimeOff != "010204" {
		t.Fatalf("unexpected off fields: %+v", qso)
	}

	if qso.Band != "20m" || qso.Mode != "SSB" || qso.Freq != "14.2" {
		t.Fatalf("unexpected band/mode/freq: %+v", qso)
	}

	if qso.RSTSent != "59" || qso.RSTRcvd != "58" {
		t.Fatalf("unexpected rst fields: %+v", qso)
	}

	if qso.QTH != "Boston" || qso.Name != "John" || qso.Comment != "Test" {
		t.Fatalf("unexpected text fields: %+v", qso)
	}

	if qso.GridSquare != "FN31" || qso.Country != "Japan" || qso.DXCC != "291" {
		t.Fatalf("unexpected location fields: %+v", qso)
	}

	if qso.MyGridSquare != "FN42" || qso.StationCall != "AB1CD" {
		t.Fatalf("unexpected station fields: %+v", qso)
	}

	if qso.MyRig != "IC730" || qso.MyAntenna != "Dip" || qso.TxPwr != "100" {
		t.Fatalf("unexpected equipment fields: %+v", qso)
	}

	if qso.QslSent != QslYes || qso.QslRcvd != QslNo || qso.LotwSent != QslRequested || qso.LotwRcvd != QslYes || qso.EqslSent != QslNo || qso.EqslRcvd != QslYes {
		t.Fatalf("unexpected qsl fields: %+v", qso)
	}
}

func TestADIFParserParseRecordLengthOverflow(t *testing.T) {
	t.Parallel()

	parser := NewADIFParser()

	record := "<call:999999999999999999999999999999>ABC<qso_date:8>20240102"
	if _, err := parser.parseRecord(record); err == nil {
		t.Fatalf("expected error for length overflow record")
	}
}

func TestADIFParserParseFileReadError(t *testing.T) {
	t.Parallel()

	parser := NewADIFParser()

	reader := errorReader{}
	if err := parser.ParseFile(reader); err == nil {
		t.Fatalf("expected error from ParseFile")
	}
}

func TestADIFParserParseTimestamp(t *testing.T) {
	t.Parallel()

	parser := NewADIFParser()

	expected := time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC)

	parsed, err := parser.parseTimestamp("20240102", "030405")
	if err != nil {
		t.Fatalf("parseTimestamp failed: %v", err)
	}

	if !parsed.Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, parsed)
	}

	if _, err := parser.parseTimestamp("202401", "030405"); err == nil {
		t.Fatalf("expected error for invalid date length")
	}

	if _, err := parser.parseTimestamp("20240102", "0304"); err == nil {
		t.Fatalf("expected error for invalid time length")
	}

	if _, err := parser.parseTimestamp("20aa0102", "030405"); err == nil {
		t.Fatalf("expected error for non-numeric year")
	}

	if _, err := parser.parseTimestamp("2024010a", "030405"); err == nil {
		t.Fatalf("expected error for non-numeric date")
	}

	if _, err := parser.parseTimestamp("2024aa02", "030405"); err == nil {
		t.Fatalf("expected error for non-numeric month")
	}

	if _, err := parser.parseTimestamp("202401aa", "030405"); err == nil {
		t.Fatalf("expected error for non-numeric day")
	}

	if _, err := parser.parseTimestamp("20240102", "aa0405"); err == nil {
		t.Fatalf("expected error for non-numeric hour")
	}

	if _, err := parser.parseTimestamp("20240102", "03aa05"); err == nil {
		t.Fatalf("expected error for non-numeric minute")
	}

	if _, err := parser.parseTimestamp("20240102", "0304aa"); err == nil {
		t.Fatalf("expected error for non-numeric second")
	}
}

func TestADIFParserSearchQSO(t *testing.T) {
	t.Parallel()

	parser := NewADIFParser()
	first := time.Date(2024, time.January, 2, 10, 0, 0, 0, time.UTC)
	second := time.Date(2024, time.January, 2, 10, 10, 0, 0, time.UTC)
	parser.QSOs = []QSO{
		{Call: "K1ABC", Timestamp: first},
		{Call: "K1ABC", Timestamp: second},
		{Call: "K2DEF", Timestamp: second},
	}

	searchTime := time.Date(2024, time.January, 2, 10, 6, 0, 0, time.UTC)

	results := parser.SearchQSO("k1abc", searchTime, 10)
	if len(results) != 1 {
		t.Fatalf("expected 1 match, got %d", len(results))
	}

	if !results[0].Timestamp.Equal(second) {
		t.Fatalf("expected closest match at %v, got %v", second, results[0].Timestamp)
	}

	results = parser.SearchQSO("k1abc", searchTime, 3)
	if len(results) != 0 {
		t.Fatalf("expected no matches outside tolerance, got %d", len(results))
	}
}

func TestADIFParserGettersAndSorting(t *testing.T) {
	t.Parallel()

	parser := NewADIFParser()
	first := time.Date(2024, time.January, 2, 10, 0, 0, 0, time.UTC)
	second := time.Date(2024, time.January, 3, 10, 0, 0, 0, time.UTC)
	third := time.Date(2024, time.January, 4, 10, 0, 0, 0, time.UTC)
	parser.QSOs = []QSO{
		{Call: "K1ABC", Country: "Japan", Timestamp: first},
		{Call: "K1ABC", Country: "Japan", Timestamp: second},
		{Call: "K2DEF", Country: "France", Timestamp: third},
		{Call: "K3GHI", Country: "", Timestamp: time.Time{}},
	}

	if got := parser.GetTotalQSOCount(); got != 4 {
		t.Fatalf("expected 4 QSOs, got %d", got)
	}

	if got := parser.GetUniqueCountriesCount(); got != 2 {
		t.Fatalf("expected 2 unique countries, got %d", got)
	}

	if got := parser.GetQSOsByCallsign("k1abc"); len(got) != 2 {
		t.Fatalf("expected 2 QSOs for call, got %d", len(got))
	}

	latest := parser.GetLatestQSOs(2)
	if len(latest) != 2 {
		t.Fatalf("expected 2 latest QSOs, got %d", len(latest))
	}

	if latest[0].Call != "K2DEF" {
		t.Fatalf("expected latest call K2DEF, got %q", latest[0].Call)
	}

	if latest[1].Call != "K1ABC" {
		t.Fatalf("expected second latest call K1ABC, got %q", latest[1].Call)
	}

	latestAll := parser.GetLatestQSOs(10)
	if len(latestAll) != 4 {
		t.Fatalf("expected all QSOs when limit is large, got %d", len(latestAll))
	}

	latestOne := parser.GetLatestQSO()
	if latestOne == nil || latestOne.Call != "K2DEF" {
		t.Fatalf("expected latest QSO K2DEF")
	}

	parser.QSOs = []QSO{{Call: "ZERO"}}
	if got := parser.GetLatestQSO(); got != nil {
		t.Fatalf("expected nil latest QSO when timestamps are zero")
	}

	parser.QSOs = []QSO{}
	if got := parser.GetLatestQSO(); got != nil {
		t.Fatalf("expected nil latest QSO when no QSOs exist")
	}
}

func TestADIFParserGetPaperQSLHallOfFame(t *testing.T) {
	t.Parallel()

	parser := NewADIFParser()
	parser.QSOs = []QSO{
		{Call: "K1ABC", Name: "", QslRcvd: QslYes},
		{Call: "K1ABC", Name: "Alice", QslRcvd: QslYes},
		{Call: "Z9XYZ", Name: "Zed", QslRcvd: QslYes},
		{Call: "B2BBB", Name: "Bob", QslRcvd: QslNo},
	}

	results := parser.GetPaperQSLHallOfFame()
	if len(results) != 2 {
		t.Fatalf("expected 2 hall-of-fame QSOs, got %d", len(results))
	}

	if results[0].Call != "K1ABC" || results[0].Name != "Alice" {
		t.Fatalf("expected K1ABC with preferred name, got %+v", results[0])
	}

	if results[1].Call != "Z9XYZ" {
		t.Fatalf("expected sorted Z9XYZ second, got %q", results[1].Call)
	}
}

func TestQSOFormattingHelpers(t *testing.T) {
	t.Parallel()

	qso := QSO{
		QSODate:   "20240102",
		TimeOn:    "030405",
		Country:   "Japan",
		Timestamp: time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC),
	}

	if got := qso.FormatQSOTime(); got != "2024-01-02 03:04:05 UTC" {
		t.Fatalf("expected formatted timestamp, got %q", got)
	}

	if got := qso.FormatDate(); got != "2024-01-02" {
		t.Fatalf("expected formatted date, got %q", got)
	}

	if got := qso.FormatTime(); got != "03:04" {
		t.Fatalf("expected formatted time, got %q", got)
	}

	if got := qso.GetFlagCode(); got != "jp" {
		t.Fatalf("expected flag code jp, got %q", got)
	}

	zero := QSO{QSODate: "20240102", TimeOn: "030405"}
	if got := zero.FormatQSOTime(); got != "20240102 030405 UTC" {
		t.Fatalf("expected fallback timestamp, got %q", got)
	}

	if got := zero.FormatDate(); got != "2024-01-02" {
		t.Fatalf("expected formatted date, got %q", got)
	}

	if got := zero.FormatTime(); got != "03:04" {
		t.Fatalf("expected formatted time, got %q", got)
	}

	short := QSO{QSODate: "202401", TimeOn: "03", Country: "Unknown"}
	if got := short.FormatDate(); got != "202401" {
		t.Fatalf("expected unmodified date, got %q", got)
	}

	if got := short.FormatTime(); got != "03" {
		t.Fatalf("expected unmodified time, got %q", got)
	}

	if got := short.GetFlagCode(); got != "" {
		t.Fatalf("expected empty flag code, got %q", got)
	}
}

func TestADIFParserGetLatestQSOsEmpty(t *testing.T) {
	t.Parallel()

	parser := NewADIFParser()

	latest := parser.GetLatestQSOs(5)
	if len(latest) != 0 {
		t.Fatalf("expected empty latest QSOs, got %d", len(latest))
	}
}

func TestADIFParserGetQSOs(t *testing.T) {
	t.Parallel()

	parser := NewADIFParser()
	parser.QSOs = []QSO{
		{Call: "A1"},
		{Call: "B2"},
	}

	qsos := parser.GetQSOs()
	if len(qsos) != 2 {
		t.Fatalf("expected 2 QSOs, got %d", len(qsos))
	}
}

type errorReader struct{}

func (errorReader) Read(_ []byte) (int, error) {
	return 0, errTestReadFailed
}
