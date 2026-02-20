import { useAuthStore } from '@/stores/auth-store';
import type {
  LoginResponse,
  RefreshResponse,
  PaginatedResponse,
  Config,
  User,
  Proxy,
  ProxiesListResponse,
  ProxyLogs,
  AuditLog,
  ApiError,
} from '@/types';

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? '';
const BASE = `${API_URL}/api/v1`;

let isRefreshing = false;
let refreshPromise: Promise<string | null> | null = null;

async function refreshToken(): Promise<string | null> {
  const { refreshToken: rt, setToken, logout } = useAuthStore.getState();
  if (!rt) {
    logout();
    return null;
  }

  try {
    const res = await fetch(`${BASE}/auth/refresh`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: rt }),
    });

    if (!res.ok) {
      logout();
      return null;
    }

    const data: RefreshResponse = await res.json();
    setToken(data.token);
    return data.token;
  } catch {
    logout();
    return null;
  }
}

async function fetchAPI<T>(path: string, options: RequestInit = {}): Promise<T> {
  const { token } = useAuthStore.getState();

  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string>),
  };

  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  let res = await fetch(`${BASE}${path}`, { ...options, headers });

  if (res.status === 401 && token) {
    if (!isRefreshing) {
      isRefreshing = true;
      refreshPromise = refreshToken();
    }

    const newToken = await refreshPromise;
    isRefreshing = false;
    refreshPromise = null;

    if (newToken) {
      headers['Authorization'] = `Bearer ${newToken}`;
      res = await fetch(`${BASE}${path}`, { ...options, headers });
    } else {
      const err: ApiError = { error: 'unauthorized', message: 'Session expired' };
      throw err;
    }
  }

  if (!res.ok) {
    let err: ApiError;
    try {
      err = await res.json();
    } catch {
      err = { error: 'unknown', message: `HTTP ${res.status}` };
    }
    throw err;
  }

  if (res.status === 204) {
    return undefined as T;
  }

  return res.json();
}

export const api = {
  auth: {
    login: (email: string, password: string) =>
      fetchAPI<LoginResponse>('/auth/login', {
        method: 'POST',
        body: JSON.stringify({ email, password }),
      }),
    beacon: () =>
      fetchAPI<{ status: string }>('/auth/beacon', { method: 'POST' }),
    logout: () =>
      fetchAPI<{ status: string }>('/auth/logout', { method: 'POST' }),
  },

  users: {
    list: (params?: { role?: string; page?: number; limit?: number }) => {
      const q = new URLSearchParams();
      if (params?.role) q.set('role', params.role);
      if (params?.page) q.set('page', String(params.page));
      if (params?.limit) q.set('limit', String(params.limit));
      const qs = q.toString();
      return fetchAPI<PaginatedResponse<User>>(`/users${qs ? `?${qs}` : ''}`);
    },
    create: (data: { username: string; email: string; password: string; role: string }) =>
      fetchAPI<User>('/users', { method: 'POST', body: JSON.stringify(data) }),
    update: (id: string, data: { username?: string; email?: string; role?: string }) =>
      fetchAPI<User>(`/users/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
    delete: (id: string) =>
      fetchAPI<void>(`/users/${id}`, { method: 'DELETE' }),
  },

  configs: {
    list: (params?: { status?: string; page?: number; limit?: number }) => {
      const q = new URLSearchParams();
      if (params?.status) q.set('status', params.status);
      if (params?.page) q.set('page', String(params.page));
      if (params?.limit) q.set('limit', String(params.limit));
      const qs = q.toString();
      return fetchAPI<PaginatedResponse<Config>>(`/configs${qs ? `?${qs}` : ''}`);
    },
    get: (id: string) => fetchAPI<Config>(`/configs/${id}`),
    create: (data: {
      name: string;
      description?: string;
      domains: { domain: string; action: string; priority: number }[];
      ip_ranges: { cidr: string; action: string; priority: number }[];
      parent_proxies: { address: string; port: number; priority: number; enabled: boolean }[];
      client_acl?: { cidr: string; action: string; priority: number }[];
      proxy_ids: string[];
    }) => fetchAPI<Config>('/configs', { method: 'POST', body: JSON.stringify(data) }),
    update: (
      id: string,
      data: {
        name: string;
        description?: string;
        domains: { domain: string; action: string; priority: number }[];
        ip_ranges: { cidr: string; action: string; priority: number }[];
        parent_proxies: { address: string; port: number; priority: number; enabled: boolean }[];
        client_acl?: { cidr: string; action: string; priority: number }[];
        proxy_ids: string[];
      }
    ) => fetchAPI<Config>(`/configs/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
    submit: (id: string) =>
      fetchAPI<Config>(`/configs/${id}/submit`, { method: 'POST' }),
    approve: (id: string) =>
      fetchAPI<Config>(`/configs/${id}/approve`, { method: 'POST' }),
    reject: (id: string, reason?: string) =>
      fetchAPI<Config>(`/configs/${id}/reject`, {
        method: 'POST',
        body: JSON.stringify({ reason }),
      }),
    clone: (id: string) =>
      fetchAPI<Config>(`/configs/${id}/clone`, { method: 'POST' }),
    delete: (id: string) =>
      fetchAPI<void>(`/configs/${id}`, { method: 'DELETE' }),
  },

  proxies: {
    list: () => fetchAPI<ProxiesListResponse>('/proxies'),
    get: (id: string) => fetchAPI<Proxy>(`/proxies/${id}`),
    startLogCapture: (id: string, durationMinutes: number) =>
      fetchAPI<{ status: string; capture_until: string }>(`/proxies/${id}/logs`, {
        method: 'POST',
        body: JSON.stringify({ duration_minutes: durationMinutes }),
      }),
    getLogs: (id: string) => fetchAPI<ProxyLogs>(`/proxies/${id}/logs`),
    delete: (id: string) => fetchAPI<void>(`/proxies/${id}`, { method: 'DELETE' }),
    assignConfig: (id: string, configId: string | null) =>
      fetchAPI<{ status: string }>(`/proxies/${id}/config`, {
        method: 'PUT',
        body: JSON.stringify({ config_id: configId }),
      }),
  },

  audit: {
    list: (params?: {
      entity_type?: string;
      entity_id?: string;
      user_id?: string;
      from?: string;
      to?: string;
      page?: number;
      limit?: number;
    }) => {
      const q = new URLSearchParams();
      if (params?.entity_type) q.set('entity_type', params.entity_type);
      if (params?.entity_id) q.set('entity_id', params.entity_id);
      if (params?.user_id) q.set('user_id', params.user_id);
      if (params?.from) q.set('from', params.from);
      if (params?.to) q.set('to', params.to);
      if (params?.page) q.set('page', String(params.page));
      if (params?.limit) q.set('limit', String(params.limit));
      const qs = q.toString();
      return fetchAPI<PaginatedResponse<AuditLog>>(`/audit${qs ? `?${qs}` : ''}`);
    },
  },
};
