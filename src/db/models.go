/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package db

import (
	"time"

	"github.com/google/uuid"
)

// Tier represents the contact tier (A-F)
type Tier string

const (
	TierA Tier = "A"
	TierB Tier = "B"
	TierC Tier = "C"
	TierD Tier = "D"
	TierE Tier = "E"
	TierF Tier = "F"
)

// Contact represents a person in the CRM
type Contact struct {
	ID             uuid.UUID  `db:"id"`
	NameGiven      *string    `db:"name_given"`
	NameAdditional *string    `db:"name_additional"`
	NameFamily     *string    `db:"name_family"`
	NameDisplay    string     `db:"name_display"`
	Nickname       *string    `db:"nickname"`
	Organization   *string    `db:"organization"`
	Title          *string    `db:"title"`
	Role           *string    `db:"role"`
	Birthday       *time.Time `db:"birthday"`
	Anniversary    *time.Time `db:"anniversary"`
	Gender         *string    `db:"gender"`
	Timezone       *string    `db:"timezone"`
	GeoLat         *float64   `db:"geo_lat"`
	GeoLon         *float64   `db:"geo_lon"`
	Language       *string    `db:"language"`
	PhotoURL       *string    `db:"photo_url"`
	Tier           Tier       `db:"tier"`
	CallSign       *string    `db:"call_sign"`
	CardDAVUUID    *string    `db:"carddav_uuid"`
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"`
}

// EmailType represents the type of email
type EmailType string

const (
	EmailPersonal EmailType = "personal"
	EmailWork     EmailType = "work"
	EmailOther    EmailType = "other"
)

// ContactEmail represents an email address for a contact
type ContactEmail struct {
	ID        uuid.UUID `db:"id"`
	ContactID uuid.UUID `db:"contact_id"`
	Email     string    `db:"email"`
	EmailType EmailType `db:"email_type"`
	IsPrimary bool      `db:"is_primary"`
	CreatedAt time.Time `db:"created_at"`
}

// PhoneType represents the type of phone number
type PhoneType string

const (
	PhoneCell  PhoneType = "cell"
	PhoneHome  PhoneType = "home"
	PhoneWork  PhoneType = "work"
	PhoneFax   PhoneType = "fax"
	PhonePager PhoneType = "pager"
	PhoneOther PhoneType = "other"
)

// ContactPhone represents a phone number for a contact
type ContactPhone struct {
	ID        uuid.UUID `db:"id"`
	ContactID uuid.UUID `db:"contact_id"`
	Phone     string    `db:"phone"`
	PhoneType PhoneType `db:"phone_type"`
	IsPrimary bool      `db:"is_primary"`
	CreatedAt time.Time `db:"created_at"`
}

// AddressType represents the type of address
type AddressType string

const (
	AddressHome  AddressType = "home"
	AddressWork  AddressType = "work"
	AddressOther AddressType = "other"
)

// ContactAddress represents a physical address for a contact
type ContactAddress struct {
	ID          uuid.UUID   `db:"id"`
	ContactID   uuid.UUID   `db:"contact_id"`
	Street      *string     `db:"street"`
	Locality    *string     `db:"locality"`
	Region      *string     `db:"region"`
	PostalCode  *string     `db:"postal_code"`
	Country     *string     `db:"country"`
	AddressType AddressType `db:"address_type"`
	IsPrimary   bool        `db:"is_primary"`
	POBox       *string     `db:"po_box"`
	Extended    *string     `db:"extended"`
	CreatedAt   time.Time   `db:"created_at"`
}

// URLType represents the type of URL/social media
type URLType string

const (
	URLWebsite   URLType = "website"
	URLBlog      URLType = "blog"
	URLTwitter   URLType = "twitter"
	URLMastodon  URLType = "mastodon"
	URLBluesky   URLType = "bluesky"
	URLThreads   URLType = "threads"
	URLFacebook  URLType = "facebook"
	URLInstagram URLType = "instagram"
	URLLinkedIn  URLType = "linkedin"
	URLGitHub    URLType = "github"
	URLGitLab    URLType = "gitlab"
	URLCodeberg  URLType = "codeberg"
	URLYouTube   URLType = "youtube"
	URLTwitch    URLType = "twitch"
	URLTikTok    URLType = "tiktok"
	URLSignal    URLType = "signal"
	URLTelegram  URLType = "telegram"
	URLWhatsApp  URLType = "whatsapp"
	URLMatrix    URLType = "matrix"
	URLQRZ       URLType = "qrz"
	URLOther     URLType = "other"
)

