-- Add default_action column to configs table
ALTER TABLE configs ADD COLUMN IF NOT EXISTS default_action VARCHAR(20) NOT NULL DEFAULT 'direct';
