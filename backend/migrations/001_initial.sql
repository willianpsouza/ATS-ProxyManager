-- ATS Proxy Manager - Database Schema
-- PostgreSQL 15+

-- =============================================================================
-- EXTENSIONS
-- =============================================================================

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- =============================================================================
-- ENUMS
-- =============================================================================

CREATE TYPE user_role AS ENUM ('root', 'admin', 'regular');
CREATE TYPE config_status AS ENUM ('draft', 'pending_approval', 'approved', 'active');
CREATE TYPE rule_action AS ENUM ('direct', 'parent');

-- =============================================================================
-- TABLES
-- =============================================================================

-- -----------------------------------------------------------------------------
-- Users
-- -----------------------------------------------------------------------------

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    username VARCHAR(100) NOT NULL UNIQUE,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    role user_role NOT NULL DEFAULT 'regular',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_login TIMESTAMP WITH TIME ZONE,
    is_active BOOLEAN DEFAULT TRUE
);

-- Índices
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_role ON users(role);

-- Usuário root inicial (senha: changeme)
INSERT INTO users (username, email, password_hash, role) VALUES (
    'root',
    'root@proxy-manager.local',
    crypt('changeme', gen_salt('bf')),
    'root'
);

-- -----------------------------------------------------------------------------
-- Configs (Configurações)
-- -----------------------------------------------------------------------------

CREATE TABLE configs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    status config_status NOT NULL DEFAULT 'draft',
    version INTEGER NOT NULL DEFAULT 1,
    
    -- Audit
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    modified_by UUID REFERENCES users(id),
    modified_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    submitted_by UUID REFERENCES users(id),
    submitted_at TIMESTAMP WITH TIME ZONE,
    approved_by UUID REFERENCES users(id),
    approved_at TIMESTAMP WITH TIME ZONE,
    
    -- Hash da config gerada (para sync)
    config_hash VARCHAR(64)
);

-- Índices
CREATE INDEX idx_configs_status ON configs(status);
CREATE INDEX idx_configs_name ON configs(name);

-- -----------------------------------------------------------------------------
-- Domain Rules (Regras de domínio)
-- -----------------------------------------------------------------------------

CREATE TABLE domain_rules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    config_id UUID NOT NULL REFERENCES configs(id) ON DELETE CASCADE,
    domain VARCHAR(255) NOT NULL,  -- Ex: .eec, .eeca, .svc.cluster.local
    action rule_action NOT NULL DEFAULT 'direct',
    priority INTEGER NOT NULL DEFAULT 100,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    UNIQUE(config_id, domain)
);

-- Índices
CREATE INDEX idx_domain_rules_config ON domain_rules(config_id);

-- -----------------------------------------------------------------------------
-- IP Range Rules (Regras de IP)
-- -----------------------------------------------------------------------------

CREATE TABLE ip_range_rules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    config_id UUID NOT NULL REFERENCES configs(id) ON DELETE CASCADE,
    cidr VARCHAR(50) NOT NULL,  -- Ex: 10.0.0.0/8
    action rule_action NOT NULL DEFAULT 'direct',
    priority INTEGER NOT NULL DEFAULT 100,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    UNIQUE(config_id, cidr)
);

-- Índices
CREATE INDEX idx_ip_range_rules_config ON ip_range_rules(config_id);

-- -----------------------------------------------------------------------------
-- Parent Proxies (Proxies upstream)
-- -----------------------------------------------------------------------------

CREATE TABLE parent_proxies (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    config_id UUID NOT NULL REFERENCES configs(id) ON DELETE CASCADE,
    address VARCHAR(255) NOT NULL,  -- IP ou hostname
    port INTEGER NOT NULL DEFAULT 3128,
    priority INTEGER NOT NULL DEFAULT 1,  -- Ordem de failover
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    UNIQUE(config_id, address, port)
);

-- Índices
CREATE INDEX idx_parent_proxies_config ON parent_proxies(config_id);

-- -----------------------------------------------------------------------------
-- Proxies (Instâncias de proxy registradas)
-- -----------------------------------------------------------------------------

CREATE TABLE proxies (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    hostname VARCHAR(255) NOT NULL UNIQUE,
    config_id UUID REFERENCES configs(id),
    
    -- Status
    is_online BOOLEAN DEFAULT FALSE,
    last_seen TIMESTAMP WITH TIME ZONE,
    current_config_hash VARCHAR(64),
    
    -- Registro
    registered_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Captura de logs
    capture_logs_until TIMESTAMP WITH TIME ZONE
);

-- Índices
CREATE INDEX idx_proxies_hostname ON proxies(hostname);
CREATE INDEX idx_proxies_config ON proxies(config_id);
CREATE INDEX idx_proxies_online ON proxies(is_online);

-- -----------------------------------------------------------------------------
-- Config-Proxy Association (Quais proxies usam qual config)
-- -----------------------------------------------------------------------------