// ContactURL represents a URL or social media handle for a contact
type ContactURL struct {
	ID        uuid.UUID `db:"id"`
	ContactID uuid.UUID `db:"contact_id"`
	URL       string    `db:"url"`
	URLType   URLType   `db:"url_type"`
	Label     *string   `db:"label"`
	Username  *string   `db:"username"`
	CreatedAt time.Time `db:"created_at"`
}

// Tag represents a tag that can be applied to contacts
type Tag struct {
	ID          uuid.UUID `db:"id"`
	Name        string    `db:"name"`
	Description *string   `db:"description"`
	CreatedAt   time.Time `db:"created_at"`
}

// ContactTag represents the many-to-many relationship between contacts and tags
type ContactTag struct {
	ContactID uuid.UUID `db:"contact_id"`
	TagID     uuid.UUID `db:"tag_id"`
	CreatedAt time.Time `db:"created_at"`
}

// LogType represents the type of interaction log
type LogType string

const (
	LogGeneral       LogType = "general"
	LogEmailSent     LogType = "email_sent"
	LogEmailReceived LogType = "email_received"
	LogCall          LogType = "call"
	LogMeeting       LogType = "meeting"
	LogMessage       LogType = "message"
	LogGiftSent      LogType = "gift_sent"
	LogGiftReceived  LogType = "gift_received"
	LogIntro         LogType = "intro"
	LogOther         LogType = "other"
)

// ContactLog represents an interaction with a contact
type ContactLog struct {
	ID        uuid.UUID `db:"id"`
	ContactID uuid.UUID `db:"contact_id"`
	LogType   LogType   `db:"log_type"`
	LoggedAt  time.Time `db:"logged_at"`
	Subject   *string   `db:"subject"`
	Content   *string   `db:"content"`
	CreatedAt time.Time `db:"created_at"`
}

