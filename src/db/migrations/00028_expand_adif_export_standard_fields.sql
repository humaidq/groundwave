-- +goose Up

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION qso_to_adif(qso_id UUID)
RETURNS TEXT AS $$
DECLARE
    q qsos%ROWTYPE;
    adif TEXT := '';
BEGIN
    SELECT * INTO q FROM qsos WHERE id = qso_id;
    IF NOT FOUND THEN RETURN NULL; END IF;

    -- Required fields
    adif := adif || '<CALL:' || length(q.call) || '>' || q.call;
    adif := adif || '<QSO_DATE:8>' || to_char(q.qso_date, 'YYYYMMDD');
    adif := adif || '<TIME_ON:6>' || to_char(q.time_on, 'HH24MISS');
    adif := adif || '<MODE:' || length(q.mode) || '>' || q.mode;

    -- QSO timing
    IF q.qso_date_off IS NOT NULL THEN
        adif := adif || '<QSO_DATE_OFF:8>' || to_char(q.qso_date_off, 'YYYYMMDD');
    END IF;
    IF q.time_off IS NOT NULL THEN
        adif := adif || '<TIME_OFF:6>' || to_char(q.time_off, 'HH24MISS');
    END IF;

    -- Frequency and mode
    IF q.band IS NOT NULL THEN
        adif := adif || '<BAND:' || length(q.band) || '>' || q.band;
    END IF;
    IF q.freq IS NOT NULL THEN
        adif := adif || '<FREQ:' || length(q.freq::text) || '>' || q.freq;
    END IF;
    IF q.band_rx IS NOT NULL THEN
        adif := adif || '<BAND_RX:' || length(q.band_rx) || '>' || q.band_rx;
    END IF;
    IF q.freq_rx IS NOT NULL THEN
        adif := adif || '<FREQ_RX:' || length(q.freq_rx::text) || '>' || q.freq_rx;
    END IF;
    IF q.submode IS NOT NULL THEN
        adif := adif || '<SUBMODE:' || length(q.submode) || '>' || q.submode;
    END IF;

    -- Reports and power
    IF q.rst_sent IS NOT NULL THEN
        adif := adif || '<RST_SENT:' || length(q.rst_sent) || '>' || q.rst_sent;
    END IF;
    IF q.rst_rcvd IS NOT NULL THEN
        adif := adif || '<RST_RCVD:' || length(q.rst_rcvd) || '>' || q.rst_rcvd;
    END IF;
    IF q.tx_pwr IS NOT NULL THEN
        adif := adif || '<TX_PWR:' || length(q.tx_pwr::text) || '>' || q.tx_pwr;
    END IF;

    -- Contact station
    IF q.name IS NOT NULL THEN
        adif := adif || '<NAME:' || length(q.name) || '>' || q.name;
    END IF;
    IF q.qth IS NOT NULL THEN
        adif := adif || '<QTH:' || length(q.qth) || '>' || q.qth;
    END IF;
    IF q.gridsquare IS NOT NULL THEN
        adif := adif || '<GRIDSQUARE:' || length(q.gridsquare) || '>' || q.gridsquare;
    END IF;

    -- Geographic/entity fields
    IF q.country IS NOT NULL THEN
        adif := adif || '<COUNTRY:' || length(q.country) || '>' || q.country;
    END IF;
    IF q.dxcc IS NOT NULL THEN
        adif := adif || '<DXCC:' || length(q.dxcc::text) || '>' || q.dxcc;
    END IF;
    IF q.cqz IS NOT NULL THEN
        adif := adif || '<CQZ:' || length(q.cqz::text) || '>' || q.cqz;
    END IF;
    IF q.ituz IS NOT NULL THEN
        adif := adif || '<ITUZ:' || length(q.ituz::text) || '>' || q.ituz;
    END IF;
    IF q.cont IS NOT NULL THEN
        adif := adif || '<CONT:' || length(q.cont) || '>' || q.cont;
    END IF;
    IF q.state IS NOT NULL THEN
        adif := adif || '<STATE:' || length(q.state) || '>' || q.state;
    END IF;
    IF q.cnty IS NOT NULL THEN
        adif := adif || '<CNTY:' || length(q.cnty) || '>' || q.cnty;
    END IF;
    IF q.pfx IS NOT NULL THEN
        adif := adif || '<PFX:' || length(q.pfx) || '>' || q.pfx;
    END IF;

    -- Program references
    IF q.iota IS NOT NULL THEN
        adif := adif || '<IOTA:' || length(q.iota) || '>' || q.iota;
    END IF;
    IF q.sota_ref IS NOT NULL THEN
        adif := adif || '<SOTA_REF:' || length(q.sota_ref) || '>' || q.sota_ref;
    END IF;
    IF q.pota_ref IS NOT NULL THEN
        adif := adif || '<POTA_REF:' || length(q.pota_ref) || '>' || q.pota_ref;
    END IF;

    -- Propagation metrics
    IF q.distance IS NOT NULL THEN
        adif := adif || '<DISTANCE:' || length(q.distance::text) || '>' || q.distance;
    END IF;
    IF q.a_index IS NOT NULL THEN
        adif := adif || '<A_INDEX:' || length(q.a_index::text) || '>' || q.a_index;
    END IF;
    IF q.k_index IS NOT NULL THEN
        adif := adif || '<K_INDEX:' || length(q.k_index::text) || '>' || q.k_index;
    END IF;
    IF q.sfi IS NOT NULL THEN
        adif := adif || '<SFI:' || length(q.sfi::text) || '>' || q.sfi;
    END IF;

    -- My station
    IF q.station_callsign IS NOT NULL THEN
        adif := adif || '<STATION_CALLSIGN:' || length(q.station_callsign) || '>' || q.station_callsign;
    END IF;
    IF q.operator IS NOT NULL THEN
        adif := adif || '<OPERATOR:' || length(q.operator) || '>' || q.operator;
    END IF;
    IF q.my_name IS NOT NULL THEN
        adif := adif || '<MY_NAME:' || length(q.my_name) || '>' || q.my_name;
    END IF;
    IF q.my_city IS NOT NULL THEN
        adif := adif || '<MY_CITY:' || length(q.my_city) || '>' || q.my_city;
    END IF;
    IF q.my_country IS NOT NULL THEN
        adif := adif || '<MY_COUNTRY:' || length(q.my_country) || '>' || q.my_country;
    END IF;
    IF q.my_cq_zone IS NOT NULL THEN
        adif := adif || '<MY_CQ_ZONE:' || length(q.my_cq_zone::text) || '>' || q.my_cq_zone;
    END IF;
    IF q.my_itu_zone IS NOT NULL THEN
        adif := adif || '<MY_ITU_ZONE:' || length(q.my_itu_zone::text) || '>' || q.my_itu_zone;
    END IF;
    IF q.my_dxcc IS NOT NULL THEN
        adif := adif || '<MY_DXCC:' || length(q.my_dxcc::text) || '>' || q.my_dxcc;
    END IF;
    IF q.my_gridsquare IS NOT NULL THEN
        adif := adif || '<MY_GRIDSQUARE:' || length(q.my_gridsquare) || '>' || q.my_gridsquare;
    END IF;
    IF q.my_rig IS NOT NULL THEN
        adif := adif || '<MY_RIG:' || length(q.my_rig) || '>' || q.my_rig;
    END IF;
    IF q.my_antenna IS NOT NULL THEN
        adif := adif || '<MY_ANTENNA:' || length(q.my_antenna) || '>' || q.my_antenna;
    END IF;

    -- QSL fields
    IF q.qsl_sent IS NOT NULL THEN
        adif := adif || '<QSL_SENT:1>' || q.qsl_sent::text;
    END IF;
    IF q.qsl_rcvd IS NOT NULL THEN
        adif := adif || '<QSL_RCVD:1>' || q.qsl_rcvd::text;
    END IF;
    IF q.qslsdate IS NOT NULL THEN
        adif := adif || '<QSLSDATE:8>' || to_char(q.qslsdate, 'YYYYMMDD');
    END IF;
    IF q.qslrdate IS NOT NULL THEN
        adif := adif || '<QSLRDATE:8>' || to_char(q.qslrdate, 'YYYYMMDD');
    END IF;
    IF q.qsl_sent_via IS NOT NULL THEN
        adif := adif || '<QSL_SENT_VIA:1>' || q.qsl_sent_via::text;
    END IF;
    IF q.qsl_rcvd_via IS NOT NULL THEN
        adif := adif || '<QSL_RCVD_VIA:1>' || q.qsl_rcvd_via::text;
    END IF;
    IF q.qsl_via IS NOT NULL THEN
        adif := adif || '<QSL_VIA:' || length(q.qsl_via) || '>' || q.qsl_via;
    END IF;
    IF q.qslmsg IS NOT NULL THEN
        adif := adif || '<QSLMSG:' || length(q.qslmsg) || '>' || q.qslmsg;
    END IF;
    IF q.qslmsg_rcvd IS NOT NULL THEN
        adif := adif || '<QSLMSG_RCVD:' || length(q.qslmsg_rcvd) || '>' || q.qslmsg_rcvd;
    END IF;

    -- LoTW
    IF q.lotw_qsl_sent IS NOT NULL THEN
        adif := adif || '<LOTW_QSL_SENT:1>' || q.lotw_qsl_sent::text;
    END IF;
    IF q.lotw_qsl_rcvd IS NOT NULL THEN
        adif := adif || '<LOTW_QSL_RCVD:1>' || q.lotw_qsl_rcvd::text;
    END IF;
    IF q.lotw_qslsdate IS NOT NULL THEN
        adif := adif || '<LOTW_QSLSDATE:8>' || to_char(q.lotw_qslsdate, 'YYYYMMDD');
    END IF;
    IF q.lotw_qslrdate IS NOT NULL THEN
        adif := adif || '<LOTW_QSLRDATE:8>' || to_char(q.lotw_qslrdate, 'YYYYMMDD');
    END IF;

    -- eQSL
    IF q.eqsl_qsl_sent IS NOT NULL THEN
        adif := adif || '<EQSL_QSL_SENT:1>' || q.eqsl_qsl_sent::text;
    END IF;
    IF q.eqsl_qsl_rcvd IS NOT NULL THEN
        adif := adif || '<EQSL_QSL_RCVD:1>' || q.eqsl_qsl_rcvd::text;
    END IF;
    IF q.eqsl_qslsdate IS NOT NULL THEN
        adif := adif || '<EQSL_QSLSDATE:8>' || to_char(q.eqsl_qslsdate, 'YYYYMMDD');
    END IF;
    IF q.eqsl_qslrdate IS NOT NULL THEN
        adif := adif || '<EQSL_QSLRDATE:8>' || to_char(q.eqsl_qslrdate, 'YYYYMMDD');
    END IF;
    IF q.eqsl_ag IS NOT NULL THEN
        adif := adif || '<EQSL_AG:1>' || CASE WHEN q.eqsl_ag THEN 'Y' ELSE 'N' END;
    END IF;

    -- Upload tracking
    IF q.clublog_qso_upload_date IS NOT NULL THEN
        adif := adif || '<CLUBLOG_QSO_UPLOAD_DATE:8>' || to_char(q.clublog_qso_upload_date, 'YYYYMMDD');
    END IF;
    IF q.clublog_qso_upload_status IS NOT NULL THEN
        adif := adif || '<CLUBLOG_QSO_UPLOAD_STATUS:1>' || q.clublog_qso_upload_status::text;
    END IF;
    IF q.hrdlog_qso_upload_date IS NOT NULL THEN
        adif := adif || '<HRDLOG_QSO_UPLOAD_DATE:8>' || to_char(q.hrdlog_qso_upload_date, 'YYYYMMDD');
    END IF;
    IF q.hrdlog_qso_upload_status IS NOT NULL THEN
        adif := adif || '<HRDLOG_QSO_UPLOAD_STATUS:1>' || q.hrdlog_qso_upload_status::text;
    END IF;

    -- Contest fields
    IF q.contest_id IS NOT NULL THEN
        adif := adif || '<CONTEST_ID:' || length(q.contest_id) || '>' || q.contest_id;
    END IF;
    IF q.srx IS NOT NULL THEN
        adif := adif || '<SRX:' || length(q.srx::text) || '>' || q.srx;
    END IF;
    IF q.stx IS NOT NULL THEN
        adif := adif || '<STX:' || length(q.stx::text) || '>' || q.stx;
    END IF;

    -- Notes
    IF q.comment IS NOT NULL THEN
        adif := adif || '<COMMENT:' || length(q.comment) || '>' || q.comment;
    END IF;
    IF q.notes IS NOT NULL THEN
        adif := adif || '<NOTES:' || length(q.notes) || '>' || q.notes;
    END IF;

    adif := adif || '<EOR>';

    RETURN adif;