CREATE TABLE config_proxies (
    config_id UUID NOT NULL REFERENCES configs(id) ON DELETE CASCADE,
    proxy_id UUID NOT NULL REFERENCES proxies(id) ON DELETE CASCADE,
    assigned_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    assigned_by UUID REFERENCES users(id),
    
    PRIMARY KEY (config_id, proxy_id)
);

-- -----------------------------------------------------------------------------
-- Proxy Stats (Métricas coletadas dos proxies)
-- -----------------------------------------------------------------------------

CREATE TABLE proxy_stats (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    proxy_id UUID NOT NULL REFERENCES proxies(id) ON DELETE CASCADE,
    collected_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Métricas
    active_connections INTEGER DEFAULT 0,
    total_connections BIGINT DEFAULT 0,
    cache_hits BIGINT DEFAULT 0,
    cache_misses BIGINT DEFAULT 0,
    errors INTEGER DEFAULT 0
);

-- Índices
CREATE INDEX idx_proxy_stats_proxy ON proxy_stats(proxy_id);
CREATE INDEX idx_proxy_stats_time ON proxy_stats(collected_at DESC);

-- Particionar por tempo (opcional, para alta volumetria)
-- CREATE TABLE proxy_stats_2025_02 PARTITION OF proxy_stats
--     FOR VALUES FROM ('2025-02-01') TO ('2025-03-01');

-- -----------------------------------------------------------------------------
-- Proxy Logs (Logs capturados temporariamente)
-- -----------------------------------------------------------------------------

CREATE TABLE proxy_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    proxy_id UUID NOT NULL REFERENCES proxies(id) ON DELETE CASCADE,
    captured_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    log_level VARCHAR(20),
    message TEXT,
    
    -- Auto-delete após 1 hora
    expires_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() + INTERVAL '1 hour'
);

-- Índices
CREATE INDEX idx_proxy_logs_proxy ON proxy_logs(proxy_id);
CREATE INDEX idx_proxy_logs_time ON proxy_logs(captured_at DESC);
CREATE INDEX idx_proxy_logs_expires ON proxy_logs(expires_at);

-- -----------------------------------------------------------------------------
-- Audit Log (Histórico de ações)
-- -----------------------------------------------------------------------------

CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES users(id),
    action VARCHAR(100) NOT NULL,  -- Ex: config.create, config.approve, user.delete
    entity_type VARCHAR(50) NOT NULL,  -- Ex: config, user, proxy
    entity_id UUID,
    old_value JSONB,
    new_value JSONB,
    ip_address VARCHAR(45),
    user_agent TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Índices
CREATE INDEX idx_audit_logs_user ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_entity ON audit_logs(entity_type, entity_id);
CREATE INDEX idx_audit_logs_action ON audit_logs(action);
CREATE INDEX idx_audit_logs_time ON audit_logs(created_at DESC);

-- -----------------------------------------------------------------------------
-- Sessions (Controle de sessões JWT)
-- -----------------------------------------------------------------------------

CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(64) NOT NULL,
    refresh_token_hash VARCHAR(64),
    ip_address VARCHAR(45),
    user_agent TEXT,
    last_beacon TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    revoked_at TIMESTAMP WITH TIME ZONE
);

-- Índices
CREATE INDEX idx_sessions_user ON sessions(user_id);
CREATE INDEX idx_sessions_token ON sessions(token_hash);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);

-- =============================================================================
-- FUNCTIONS
-- =============================================================================

-- Função para atualizar updated_at automaticamente
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger para users
CREATE TRIGGER trigger_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

-- Função para calcular hash da config
CREATE OR REPLACE FUNCTION calculate_config_hash(p_config_id UUID)
RETURNS VARCHAR(64) AS $$
DECLARE
    v_hash VARCHAR(64);
BEGIN
    SELECT encode(
        digest(
            string_agg(content, '|' ORDER BY seq),
            'sha256'
        ),
        'hex'
    )
    INTO v_hash
    FROM (
        -- Domínios
        SELECT 1 as seq, string_agg(domain || ':' || action || ':' || priority, ',' ORDER BY priority) as content
        FROM domain_rules WHERE config_id = p_config_id
        UNION ALL
        -- IPs
        SELECT 2 as seq, string_agg(cidr || ':' || action || ':' || priority, ',' ORDER BY priority) as content
        FROM ip_range_rules WHERE config_id = p_config_id
        UNION ALL
        -- Parent proxies
        SELECT 3 as seq, string_agg(address || ':' || port || ':' || priority, ',' ORDER BY priority) as content
        FROM parent_proxies WHERE config_id = p_config_id AND enabled = true
    ) sub;
    
    RETURN COALESCE(v_hash, 'empty');
END;
$$ LANGUAGE plpgsql;

-- Função para atualizar status de proxy (online/offline)
CREATE OR REPLACE FUNCTION update_proxy_online_status()
RETURNS void AS $$
BEGIN
    UPDATE proxies
    SET is_online = FALSE
    WHERE last_seen < NOW() - INTERVAL '2 minutes'
    AND is_online = TRUE;