// ContactNote represents a timestamped note for a contact
type ContactNote struct {
	ID        uuid.UUID `db:"id"`
	ContactID uuid.UUID `db:"contact_id"`
	Content   string    `db:"content"`
	NotedAt   time.Time `db:"noted_at"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// QSL Status types
type QSLStatus string

const (
	QSLYes       QSLStatus = "Y"
	QSLNo        QSLStatus = "N"
	QSLRequested QSLStatus = "R"
	QSLIgnore    QSLStatus = "I"
	QSLVerified  QSLStatus = "V"
)

type QSLSentStatus string

const (
	QSLSentYes       QSLSentStatus = "Y"
	QSLSentNo        QSLSentStatus = "N"
	QSLSentRequested QSLSentStatus = "R"
	QSLSentQueued    QSLSentStatus = "Q"
	QSLSentIgnore    QSLSentStatus = "I"
)

type QSLVia string

const (
	QSLViaBureau     QSLVia = "B"
	QSLViaDirect     QSLVia = "D"
	QSLViaElectronic QSLVia = "E"
	QSLViaManager    QSLVia = "M"
)

type QSOUploadStatus string

const (
	QSOUploaded    QSOUploadStatus = "Y"
	QSONotUploaded QSOUploadStatus = "N"
	QSOModified    QSOUploadStatus = "M"
)

type QSOComplete string

const (
	QSOCompleteYes     QSOComplete = "Y"
	QSOCompleteNo      QSOComplete = "N"
	QSOCompleteNIL     QSOComplete = "NIL"
	QSOCompleteUnknown QSOComplete = "?"
)

type AntPath string

const (
	AntPathGrayline  AntPath = "G"
	AntPathOther     AntPath = "O"
	AntPathShortPath AntPath = "S"
	AntPathLongPath  AntPath = "L"
)

// QSO represents an amateur radio contact (ADIF 3.1.6 compatible)
type QSO struct {
	ID                      uuid.UUID        `db:"id"`
	ContactID               *uuid.UUID       `db:"contact_id"`
	Call                    string           `db:"call"`
	QSODate                 time.Time        `db:"qso_date"`
	TimeOn                  time.Time        `db:"time_on"`
	Band                    *string          `db:"band"`
	Freq                    *float64         `db:"freq"`
	Mode                    string           `db:"mode"`
	TimeOff                 *time.Time       `db:"time_off"`
	QSODateOff              *time.Time       `db:"qso_date_off"`
	BandRx                  *string          `db:"band_rx"`
	FreqRx                  *float64         `db:"freq_rx"`
	Submode                 *string          `db:"submode"`
	RSTSent                 *string          `db:"rst_sent"`
	RSTRcvd                 *string          `db:"rst_rcvd"`
	TxPwr                   *float64         `db:"tx_pwr"`
	RxPwr                   *float64         `db:"rx_pwr"`
	Name                    *string          `db:"name"`
	QTH                     *string          `db:"qth"`
	GridSquare              *string          `db:"gridsquare"`
	GridSquareExt           *string          `db:"gridsquare_ext"`
	VUCCGrids               *string          `db:"vucc_grids"`
	Lat                     *string          `db:"lat"`
	Lon                     *string          `db:"lon"`
	Altitude                *float64         `db:"altitude"`
	Country                 *string          `db:"country"`
	DXCC                    *int             `db:"dxcc"`
	CQZ                     *int             `db:"cqz"`
	ITUZ                    *int             `db:"ituz"`
	Cont                    *string          `db:"cont"`
	State                   *string          `db:"state"`
	Cnty                    *string          `db:"cnty"`
	CntyAlt                 *string          `db:"cnty_alt"`
	Pfx                     *string          `db:"pfx"`
	Age                     *int             `db:"age"`
	Email                   *string          `db:"email"`
	Web                     *string          `db:"web"`
	Address                 *string          `db:"address"`
	IOTA                    *string          `db:"iota"`
	IOTAIslandID            *string          `db:"iota_island_id"`
	SOTARef                 *string          `db:"sota_ref"`
	POTARef                 *string          `db:"pota_ref"`
	WWFFRef                 *string          `db:"wwff_ref"`
	Sig                     *string          `db:"sig"`
	SigInfo                 *string          `db:"sig_info"`
	DARCDOK                 *string          `db:"darc_dok"`
	FISTS                   *int             `db:"fists"`
	FISTSCC                 *int             `db:"fists_cc"`
	SKCC                    *string          `db:"skcc"`
	TenTen                  *int             `db:"ten_ten"`
	UKSMG                   *int             `db:"uksmg"`
	USACACounties           *string          `db:"usaca_counties"`
	ContactedOp             *string          `db:"contacted_op"`
	EQCall                  *string          `db:"eq_call"`
	GuestOp                 *string          `db:"guest_op"`
	SilentKey               *bool            `db:"silent_key"`
	StationCallsign         *string          `db:"station_callsign"`
	Operator                *string          `db:"operator"`
	OwnerCallsign           *string          `db:"owner_callsign"`
	MyName                  *string          `db:"my_name"`
	MyGridSquare            *string          `db:"my_gridsquare"`
	MyGridSquareExt         *string          `db:"my_gridsquare_ext"`
	MyVUCCGrids             *string          `db:"my_vucc_grids"`
	MyLat                   *string          `db:"my_lat"`
	MyLon                   *string          `db:"my_lon"`
	MyAltitude              *float64         `db:"my_altitude"`
	MyCity                  *string          `db:"my_city"`
	MyStreet                *string          `db:"my_street"`
	MyPostalCode            *string          `db:"my_postal_code"`
	MyState                 *string          `db:"my_state"`
	MyCnty                  *string          `db:"my_cnty"`
	MyCntyAlt               *string          `db:"my_cnty_alt"`
	MyCountry               *string          `db:"my_country"`
	MyDXCC                  *int             `db:"my_dxcc"`
	MyCQZone                *int             `db:"my_cq_zone"`
	MyITUZone               *int             `db:"my_itu_zone"`
	MyIOTA                  *string          `db:"my_iota"`
	MyIOTAIslandID          *string          `db:"my_iota_island_id"`
	MySOTARef               *string          `db:"my_sota_ref"`
	MyPOTARef               *string          `db:"my_pota_ref"`
	MyWWFFRef               *string          `db:"my_wwff_ref"`
	MySig                   *string          `db:"my_sig"`
	MySigInfo               *string          `db:"my_sig_info"`
	MyARRLSect              *string          `db:"my_arrl_sect"`
	MyDARCDOK               *string          `db:"my_darc_dok"`
	MyFISTS                 *int             `db:"my_fists"`
	MyUSACACounties         *string          `db:"my_usaca_counties"`
	MyRig                   *string          `db:"my_rig"`
	MyAntenna               *string          `db:"my_antenna"`
	Rig                     *string          `db:"rig"`
	MorseKeyType            *string          `db:"morse_key_type"`
	MorseKeyInfo            *string          `db:"morse_key_info"`
	MyMorseKeyType          *string          `db:"my_morse_key_type"`
	MyMorseKeyInfo          *string          `db:"my_morse_key_info"`
	PropMode                *string          `db:"prop_mode"`
	AntPath                 *AntPath         `db:"ant_path"`
	AntAz                   *float64         `db:"ant_az"`
	AntEl                   *float64         `db:"ant_el"`
	Distance                *int             `db:"distance"`
	AIndex                  *int             `db:"a_index"`
	KIndex                  *int             `db:"k_index"`
	SFI                     *int             `db:"sfi"`
	SatName                 *string          `db:"sat_name"`
	SatMode                 *string          `db:"sat_mode"`
	MSShower                *string          `db:"ms_shower"`
	MaxBursts               *int             `db:"max_bursts"`
	NrBursts                *int             `db:"nr_bursts"`
	NrPings                 *int             `db:"nr_pings"`
	ContestID               *string          `db:"contest_id"`
	SRX                     *int             `db:"srx"`
	SRXString               *string          `db:"srx_string"`
	STX                     *int             `db:"stx"`
	STXString               *string          `db:"stx_string"`
	Class                   *string          `db:"class"`
	ARRLSect                *string          `db:"arrl_sect"`
	CheckField              *string          `db:"check_field"`
	Precedence              *string          `db:"precedence"`
	Region                  *string          `db:"region"`
	QSOComplete             *QSOComplete     `db:"qso_complete"`
	QSORandom               *bool            `db:"qso_random"`
	ForceInit               *bool            `db:"force_init"`
	SWL                     *bool            `db:"swl"`
	QSLSent                 *QSLSentStatus   `db:"qsl_sent"`
	QSLSDate                *time.Time       `db:"qslsdate"`
	QSLSentVia              *QSLVia          `db:"qsl_sent_via"`
	QSLRcvd                 *QSLStatus       `db:"qsl_rcvd"`
	QSLRDate                *time.Time       `db:"qslrdate"`
	QSLRcvdVia              *QSLVia          `db:"qsl_rcvd_via"`
	QSLVia                  *string          `db:"qsl_via"`
	QSLMsg                  *string          `db:"qslmsg"`
	QSLMsgRcvd              *string          `db:"qslmsg_rcvd"`
	LoTWQSLSent             *QSLSentStatus   `db:"lotw_qsl_sent"`
	LoTWQSLSDate            *time.Time       `db:"lotw_qslsdate"`
	LoTWQSLRcvd             *QSLStatus       `db:"lotw_qsl_rcvd"`
	LoTWQSLRDate            *time.Time       `db:"lotw_qslrdate"`
	EQSLQSLSent             *QSLSentStatus   `db:"eqsl_qsl_sent"`
	EQSLQSLSDate            *time.Time       `db:"eqsl_qslsdate"`
	EQSLQSLRcvd             *QSLStatus       `db:"eqsl_qsl_rcvd"`
	EQSLQSLRDate            *time.Time       `db:"eqsl_qslrdate"`
	EQSLAG                  *bool            `db:"eqsl_ag"`
	ClublogQSOUploadDate    *time.Time       `db:"clublog_qso_upload_date"`
	ClublogQSOUploadStatus  *QSOUploadStatus `db:"clublog_qso_upload_status"`
	QRZComQSOUploadDate     *time.Time       `db:"qrzcom_qso_upload_date"`
	QRZComQSOUploadStatus   *QSOUploadStatus `db:"qrzcom_qso_upload_status"`
	QRZComQSODownloadDate   *time.Time       `db:"qrzcom_qso_download_date"`
	QRZComQSODownloadStatus *QSOUploadStatus `db:"qrzcom_qso_download_status"`
	HRDLogQSOUploadDate     *time.Time       `db:"hrdlog_qso_upload_date"`
	HRDLogQSOUploadStatus   *QSOUploadStatus `db:"hrdlog_qso_upload_status"`
	HamLogEUQSOUploadDate   *time.Time       `db:"hamlogeu_qso_upload_date"`
	HamLogEUQSOUploadStatus *QSOUploadStatus `db:"hamlogeu_qso_upload_status"`
	HamQTHQSOUploadDate     *time.Time       `db:"hamqth_qso_upload_date"`
	HamQTHQSOUploadStatus   *QSOUploadStatus `db:"hamqth_qso_upload_status"`
	DCLQSLSent              *QSLSentStatus   `db:"dcl_qsl_sent"`
	DCLQSLSDate             *time.Time       `db:"dcl_qslsdate"`
	DCLQSLRcvd              *QSLStatus       `db:"dcl_qsl_rcvd"`
	DCLQSLRDate             *time.Time       `db:"dcl_qslrdate"`
	AwardSubmitted          *string          `db:"award_submitted"`
	AwardGranted            *string          `db:"award_granted"`
	CreditSubmitted         *string          `db:"credit_submitted"`
	CreditGranted           *string          `db:"credit_granted"`
	Comment                 *string          `db:"comment"`
	Notes                   *string          `db:"notes"`
	PublicKey               *string          `db:"public_key"`
	AppFields               map[string]any   `db:"app_fields"`
	UserFields              map[string]any   `db:"user_fields"`
	CreatedAt               time.Time        `db:"created_at"`
	UpdatedAt               time.Time        `db:"updated_at"`
}

// FormatDate formats QSO date as YYYY-MM-DD
func (q *QSO) FormatDate() string {
	return q.QSODate.Format("2006-01-02")
}

// FormatTime formats QSO time as HH:MM (UTC)
func (q *QSO) FormatTime() string {
	return q.TimeOn.UTC().Format("15:04")
}

// FormatQSOTime formats QSO timestamp for display in email links etc.
func (q *QSO) FormatQSOTime() string {
	return q.TimeOn.UTC().Format("2006-01-02 15:04:05 UTC")
}

// QSL status helper methods for templates

// IsPaperQSLSent returns true if paper QSL was sent
func (q *QSO) IsPaperQSLSent() bool {
	return q.QSLSent != nil && *q.QSLSent == QSLSentYes
}

// IsPaperQSLReceived returns true if paper QSL was received
func (q *QSO) IsPaperQSLReceived() bool {
	return q.QSLRcvd != nil && *q.QSLRcvd == QSLYes
}

// IsLoTWQSLSent returns true if LoTW QSL was sent
func (q *QSO) IsLoTWQSLSent() bool {
	return q.LoTWQSLSent != nil && *q.LoTWQSLSent == QSLSentYes
}

// IsLoTWQSLReceived returns true if LoTW QSL was received
func (q *QSO) IsLoTWQSLReceived() bool {
	return q.LoTWQSLRcvd != nil && *q.LoTWQSLRcvd == QSLYes
}

// IsEQSLQSLSent returns true if eQSL was sent
func (q *QSO) IsEQSLQSLSent() bool {
	return q.EQSLQSLSent != nil && *q.EQSLQSLSent == QSLSentYes
}

// IsEQSLQSLReceived returns true if eQSL was received
func (q *QSO) IsEQSLQSLReceived() bool {
	return q.EQSLQSLRcvd != nil && *q.EQSLQSLRcvd == QSLYes
}

// HasContact returns true if this QSO is linked to a contact
func (q *QSO) HasContact() bool {
	return q.ContactID != nil
}

// GetContactID returns the contact ID as a string for templates
func (q *QSO) GetContactID() string {
	if q.ContactID == nil {
		return ""
	}
	return q.ContactID.String()
}

// ContactView represents the aggregated contact view with primary email/phone
type ContactView struct {
	Contact
	PrimaryEmail   *string    `db:"primary_email"`
	PrimaryPhone   *string    `db:"primary_phone"`
	Tags           []string   `db:"tags"`
	LastCRMContact *time.Time `db:"last_crm_contact"`
	LastQSO        *time.Time `db:"last_qso"`
}

// ContactDue represents a contact that is due for follow-up
type ContactDue struct {
	ContactView
	LastContact     *time.Time `db:"last_contact"`
	ContactInterval *string    `db:"contact_interval"`
	IsDue           bool       `db:"is_due"`
}

// ZettelComment represents a temporary comment on a zettelkasten note
type ZettelComment struct {
	ID        uuid.UUID `db:"id"`
	ZettelID  string    `db:"zettel_id"` // org-mode ID (not UUID foreign key)
	Content   string    `db:"content"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// ZettelCommentWithNote represents a comment with its associated zettel metadata
// Used for the inbox view
type ZettelCommentWithNote struct {
	ZettelComment
	ZettelTitle    string `db:"zettel_title"`    // Fetched from WebDAV
	ZettelFilename string `db:"zettel_filename"` // For debugging
	OrphanedNote   bool   `db:"orphaned_note"`   // True if zettel file not found
}
