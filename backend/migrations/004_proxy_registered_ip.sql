-- Migration 004: Add registered_ip to proxies for duplicate hostname detection
ALTER TABLE proxies ADD COLUMN IF NOT EXISTS registered_ip VARCHAR(45);
