'use client';

import { useEffect, useState } from 'react';
import { api } from '@/lib/api';
import type { ProxiesListResponse, PaginatedResponse, Config, AuditLog, ProxyStats } from '@/types';
import { formatRelative, formatBytes } from '@/lib/utils';

interface DashboardData {
  proxySummary: { total: number; online: number; offline: number } | null;
  activeConfigs: number;
  latestAudit: AuditLog | null;
  totalConfigs: number;
  aggregatedStats: {
    totalRequests: number;
    totalErrors: number;
    errorRate: string;
    totalBytesIn: number;
    totalBytesOut: number;
    responses2xx: number;
    responses4xx: number;
    responses5xx: number;
  };
}

export default function DashboardPage() {
  const [data, setData] = useState<DashboardData>({
    proxySummary: null,
    activeConfigs: 0,
    latestAudit: null,
    totalConfigs: 0,
    aggregatedStats: {
      totalRequests: 0, totalErrors: 0, errorRate: '0.00',
      totalBytesIn: 0, totalBytesOut: 0,
      responses2xx: 0, responses4xx: 0, responses5xx: 0,
    },
  });
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    async function load() {
      try {
        const [proxies, configs, audit] = await Promise.all([
          api.proxies.list().catch(() => null),
          api.configs.list({ limit: 1 }).catch(() => null),
          api.audit.list({ limit: 1 }).catch(() => null),
        ]);

        const activeConfigs = await api.configs
          .list({ status: 'active', limit: 1 })
          .catch(() => null);

        // Aggregate stats from all proxies
        const proxyList = (proxies as ProxiesListResponse)?.data || [];
        const agg = {
          totalRequests: 0, totalErrors: 0,
          totalBytesIn: 0, totalBytesOut: 0,
          responses2xx: 0, responses4xx: 0, responses5xx: 0,
        };
        for (const p of proxyList) {
          const s = p.stats as ProxyStats | undefined;
          if (s) {
            agg.totalRequests += s.total_requests_1h || 0;
            agg.totalErrors += s.errors_1h || 0;
            agg.totalBytesIn += s.bytes_in_1h || 0;
            agg.totalBytesOut += s.bytes_out_1h || 0;
            agg.responses2xx += s.responses_2xx_1h || 0;
            agg.responses4xx += s.responses_4xx_1h || 0;
            agg.responses5xx += s.responses_5xx_1h || 0;
          }
        }
        const errorRate = agg.totalRequests > 0
          ? ((agg.totalErrors / agg.totalRequests) * 100).toFixed(2)
          : '0.00';

        setData({
          proxySummary: (proxies as ProxiesListResponse)?.summary || null,
          activeConfigs: (activeConfigs as PaginatedResponse<Config>)?.pagination?.total || 0,
          latestAudit: (audit as PaginatedResponse<AuditLog>)?.data?.[0] || null,
          totalConfigs: (configs as PaginatedResponse<Config>)?.pagination?.total || 0,
          aggregatedStats: { ...agg, errorRate },
        });
      } finally {
        setLoading(false);
      }
    }
    load();
    const interval = setInterval(load, 30000);
    return () => clearInterval(interval);
  }, []);

  if (loading) {
    return (
      <div className="animate-pulse space-y-4">
        <div className="h-8 w-48 bg-gray-200 rounded" />
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="h-28 bg-gray-200 rounded-lg" />
          ))}
        </div>
      </div>
    );
  }

  const { aggregatedStats: agg } = data;

  return (
    <div>
      <h1 className="text-2xl font-bold text-gray-900 mb-6">Dashboard</h1>

      {/* Row 1: Infrastructure */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
        <Card
          title="Proxies Online"
          value={data.proxySummary?.online ?? 0}
          subtitle={`${data.proxySummary?.total ?? 0} total`}
          color="green"
        />
        <Card
          title="Proxies Offline"
          value={data.proxySummary?.offline ?? 0}
          subtitle={`${data.proxySummary?.total ?? 0} total`}
          color={data.proxySummary?.offline ? 'red' : 'gray'}
        />
        <Card
          title="Configs Ativas"
          value={data.activeConfigs}
          subtitle={`${data.totalConfigs} total`}
          color="blue"
        />
        <Card
          title="Último Evento"
          value={data.latestAudit?.action || '-'}
          subtitle={data.latestAudit ? formatRelative(data.latestAudit.created_at) : '-'}
          color="purple"
          isText
        />
      </div>

      {/* Row 2: Traffic Stats */}
      <h2 className="text-lg font-semibold text-gray-800 mb-3">Tráfego (Última Hora)</h2>
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
        <Card
          title="Total Requests"
          value={agg.totalRequests.toLocaleString()}
          subtitle={`2xx: ${agg.responses2xx.toLocaleString()}`}
          color="blue"
          isText
        />
        <Card
          title="Erros"
          value={agg.totalErrors.toLocaleString()}
          subtitle={`Taxa: ${agg.errorRate}%`}
          color={agg.totalErrors > 0 ? 'red' : 'gray'}
          isText
        />
        <Card
          title="Bytes Recebidos"
          value={formatBytes(agg.totalBytesIn)}
          subtitle="client → proxy"
          color="gray"
          isText
        />
        <Card
          title="Bytes Enviados"
          value={formatBytes(agg.totalBytesOut)}
          subtitle="proxy → client"
          color="gray"
          isText
        />
      </div>

      {/* Row 3: Status Codes */}
      {(agg.responses4xx > 0 || agg.responses5xx > 0) && (
        <>
          <h2 className="text-lg font-semibold text-gray-800 mb-3">Status Codes</h2>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <Card title="2xx (Sucesso)" value={agg.responses2xx.toLocaleString()} subtitle="" color="green" isText />
            <Card title="4xx (Client Error)" value={agg.responses4xx.toLocaleString()} subtitle="" color={agg.responses4xx > 0 ? 'yellow' : 'gray'} isText />
            <Card title="5xx (Server Error)" value={agg.responses5xx.toLocaleString()} subtitle="" color={agg.responses5xx > 0 ? 'red' : 'gray'} isText />
          </div>
        </>
      )}
    </div>
  );
}

function Card({
  title,
  value,
  subtitle,
  color,
  isText,
}: {
  title: string;
  value: number | string;
  subtitle: string;
  color: string;
  isText?: boolean;
}) {
  const colorMap: Record<string, string> = {
    green: 'bg-green-50 border-green-200',
    red: 'bg-red-50 border-red-200',
    blue: 'bg-blue-50 border-blue-200',
    purple: 'bg-purple-50 border-purple-200',
    yellow: 'bg-yellow-50 border-yellow-200',
    gray: 'bg-gray-50 border-gray-200',
  };

  return (
    <div className={`rounded-lg border p-5 ${colorMap[color] || colorMap.gray}`}>
      <p className="text-sm font-medium text-gray-600">{title}</p>
      <p className={`mt-2 ${isText ? 'text-lg' : 'text-3xl'} font-bold text-gray-900`}>
        {value}
      </p>
      {subtitle && <p className="text-xs text-gray-500 mt-1">{subtitle}</p>}
    </div>
  );
}
