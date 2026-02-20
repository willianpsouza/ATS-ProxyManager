export type UserRole = 'root' | 'admin' | 'regular';
export type ConfigStatus = 'draft' | 'pending_approval' | 'approved' | 'active';
export type RuleAction = 'direct' | 'parent';

export interface User {
  id: string;
  username: string;
  email: string;
  role: UserRole;
  created_at: string;
  updated_at?: string;
  last_login?: string;
  is_active?: boolean;
}

export interface Config {
  id: string;
  name: string;
  description?: string;
  status: ConfigStatus;
  version: number;
  proxy_count?: number;
  domains?: DomainRule[];
  ip_ranges?: IPRangeRule[];
  parent_proxies?: ParentProxy[];
  proxies?: ProxySummary[];
  created_by?: UserRef;
  modified_by?: UserRef;
  modified_at: string;
  submitted_by?: UserRef;
  submitted_at?: string;
  approved_by?: UserRef;
  approved_at?: string;
  config_hash?: string;
}

export interface UserRef {
  id: string;
  username: string;
}

export interface DomainRule {
  id?: string;
  domain: string;
  action: RuleAction;
  priority: number;
}

export interface IPRangeRule {
  id?: string;
  cidr: string;
  action: RuleAction;
  priority: number;
}

export interface ParentProxy {
  id?: string;
  address: string;
  port: number;
  priority: number;
  enabled: boolean;
}

export interface ProxySummary {
  id: string;
  hostname: string;
  is_online: boolean;
  last_seen?: string;
}

export interface Proxy {
  id: string;
  hostname: string;
  config?: { id: string; name: string; version: number; config_hash?: string; in_sync: boolean };
  is_online: boolean;
  last_seen?: string;
  registered_at: string;
  current_config_hash?: string;
  stats?: ProxyStats;
  stats_history?: ProxyStatsHistory[];
}

export interface ProxyStats {
  active_connections: number;
  total_connections_1h: number;
  cache_hit_rate: number;
  total_requests_1h: number;
  errors_1h: number;
  responses_2xx_1h: number;
  responses_4xx_1h: number;
  responses_5xx_1h: number;
  bytes_in_1h: number;
  bytes_out_1h: number;
}

export interface ProxyStatsHistory {
  collected_at: string;
  active_connections: number;
  total_connections: number;
  cache_hits: number;
  cache_misses: number;
  errors: number;
  total_requests: number;
  connect_requests: number;
  responses_2xx: number;
  responses_3xx: number;
  responses_4xx: number;
  responses_5xx: number;
  err_connect_fail: number;
  err_client_abort: number;
  broken_server_conns: number;
  bytes_in: number;
  bytes_out: number;
}

export interface ProxyLogLine {
  timestamp: string;
  level: string;
  message: string;
}

export interface ProxyLogs {
  proxy_id: string;
  capture_started?: string;
  capture_ended?: string;
  lines: ProxyLogLine[];
}

export interface AuditLog {
  id: string;
  user?: UserRef;
  action: string;
  entity_type: string;
  entity_id?: string;
  changes?: Record<string, { old: string; new: string }>;
  ip_address?: string;
  created_at: string;
}

export interface Pagination {
  page: number;
  limit: number;
  total: number;
  total_pages: number;
}

export interface PaginatedResponse<T> {
  data: T[];
  pagination: Pagination;
}

export interface ApiError {
  error: string;
  message: string;
}

export interface LoginResponse {
  token: string;
  refresh_token: string;
  expires_in: number;
  user: User;
}

export interface RefreshResponse {
  token: string;
  expires_in: number;
}

export interface ProxiesListResponse {
  data: Proxy[];
  summary: {
    total: number;
    online: number;
    offline: number;
  };
}