END;
$$ LANGUAGE plpgsql STABLE;
-- +goose StatementEnd

-- +goose Down

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION qso_to_adif(qso_id UUID)
RETURNS TEXT AS $$
DECLARE
    q qsos%ROWTYPE;
    adif TEXT := '';
BEGIN
    SELECT * INTO q FROM qsos WHERE id = qso_id;
    IF NOT FOUND THEN RETURN NULL; END IF;

    -- Helper to add ADIF field
    -- Format: <FIELD:length>value

    -- Required fields
    adif := adif || '<CALL:' || length(q.call) || '>' || q.call;
    adif := adif || '<QSO_DATE:8>' || to_char(q.qso_date, 'YYYYMMDD');
    adif := adif || '<TIME_ON:' || CASE WHEN q.time_on IS NOT NULL THEN '6>' || to_char(q.time_on, 'HH24MISS') ELSE '4>' || to_char(q.time_on, 'HH24MI') END;
    adif := adif || '<MODE:' || length(q.mode) || '>' || q.mode;

    -- Optional fields (only if not null)
    IF q.band IS NOT NULL THEN
        adif := adif || '<BAND:' || length(q.band) || '>' || q.band;
    END IF;
    IF q.freq IS NOT NULL THEN
        adif := adif || '<FREQ:' || length(q.freq::text) || '>' || q.freq;
    END IF;
    IF q.submode IS NOT NULL THEN
        adif := adif || '<SUBMODE:' || length(q.submode) || '>' || q.submode;
    END IF;
    IF q.rst_sent IS NOT NULL THEN
        adif := adif || '<RST_SENT:' || length(q.rst_sent) || '>' || q.rst_sent;
    END IF;
    IF q.rst_rcvd IS NOT NULL THEN
        adif := adif || '<RST_RCVD:' || length(q.rst_rcvd) || '>' || q.rst_rcvd;
    END IF;
    IF q.tx_pwr IS NOT NULL THEN
        adif := adif || '<TX_PWR:' || length(q.tx_pwr::text) || '>' || q.tx_pwr;
    END IF;
    IF q.name IS NOT NULL THEN
        adif := adif || '<NAME:' || length(q.name) || '>' || q.name;
    END IF;
    IF q.qth IS NOT NULL THEN
        adif := adif || '<QTH:' || length(q.qth) || '>' || q.qth;
    END IF;
    IF q.gridsquare IS NOT NULL THEN
        adif := adif || '<GRIDSQUARE:' || length(q.gridsquare) || '>' || q.gridsquare;
    END IF;
    IF q.country IS NOT NULL THEN
        adif := adif || '<COUNTRY:' || length(q.country) || '>' || q.country;
    END IF;
    IF q.dxcc IS NOT NULL THEN
        adif := adif || '<DXCC:' || length(q.dxcc::text) || '>' || q.dxcc;
    END IF;
    IF q.cqz IS NOT NULL THEN
        adif := adif || '<CQZ:' || length(q.cqz::text) || '>' || q.cqz;
    END IF;
    IF q.ituz IS NOT NULL THEN
        adif := adif || '<ITUZ:' || length(q.ituz::text) || '>' || q.ituz;
    END IF;
    IF q.state IS NOT NULL THEN
        adif := adif || '<STATE:' || length(q.state) || '>' || q.state;
    END IF;
    IF q.iota IS NOT NULL THEN
        adif := adif || '<IOTA:' || length(q.iota) || '>' || q.iota;
    END IF;
    IF q.sota_ref IS NOT NULL THEN
        adif := adif || '<SOTA_REF:' || length(q.sota_ref) || '>' || q.sota_ref;
    END IF;
    IF q.pota_ref IS NOT NULL THEN
        adif := adif || '<POTA_REF:' || length(q.pota_ref) || '>' || q.pota_ref;
    END IF;
    IF q.station_callsign IS NOT NULL THEN
        adif := adif || '<STATION_CALLSIGN:' || length(q.station_callsign) || '>' || q.station_callsign;
    END IF;
    IF q.my_gridsquare IS NOT NULL THEN
        adif := adif || '<MY_GRIDSQUARE:' || length(q.my_gridsquare) || '>' || q.my_gridsquare;
    END IF;

    -- QSL fields
    IF q.qsl_sent IS NOT NULL THEN
        adif := adif || '<QSL_SENT:1>' || q.qsl_sent::text;
    END IF;
    IF q.qsl_rcvd IS NOT NULL THEN
        adif := adif || '<QSL_RCVD:1>' || q.qsl_rcvd::text;
    END IF;
    IF q.lotw_qsl_sent IS NOT NULL THEN
        adif := adif || '<LOTW_QSL_SENT:1>' || q.lotw_qsl_sent::text;
    END IF;
    IF q.lotw_qsl_rcvd IS NOT NULL THEN
        adif := adif || '<LOTW_QSL_RCVD:1>' || q.lotw_qsl_rcvd::text;
    END IF;

    -- Contest fields
    IF q.contest_id IS NOT NULL THEN
        adif := adif || '<CONTEST_ID:' || length(q.contest_id) || '>' || q.contest_id;
    END IF;
    IF q.srx IS NOT NULL THEN
        adif := adif || '<SRX:' || length(q.srx::text) || '>' || q.srx;
    END IF;
    IF q.stx IS NOT NULL THEN
        adif := adif || '<STX:' || length(q.stx::text) || '>' || q.stx;
    END IF;

    -- Notes
    IF q.comment IS NOT NULL THEN
        adif := adif || '<COMMENT:' || length(q.comment) || '>' || q.comment;
    END IF;

    adif := adif || '<EOR>';

    RETURN adif;
END;
$$ LANGUAGE plpgsql STABLE;
-- +goose StatementEnd
