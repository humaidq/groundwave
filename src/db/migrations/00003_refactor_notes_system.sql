-- +goose Up
-- Refactor notes system: separate contact notes from contact logs
-- and rename log type 'note' to 'general'

--------------------------------------------------------------------------------
-- CREATE CONTACT NOTES TABLE
--------------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS contact_notes (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contact_id      UUID NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,

    content         TEXT NOT NULL,                        -- note content
    noted_at        TIMESTAMPTZ NOT NULL DEFAULT now(),   -- when the note was created

    -- Metadata
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_contact_notes_contact ON contact_notes(contact_id);
CREATE INDEX IF NOT EXISTS idx_contact_notes_date ON contact_notes(noted_at DESC);

-- Trigger for contact_notes updated_at
DROP TRIGGER IF EXISTS contact_notes_updated_at ON contact_notes;
CREATE TRIGGER contact_notes_updated_at
    BEFORE UPDATE ON contact_notes
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

--------------------------------------------------------------------------------
-- MIGRATE EXISTING NOTES FROM CONTACTS TABLE
--------------------------------------------------------------------------------

-- Migrate existing notes to the new contact_notes table
-- Use the contact's created_at as the noted_at timestamp
INSERT INTO contact_notes (contact_id, content, noted_at, created_at)
SELECT id, notes, created_at, created_at
FROM contacts
WHERE notes IS NOT NULL AND notes != '';

--------------------------------------------------------------------------------
-- RENAME LOG TYPE 'note' TO 'general'
--------------------------------------------------------------------------------

-- Drop and recreate the log_type enum with the new value
-- We need to do this carefully to avoid breaking the existing data

-- Step 1: Create a temporary type
-- +goose StatementBegin
DO $$ BEGIN
    CREATE TYPE log_type_new AS ENUM (
        'general',        -- renamed from 'note'
        'email_sent',
        'email_received',
        'call',
        'meeting',
        'message',
        'gift_sent',
        'gift_received',
        'intro',
        'other'
    );
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;
-- +goose StatementEnd

-- Step 2: Drop the default value temporarily
ALTER TABLE contact_logs ALTER COLUMN log_type DROP DEFAULT;

-- Step 3: Alter the column to use the new type, converting 'note' to 'general'
ALTER TABLE contact_logs
    ALTER COLUMN log_type TYPE log_type_new
    USING (CASE WHEN log_type::text = 'note' THEN 'general'::log_type_new ELSE log_type::text::log_type_new END);

-- Step 4: Drop the old type
DROP TYPE log_type;

-- Step 5: Rename the new type to the original name
ALTER TYPE log_type_new RENAME TO log_type;

-- Step 6: Restore the default value (now using 'general' instead of 'note')
ALTER TABLE contact_logs ALTER COLUMN log_type SET DEFAULT 'general'::log_type;

--------------------------------------------------------------------------------
-- DROP NOTES COLUMN FROM CONTACTS TABLE
--------------------------------------------------------------------------------

-- Drop the column with CASCADE to remove any dependent views or constraints
ALTER TABLE contacts DROP COLUMN IF EXISTS notes CASCADE;

-- +goose Down
-- Restore notes column to contacts table
ALTER TABLE contacts ADD COLUMN notes TEXT;

-- Migrate notes back from contact_notes to contacts
-- Take the most recent note for each contact
WITH latest_notes AS (
    SELECT DISTINCT ON (contact_id)
        contact_id,
        content
    FROM contact_notes
    ORDER BY contact_id, noted_at DESC
)
UPDATE contacts c
SET notes = ln.content
FROM latest_notes ln
WHERE c.id = ln.contact_id;

-- Drop the contact_notes table
DROP TABLE IF EXISTS contact_notes;

-- Restore the original log_type enum with 'note'
-- +goose StatementBegin
DO $$ BEGIN
    CREATE TYPE log_type_old AS ENUM (
        'note',
        'email_sent',
        'email_received',
        'call',
        'meeting',
        'message',
        'gift_sent',
        'gift_received',
        'intro',
        'other'
    );
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;
-- +goose StatementEnd

-- Drop the default value temporarily
ALTER TABLE contact_logs ALTER COLUMN log_type DROP DEFAULT;

-- Convert 'general' back to 'note' during type conversion
ALTER TABLE contact_logs
    ALTER COLUMN log_type TYPE log_type_old
    USING (CASE WHEN log_type::text = 'general' THEN 'note'::log_type_old ELSE log_type::text::log_type_old END);

DROP TYPE log_type;

ALTER TYPE log_type_old RENAME TO log_type;

-- Restore the default value (now using 'note' again)
ALTER TABLE contact_logs ALTER COLUMN log_type SET DEFAULT 'note'::log_type;
