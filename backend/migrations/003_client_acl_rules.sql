-- Migration 003: Create client_acl_rules table for ip_allow.yaml generation
CREATE TABLE IF NOT EXISTS client_acl_rules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    config_id UUID NOT NULL REFERENCES configs(id) ON DELETE CASCADE,
    cidr VARCHAR(50) NOT NULL,
    action VARCHAR(20) NOT NULL DEFAULT 'allow',
    priority INTEGER NOT NULL DEFAULT 100,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(config_id, cidr)
);
CREATE INDEX IF NOT EXISTS idx_client_acl_rules_config ON client_acl_rules(config_id);