END;
$$ LANGUAGE plpgsql;

-- Função para limpar logs expirados
CREATE OR REPLACE FUNCTION cleanup_expired_logs()
RETURNS void AS $$
BEGIN
    DELETE FROM proxy_logs WHERE expires_at < NOW();
END;
$$ LANGUAGE plpgsql;

-- Função para limpar stats antigas (manter últimos 7 dias)
CREATE OR REPLACE FUNCTION cleanup_old_stats()
RETURNS void AS $$
BEGIN
    DELETE FROM proxy_stats WHERE collected_at < NOW() - INTERVAL '7 days';
END;
$$ LANGUAGE plpgsql;

-- =============================================================================
-- VIEWS
-- =============================================================================

-- View de proxies com estatísticas recentes
CREATE OR REPLACE VIEW v_proxies_with_stats AS
SELECT 
    p.id,
    p.hostname,
    p.config_id,
    c.name as config_name,
    p.is_online,
    p.last_seen,
    p.current_config_hash,
    p.registered_at,
    
    -- Stats da última hora
    COALESCE(s.active_connections, 0) as active_connections,
    COALESCE(s.total_connections_1h, 0) as total_connections_1h,
    COALESCE(s.cache_hits_1h, 0) as cache_hits_1h,
    COALESCE(s.cache_misses_1h, 0) as cache_misses_1h,
    CASE 
        WHEN COALESCE(s.cache_hits_1h, 0) + COALESCE(s.cache_misses_1h, 0) > 0 
        THEN ROUND(s.cache_hits_1h::numeric / (s.cache_hits_1h + s.cache_misses_1h) * 100, 2)
        ELSE 0 
    END as cache_hit_rate
FROM proxies p
LEFT JOIN configs c ON p.config_id = c.id
LEFT JOIN LATERAL (
    SELECT 
        (SELECT active_connections FROM proxy_stats WHERE proxy_id = p.id ORDER BY collected_at DESC LIMIT 1) as active_connections,
        SUM(total_connections) as total_connections_1h,
        SUM(cache_hits) as cache_hits_1h,
        SUM(cache_misses) as cache_misses_1h
    FROM proxy_stats
    WHERE proxy_id = p.id
    AND collected_at > NOW() - INTERVAL '1 hour'
) s ON true;

-- View de configs com contagem de proxies
CREATE OR REPLACE VIEW v_configs_summary AS
SELECT 
    c.id,
    c.name,
    c.description,
    c.status,
    c.version,
    c.modified_at,
    c.approved_at,
    u_mod.username as modified_by_username,
    u_app.username as approved_by_username,
    COUNT(DISTINCT cp.proxy_id) as proxy_count,
    COUNT(DISTINCT dr.id) as domain_count,
    COUNT(DISTINCT ir.id) as ip_range_count,
    COUNT(DISTINCT pp.id) as parent_proxy_count
FROM configs c
LEFT JOIN users u_mod ON c.modified_by = u_mod.id
LEFT JOIN users u_app ON c.approved_by = u_app.id
LEFT JOIN config_proxies cp ON c.id = cp.config_id
LEFT JOIN domain_rules dr ON c.id = dr.config_id
LEFT JOIN ip_range_rules ir ON c.id = ir.config_id
LEFT JOIN parent_proxies pp ON c.id = pp.config_id
GROUP BY c.id, c.name, c.description, c.status, c.version, 
         c.modified_at, c.approved_at, u_mod.username, u_app.username;

-- View de dashboard summary
CREATE OR REPLACE VIEW v_dashboard_summary AS
SELECT
    (SELECT COUNT(*) FROM proxies WHERE is_online = true) as proxies_online,
    (SELECT COUNT(*) FROM proxies WHERE is_online = false) as proxies_offline,
    (SELECT COUNT(*) FROM configs WHERE status = 'pending_approval') as configs_pending,
    (SELECT COUNT(*) FROM configs WHERE status = 'active') as configs_active,
    (SELECT COALESCE(SUM(active_connections), 0) FROM proxy_stats 
     WHERE collected_at > NOW() - INTERVAL '1 minute') as total_active_connections,
    (SELECT COALESCE(SUM(total_connections), 0) FROM proxy_stats 
     WHERE collected_at > NOW() - INTERVAL '1 hour') as total_connections_1h;

-- =============================================================================
-- SCHEDULED JOBS (usar pg_cron ou app-level)
-- =============================================================================

-- Exemplo de uso com pg_cron:
-- SELECT cron.schedule('cleanup-logs', '*/5 * * * *', 'SELECT cleanup_expired_logs()');
-- SELECT cron.schedule('cleanup-stats', '0 2 * * *', 'SELECT cleanup_old_stats()');
-- SELECT cron.schedule('update-proxy-status', '* * * * *', 'SELECT update_proxy_online_status()');

-- =============================================================================
-- GRANTS (ajustar conforme necessário)
-- =============================================================================

-- GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO proxy_app;
-- GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO proxy_app;
