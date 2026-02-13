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

// Tier values represent supported contact tiers.
const (
	TierA Tier = "A"
	TierB Tier = "B"
	TierC Tier = "C"
	TierD Tier = "D"
	TierE Tier = "E"
	TierF Tier = "F"
)

// Gender represents biological sex for medical reference ranges
type Gender string

// Gender values represent supported biological-sex categories.
const (
	GenderMale   Gender = "Male"
	GenderFemale Gender = "Female"
	GenderUnisex Gender = "Unisex" // For ranges that don't vary by gender
)

// AgeRange represents age-based categorization for reference ranges
type AgeRange string

// AgeRange values represent supported age groups for lab ranges.
const (
	AgePediatric AgeRange = "Pediatric" // 0-17
	AgeAdult     AgeRange = "Adult"     // 18-49
	AgeMiddleAge AgeRange = "MiddleAge" // 50-64
	AgeSenior    AgeRange = "Senior"    // 65+
)

// User represents an authenticated account.
type User struct {
	ID          uuid.UUID `db:"id"`
	DisplayName string    `db:"display_name"`
	IsAdmin     bool      `db:"is_admin"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

// UserPasskey represents a stored WebAuthn credential.
type UserPasskey struct {
	ID             uuid.UUID  `db:"id"`
	UserID         uuid.UUID  `db:"user_id"`
	CredentialID   []byte     `db:"credential_id"`
	CredentialData []byte     `db:"credential_data"`
	Label          *string    `db:"label"`
	CreatedAt      time.Time  `db:"created_at"`
	LastUsedAt     *time.Time `db:"last_used_at"`
}

// Contact represents a person in the CRM
type Contact struct {
	ID              uuid.UUID  `db:"id"`
	NameGiven       *string    `db:"name_given"`
	NameAdditional  *string    `db:"name_additional"`
	NameFamily      *string    `db:"name_family"`
	NameDisplay     string     `db:"name_display"`
	Nickname        *string    `db:"nickname"`
	Organization    *string    `db:"organization"`
	Title           *string    `db:"title"`
	Role            *string    `db:"role"`
	Birthday        *time.Time `db:"birthday"`
	Anniversary     *time.Time `db:"anniversary"`
	Gender          *string    `db:"gender"`
	Timezone        *string    `db:"timezone"`
	GeoLat          *float64   `db:"geo_lat"`
	GeoLon          *float64   `db:"geo_lon"`
	Language        *string    `db:"language"`
	PhotoURL        *string    `db:"photo_url"`
	Tier            Tier       `db:"tier"`
	CallSign        *string    `db:"call_sign"`
	IsService       bool       `db:"is_service"`
	CardDAVUUID     *string    `db:"carddav_uuid"`
	CreatedAt       time.Time  `db:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at"`
	LastAutoContact *time.Time `db:"last_auto_contact"` // Auto-updated by WhatsApp message tracking
}

// EmailType represents the type of email
type EmailType string

// EmailType values represent supported email categories.
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
	Source    string    `db:"source"`
	CreatedAt time.Time `db:"created_at"`
}

// PhoneType represents the type of phone number
type PhoneType string

// PhoneType values represent supported phone categories.
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
	Source    string    `db:"source"`
	CreatedAt time.Time `db:"created_at"`
}

// AddressType represents the type of address
type AddressType string

// AddressType values represent supported address categories.
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

// URLType values represent supported external profile/link categories.
const (
	URLWebsite       URLType = "website"
	URLBlog          URLType = "blog"
	URLTwitter       URLType = "twitter"
	URLMastodon      URLType = "mastodon"
	URLBluesky       URLType = "bluesky"
	URLThreads       URLType = "threads"
	URLFacebook      URLType = "facebook"
	URLInstagram     URLType = "instagram"
	URLLinkedIn      URLType = "linkedin"
	URLOrcid         URLType = "orcid"
	URLGoogleScholar URLType = "google_scholar"
	URLGitHub        URLType = "github"
	URLGitLab        URLType = "gitlab"
	URLCodeberg      URLType = "codeberg"
	URLYouTube       URLType = "youtube"
	URLTwitch        URLType = "twitch"
	URLTikTok        URLType = "tiktok"
	URLSignal        URLType = "signal"
	URLTelegram      URLType = "telegram"
	URLWhatsApp      URLType = "whatsapp"
	URLMatrix        URLType = "matrix"
	URLQRZ           URLType = "qrz"
	URLOther         URLType = "other"
)

// ContactURL represents a URL or social media handle for a contact
type ContactURL struct {
	ID          uuid.UUID `db:"id"`
	ContactID   uuid.UUID `db:"contact_id"`
	URL         string    `db:"url"`
	URLType     URLType   `db:"url_type"`
	Description *string   `db:"description"`
	CreatedAt   time.Time `db:"created_at"`
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

// LogType values represent supported interaction log categories.
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

// ChatPlatform represents the chat platform/source
type ChatPlatform string

// ChatPlatform values represent supported chat sources.
const (
	ChatPlatformManual   ChatPlatform = "manual"
	ChatPlatformEmail    ChatPlatform = "email"
	ChatPlatformWhatsApp ChatPlatform = "whatsapp"
	ChatPlatformSignal   ChatPlatform = "signal"
	ChatPlatformWeChat   ChatPlatform = "wechat"
	ChatPlatformTeams    ChatPlatform = "teams"
	ChatPlatformSlack    ChatPlatform = "slack"
	ChatPlatformOther    ChatPlatform = "other"
)

// ChatSender represents the sender for a chat entry
type ChatSender string

// ChatSender values represent who authored a chat entry.
const (
	ChatSenderMe   ChatSender = "me"
	ChatSenderThem ChatSender = "them"
	ChatSenderMix  ChatSender = "mix"
)

// ContactChat represents a chat message with a contact
type ContactChat struct {
	ID        uuid.UUID    `db:"id"`
	ContactID uuid.UUID    `db:"contact_id"`
	Platform  ChatPlatform `db:"platform"`
	Sender    ChatSender   `db:"sender"`
	Message   string       `db:"message"`
	SentAt    time.Time    `db:"sent_at"`
	CreatedAt time.Time    `db:"created_at"`
}

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

// ContactLogTimelineEntry represents a contact log entry for the timeline feed.
type ContactLogTimelineEntry struct {
	ID          uuid.UUID `db:"id"`
	ContactID   uuid.UUID `db:"contact_id"`
	ContactName string    `db:"name_display"`
	LogType     LogType   `db:"log_type"`
	LoggedAt    time.Time `db:"logged_at"`
	Subject     *string   `db:"subject"`
	Content     *string   `db:"content"`
	CreatedAt   time.Time `db:"created_at"`
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

// QSLStatus represents received QSL confirmation status.
type QSLStatus string

// QSLStatus values represent ADIF QSL_RCVD statuses.
const (
	QSLYes       QSLStatus = "Y"
	QSLNo        QSLStatus = "N"
	QSLRequested QSLStatus = "R"
	QSLIgnore    QSLStatus = "I"
	QSLVerified  QSLStatus = "V"
)

// QSLSentStatus represents sent QSL confirmation status.
type QSLSentStatus string

// QSLSentStatus values represent ADIF QSL_SENT statuses.
const (
	QSLSentYes       QSLSentStatus = "Y"
	QSLSentNo        QSLSentStatus = "N"
	QSLSentRequested QSLSentStatus = "R"
	QSLSentQueued    QSLSentStatus = "Q"
	QSLSentIgnore    QSLSentStatus = "I"
)

// QSLVia represents the QSL route used for confirmation.
type QSLVia string

// QSLVia values represent ADIF QSL_VIA codes.
const (
	QSLViaBureau     QSLVia = "B"
	QSLViaDirect     QSLVia = "D"
	QSLViaElectronic QSLVia = "E"
	QSLViaManager    QSLVia = "M"
)

// QSOUploadStatus represents upload state to external QSO services.
type QSOUploadStatus string

// QSOUploadStatus values represent ADIF upload flags.
const (
	QSOUploaded    QSOUploadStatus = "Y"
	QSONotUploaded QSOUploadStatus = "N"
	QSOModified    QSOUploadStatus = "M"
)

// QSOComplete represents completion state for a QSO record.
type QSOComplete string

// QSOComplete values represent ADIF completion statuses.
const (
	QSOCompleteYes     QSOComplete = "Y"
	QSOCompleteNo      QSOComplete = "N"
	QSOCompleteNIL     QSOComplete = "NIL"
	QSOCompleteUnknown QSOComplete = "?"
)

// AntPath represents signal path classification for a QSO.
type AntPath string

// AntPath values represent ADIF antenna path values.
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
	Distance                *float64         `db:"distance"`
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

// IsPaperQSLQueued returns true if paper QSL is queued to be sent.
func (q *QSO) IsPaperQSLQueued() bool {
	return q.QSLSent != nil && *q.QSLSent == QSLSentQueued
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

// JournalDayLocation represents a location attached to a daily journal entry.
type JournalDayLocation struct {
	ID          uuid.UUID `db:"id"`
	Day         time.Time `db:"day"`
	LocationLat float64   `db:"location_lat"`
	LocationLon float64   `db:"location_lon"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
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

// InventoryStatus represents the status of an inventory item
type InventoryStatus string

// InventoryStatus values represent supported inventory lifecycle states.
const (
	InventoryStatusActive              InventoryStatus = "active"
	InventoryStatusStored              InventoryStatus = "stored"
	InventoryStatusDamaged             InventoryStatus = "damaged"
	InventoryStatusMaintenanceRequired InventoryStatus = "maintenance_required"
	InventoryStatusGiven               InventoryStatus = "given"
	InventoryStatusDisposed            InventoryStatus = "disposed"
	InventoryStatusLost                InventoryStatus = "lost"
)

// InventoryStatusLabel returns a human-readable label for an inventory status.
func InventoryStatusLabel(status InventoryStatus) string {
	switch status {
	case InventoryStatusActive:
		return "Active"
	case InventoryStatusStored:
		return "Stored"
	case InventoryStatusDamaged:
		return "Damaged"
	case InventoryStatusMaintenanceRequired:
		return "Maintenance Required"
	case InventoryStatusGiven:
		return "Given"
	case InventoryStatusDisposed:
		return "Disposed"
	case InventoryStatusLost:
		return "Lost"
	default:
		return string(status)
	}
}

// InventoryItem represents an item in the inventory system
type InventoryItem struct {
	ID             int             `db:"id"`           // Numeric ID for DB relationships
	InventoryID    string          `db:"inventory_id"` // Formatted ID (GW-00001)
	Name           string          `db:"name"`
	Location       *string         `db:"location"`
	Description    *string         `db:"description"`
	Status         InventoryStatus `db:"status"`
	InspectionDate *time.Time      `db:"inspection_date"`
	InspectionDue  bool            `db:"inspection_due"`
	CreatedAt      time.Time       `db:"created_at"`
	UpdatedAt      time.Time       `db:"updated_at"`
}

// InventoryComment represents a comment on an inventory item
type InventoryComment struct {
	ID        uuid.UUID `db:"id"`
	ItemID    int       `db:"item_id"`
	Content   string    `db:"content"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// LedgerAccountType represents the type of financial account.
type LedgerAccountType string

// LedgerAccountType values represent supported ledger account kinds.
const (
	LedgerAccountRegular  LedgerAccountType = "regular"
	LedgerAccountDebt     LedgerAccountType = "debt"
	LedgerAccountTracking LedgerAccountType = "tracking"
)

// LedgerTransactionStatus represents the status of a ledger transaction.
type LedgerTransactionStatus string

// LedgerTransactionStatus values represent supported transaction states.
const (
	LedgerTransactionPending  LedgerTransactionStatus = "pending"
	LedgerTransactionCleared  LedgerTransactionStatus = "cleared"
	LedgerTransactionRefunded LedgerTransactionStatus = "refunded"
	LedgerTransactionRejected LedgerTransactionStatus = "rejected"
)

// LedgerBudget represents a monthly budget category.
type LedgerBudget struct {
	ID           uuid.UUID `db:"id"`
	CategoryName string    `db:"category_name"`
	Amount       float64   `db:"amount"`
	Currency     string    `db:"currency"`
	PeriodStart  time.Time `db:"period_start"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

// LedgerAccount represents a ledger account.
type LedgerAccount struct {
	ID             uuid.UUID         `db:"id"`
	Name           string            `db:"name"`
	AccountType    LedgerAccountType `db:"account_type"`
	OpeningBalance float64           `db:"opening_balance"`
	IBAN           *string           `db:"iban"`
	BankName       *string           `db:"bank_name"`
	AccountNumber  *string           `db:"account_number"`
	Description    *string           `db:"description"`
	CreatedAt      time.Time         `db:"created_at"`
	UpdatedAt      time.Time         `db:"updated_at"`
}

// LedgerTransaction represents a transaction in an account.
type LedgerTransaction struct {
	ID         uuid.UUID               `db:"id"`
	AccountID  uuid.UUID               `db:"account_id"`
	BudgetID   *uuid.UUID              `db:"budget_id"`
	Amount     float64                 `db:"amount"`
	Merchant   string                  `db:"merchant"`
	Status     LedgerTransactionStatus `db:"status"`
	OccurredAt time.Time               `db:"occurred_at"`
	Note       *string                 `db:"note"`
	CreatedAt  time.Time               `db:"created_at"`
	UpdatedAt  time.Time               `db:"updated_at"`
}

// LedgerReconciliation represents a reconciled balance snapshot.
type LedgerReconciliation struct {
	ID           uuid.UUID `db:"id"`
	AccountID    uuid.UUID `db:"account_id"`
	Balance      float64   `db:"balance"`
	ReconciledAt time.Time `db:"reconciled_at"`
	Note         *string   `db:"note"`
	CreatedAt    time.Time `db:"created_at"`
}

// LabTestCategory represents the category of a lab test
type LabTestCategory string

// LabTestCategory values represent supported lab test buckets.
const (
	CategoryBloodCounts      LabTestCategory = "Blood Counts"
	CategoryLipidPanel       LabTestCategory = "Lipid Panel"
	CategoryMetabolic        LabTestCategory = "Metabolic"
	CategoryLiverFunction    LabTestCategory = "Liver Function"
	CategoryVitaminsMinerals LabTestCategory = "Vitamins & Minerals"
	CategoryEndocrineOther   LabTestCategory = "Endocrine & Other"
)

// LabTest represents a predefined medical lab test
type LabTest struct {
	Name     string
	Unit     string
	Category LabTestCategory
}

// HealthProfile represents a person being tracked in the health system
type HealthProfile struct {
	ID          uuid.UUID  `db:"id"`
	Name        string     `db:"name"`
	DateOfBirth *time.Time `db:"date_of_birth"`
	Gender      *Gender    `db:"gender"`
	Description *string    `db:"description"`
	IsPrimary   bool       `db:"is_primary"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
}

// GetAge calculates the age in years at a given date
func (h *HealthProfile) GetAge(atDate time.Time) *int {
	if h.DateOfBirth == nil {
		return nil
	}

	years := atDate.Year() - h.DateOfBirth.Year()
	// Adjust if birthday hasn't occurred yet this year
	if atDate.Month() < h.DateOfBirth.Month() ||
		(atDate.Month() == h.DateOfBirth.Month() && atDate.Day() < h.DateOfBirth.Day()) {
		years--
	}

	return &years
}

// GetAgeRange returns the age range category for reference ranges
func (h *HealthProfile) GetAgeRange(atDate time.Time) AgeRange {
	age := h.GetAge(atDate)
	if age == nil {
		// Default to adult if no DOB
		return AgeAdult
	}

	switch {
	case *age <= 17:
		return AgePediatric
	case *age <= 49:
		return AgeAdult
	case *age <= 64:
		return AgeMiddleAge
	default:
		return AgeSenior
	}
}

// HealthProfileSummary represents a profile with follow-up statistics
type HealthProfileSummary struct {
	HealthProfile
	FollowupCount    int        `db:"followup_count"`
	LastFollowupDate *time.Time `db:"last_followup_date"`
}

// HealthFollowup represents a medical visit or report
type HealthFollowup struct {
	ID           uuid.UUID `db:"id"`
	ProfileID    uuid.UUID `db:"profile_id"`
	FollowupDate time.Time `db:"followup_date"`
	HospitalName string    `db:"hospital_name"`
	Notes        *string   `db:"notes"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

// HealthFollowupSummary represents a follow-up with result statistics
type HealthFollowupSummary struct {
	HealthFollowup
	ResultCount int `db:"result_count"`
}

// HealthLabResult represents an individual lab test result
type HealthLabResult struct {
	ID         uuid.UUID `db:"id"`
	FollowupID uuid.UUID `db:"followup_id"`
	TestName   string    `db:"test_name"`
	TestUnit   *string   `db:"test_unit"`
	TestValue  float64   `db:"test_value"`
	CreatedAt  time.Time `db:"created_at"`
}

// ReferenceRange represents reference and optimal ranges for a lab test
type ReferenceRange struct {
	ID           uuid.UUID `db:"id"`
	TestName     string    `db:"test_name"`
	AgeRange     AgeRange  `db:"age_range"`
	Gender       Gender    `db:"gender"`
	ReferenceMin *float64  `db:"reference_min"`
	ReferenceMax *float64  `db:"reference_max"`
	OptimalMin   *float64  `db:"optimal_min"`
	OptimalMax   *float64  `db:"optimal_max"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

// GetDisplayRange returns the range to display based on the logic:
// - If both optimal min/max missing: use reference only
// - If only optimal max set: optimal min = reference min
// - If only optimal min set: optimal max = reference max
func (r *ReferenceRange) GetDisplayRange() (refMin, refMax, optMin, optMax *float64, hasOptimal bool) {
	refMin = r.ReferenceMin
	refMax = r.ReferenceMax

	// If both optimal values are nil, no optimal range
	if r.OptimalMin == nil && r.OptimalMax == nil {
		return refMin, refMax, nil, nil, false
	}

	// Fill in missing optimal boundaries from reference
	optMin = r.OptimalMin
	if optMin == nil {
		optMin = r.ReferenceMin
	}

	optMax = r.OptimalMax
	if optMax == nil {
		optMax = r.ReferenceMax
	}

	return refMin, refMax, optMin, optMax, true
}

// GetPredefinedLabTests returns all predefined lab tests organized by category
func GetPredefinedLabTests() []LabTest {
	return []LabTest{
		// Blood Counts (15 tests)
		{Name: "White blood cells", Unit: "×10³/μL", Category: CategoryBloodCounts},
		{Name: "Red blood cells", Unit: "×10⁶/μL", Category: CategoryBloodCounts},
		{Name: "Hemoglobin", Unit: "g/dL", Category: CategoryBloodCounts},
		{Name: "HCT", Unit: "%", Category: CategoryBloodCounts},
		{Name: "M.C.V", Unit: "fL", Category: CategoryBloodCounts},
		{Name: "RDW - CV", Unit: "%", Category: CategoryBloodCounts},
		{Name: "Platelets", Unit: "×10³/μL", Category: CategoryBloodCounts},
		{Name: "M.C.H", Unit: "pg", Category: CategoryBloodCounts},
		{Name: "M.C.H.C", Unit: "g/dL", Category: CategoryBloodCounts},
		{Name: "M.P.V", Unit: "fL", Category: CategoryBloodCounts},
		{Name: "Neutrophils", Unit: "%", Category: CategoryBloodCounts},
		{Name: "Lymphocytes", Unit: "%", Category: CategoryBloodCounts},
		{Name: "Monocytes", Unit: "%", Category: CategoryBloodCounts},
		{Name: "Eosinophils", Unit: "%", Category: CategoryBloodCounts},
		{Name: "Basophils", Unit: "%", Category: CategoryBloodCounts},

		// Lipid Panel (6 tests)
		{Name: "Total Cholesterol", Unit: "mg/dL", Category: CategoryLipidPanel},
		{Name: "LDL Cholesterol", Unit: "mg/dL", Category: CategoryLipidPanel},
		{Name: "HDL Cholesterol", Unit: "mg/dL", Category: CategoryLipidPanel},
		{Name: "Triglycerides", Unit: "mg/dL", Category: CategoryLipidPanel},
		{Name: "Non-HDL Cholesterol", Unit: "mg/dL", Category: CategoryLipidPanel},
		{Name: "Apolipoprotein B", Unit: "mg/dL", Category: CategoryLipidPanel},

		// Metabolic (8 tests)
		{Name: "Glucose fasting FBS", Unit: "mg/dL", Category: CategoryMetabolic},
		{Name: "Creatinine", Unit: "mg/dL", Category: CategoryMetabolic},
		{Name: "Calcium", Unit: "mmol/L", Category: CategoryMetabolic},
		{Name: "Uric Acid", Unit: "umol/L", Category: CategoryMetabolic},
		{Name: "Bicarbonate", Unit: "mmol/L", Category: CategoryMetabolic},
		{Name: "Sodium", Unit: "mmol/L", Category: CategoryMetabolic},
		{Name: "Potassium", Unit: "mmol/L", Category: CategoryMetabolic},
		{Name: "Chloride", Unit: "mmol/L", Category: CategoryMetabolic},

		// Liver Function (10 tests)
		{Name: "SGPT (ALT), Serum", Unit: "IU/L", Category: CategoryLiverFunction},
		{Name: "SGOT (AST)", Unit: "IU/L", Category: CategoryLiverFunction},
		{Name: "GGT", Unit: "IU/L", Category: CategoryLiverFunction},
		{Name: "Bilirubin Total", Unit: "mg/dL", Category: CategoryLiverFunction},
		{Name: "Bilirubin Indirect", Unit: "mg/dL", Category: CategoryLiverFunction},
		{Name: "Bilirubin Direct", Unit: "mg/dL", Category: CategoryLiverFunction},
		{Name: "Alkaline Phosphatase (ALP)", Unit: "IU/L", Category: CategoryLiverFunction},
		{Name: "Albumin", Unit: "g/dL", Category: CategoryLiverFunction},
		{Name: "Globulin", Unit: "g/dL", Category: CategoryLiverFunction},
		{Name: "Total Protein", Unit: "g/dL", Category: CategoryLiverFunction},

		// Vitamins & Minerals (6 tests)
		{Name: "Vitamin D", Unit: "nmol/L", Category: CategoryVitaminsMinerals},
		{Name: "Vitamin B12", Unit: "pmol/L", Category: CategoryVitaminsMinerals},
		{Name: "Magnesium, Serum", Unit: "mmol/L", Category: CategoryVitaminsMinerals},
		{Name: "Iron, Serum", Unit: "umol/L", Category: CategoryVitaminsMinerals},
		{Name: "Ferritin", Unit: "ng/mL", Category: CategoryVitaminsMinerals},
		{Name: "Zinc", Unit: "umol/L", Category: CategoryVitaminsMinerals},

		// Endocrine & Other (3 tests)
		{Name: "TSH", Unit: "uIU/mL", Category: CategoryEndocrineOther},
		{Name: "Haemoglobin HbA1c", Unit: "%", Category: CategoryEndocrineOther},
		{Name: "ESR", Unit: "mm/h", Category: CategoryEndocrineOther},
	}
}
