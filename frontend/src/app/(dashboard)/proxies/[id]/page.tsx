'use client';

import { useEffect, useState, useCallback, useRef } from 'react';
import { useParams } from 'next/navigation';
import Link from 'next/link';
import toast from 'react-hot-toast';
import { api } from '@/lib/api';
import type { Proxy, ProxyLogs, ApiError } from '@/types';
import { StatusBadge } from '@/components/status-badge';
import { Loading } from '@/components/loading';
import { formatDate, formatRelative, formatBytes } from '@/lib/utils';

export default function ProxyDetailPage() {
  const params = useParams();
  const id = params.id as string;

  const [proxy, setProxy] = useState<Proxy | null>(null);
  const [loading, setLoading] = useState(true);
  const [logs, setLogs] = useState<ProxyLogs | null>(null);
  const [capturing, setCapturing] = useState(false);
  const [captureMinutes, setCaptureMinutes] = useState(1);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const load = useCallback(async () => {
    try {
      const data = await api.proxies.get(id);
      setProxy(data);
    } catch (err) {
      toast.error((err as ApiError).message || 'Erro ao carregar proxy');
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    load();
    const interval = setInterval(load, 30000);
    return () => {
      clearInterval(interval);
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, [load]);

  async function startCapture() {
    try {
      await api.proxies.startLogCapture(id, captureMinutes);
      setCapturing(true);
      toast.success('Captura de logs iniciada');

      pollRef.current = setInterval(async () => {
        try {
          const res = await api.proxies.getLogs(id);
          setLogs(res);
          if (res.capture_ended) {
            setCapturing(false);
            if (pollRef.current) clearInterval(pollRef.current);
            toast.success('Captura finalizada');
          }
        } catch {
          // keep polling
        }
      }, 3000);
    } catch (err) {
      toast.error((err as ApiError).message || 'Erro ao iniciar captura');
    }
  }

  async function fetchLogs() {
    try {
      const res = await api.proxies.getLogs(id);
      setLogs(res);
    } catch (err) {
      toast.error((err as ApiError).message || 'Erro ao buscar logs');
    }
  }

  if (loading) return <Loading />;
  if (!proxy) return <p className="text-gray-500">Proxy não encontrado.</p>;

  const stats = proxy.stats;
  const errorRate = stats && stats.total_requests_1h > 0
    ? ((stats.errors_1h / stats.total_requests_1h) * 100).toFixed(2)
    : '0.00';

  return (
    <div>
      <div className="flex items-center gap-3 mb-6">
        <h1 className="text-2xl font-bold text-gray-900">{proxy.hostname}</h1>
        <StatusBadge status={proxy.is_online ? 'online' : 'offline'} />
      </div>

      {/* Info Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
        <InfoCard label="Registrado em" value={formatDate(proxy.registered_at)} />
        <InfoCard label="Último Sinal" value={formatRelative(proxy.last_seen)} />
        <InfoCard
          label="Config"
          value={
            proxy.config ? (
              <Link href={`/configs/${proxy.config.id}`} className="text-blue-600 hover:underline">
                {proxy.config.name}
              </Link>
            ) : (
              '-'
            )
          }
        />
        <InfoCard label="Hash" value={proxy.current_config_hash?.slice(0, 12) || '-'} mono />
      </div>

      {/* Stats Overview */}
      {stats && (
        <div className="bg-white rounded-lg border p-5 mb-6">
          <h2 className="text-base font-semibold text-gray-900 mb-4">Estatísticas (Última Hora)</h2>

          {/* Row 1: Conexões e Requests */}
          <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 gap-4 mb-4">
            <StatCard label="Conexões Ativas" value={stats.active_connections} />
            <StatCard label="Conexões (1h)" value={stats.total_connections_1h} />
            <StatCard label="Requests (1h)" value={stats.total_requests_1h} />
            <StatCard label="Cache Hit Rate" value={`${(stats.cache_hit_rate * 100).toFixed(1)}%`} />
            <StatCard label="Bytes In (1h)" value={formatBytes(stats.bytes_in_1h)} />
            <StatCard label="Bytes Out (1h)" value={formatBytes(stats.bytes_out_1h)} />
          </div>

          {/* Row 2: Status Codes */}
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-4">
            <StatCard label="2xx Responses" value={stats.responses_2xx_1h} color="green" />
            <StatCard label="4xx Responses" value={stats.responses_4xx_1h} color={stats.responses_4xx_1h > 0 ? 'yellow' : 'default'} />
            <StatCard label="5xx Responses" value={stats.responses_5xx_1h} color={stats.responses_5xx_1h > 0 ? 'red' : 'default'} />
            <StatCard label="Taxa de Erros" value={`${errorRate}%`} color={parseFloat(errorRate) > 0 ? 'red' : 'default'} />
          </div>

          {/* Row 3: Erros detalhados */}
          {stats.errors_1h > 0 && (
            <div className="bg-red-50 border border-red-200 rounded-md p-3">
              <p className="text-sm font-medium text-red-800 mb-2">Detalhamento de Erros (1h)</p>
              <div className="grid grid-cols-3 gap-2 text-sm text-red-700">
                <span>Total: {stats.errors_1h}</span>
              </div>
            </div>
          )}
        </div>
      )}

      {/* Stats History Table */}
      {proxy.stats_history && proxy.stats_history.length > 0 && (
        <div className="bg-white rounded-lg border p-5 mb-6">
          <h2 className="text-base font-semibold text-gray-900 mb-4">Histórico de Stats</h2>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-gray-50">
                <tr>
                  <th className="text-left px-3 py-2 font-medium text-gray-600">Timestamp</th>
                  <th className="text-right px-3 py-2 font-medium text-gray-600">Ativas</th>
                  <th className="text-right px-3 py-2 font-medium text-gray-600">Total Conn</th>
                  <th className="text-right px-3 py-2 font-medium text-gray-600">Requests</th>
                  <th className="text-right px-3 py-2 font-medium text-gray-600">CONNECT</th>
                  <th className="text-right px-3 py-2 font-medium text-gray-600">2xx</th>
                  <th className="text-right px-3 py-2 font-medium text-gray-600">4xx</th>
                  <th className="text-right px-3 py-2 font-medium text-gray-600">5xx</th>
                  <th className="text-right px-3 py-2 font-medium text-gray-600">Erros</th>
                  <th className="text-right px-3 py-2 font-medium text-gray-600">Bytes In</th>
                  <th className="text-right px-3 py-2 font-medium text-gray-600">Bytes Out</th>
                </tr>
              </thead>
              <tbody className="divide-y">
                {proxy.stats_history.map((s, i) => (
                  <tr key={i} className="hover:bg-gray-50">
                    <td className="px-3 py-2 text-gray-600 whitespace-nowrap">{formatDate(s.collected_at)}</td>
                    <td className="px-3 py-2 text-right">{s.active_connections}</td>
                    <td className="px-3 py-2 text-right">{s.total_connections.toLocaleString()}</td>
                    <td className="px-3 py-2 text-right">{s.total_requests.toLocaleString()}</td>
                    <td className="px-3 py-2 text-right">{s.connect_requests.toLocaleString()}</td>
                    <td className="px-3 py-2 text-right text-green-600">{s.responses_2xx.toLocaleString()}</td>
                    <td className="px-3 py-2 text-right text-yellow-600">{s.responses_4xx > 0 ? s.responses_4xx.toLocaleString() : '-'}</td>
                    <td className="px-3 py-2 text-right text-red-600">{s.responses_5xx > 0 ? s.responses_5xx.toLocaleString() : '-'}</td>
                    <td className="px-3 py-2 text-right">{s.errors > 0 ? <span className="text-red-600">{s.errors}</span> : '-'}</td>
                    <td className="px-3 py-2 text-right text-gray-500">{formatBytes(s.bytes_in)}</td>
                    <td className="px-3 py-2 text-right text-gray-500">{formatBytes(s.bytes_out)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Log Capture */}
      <div className="bg-white rounded-lg border p-5">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-base font-semibold text-gray-900">Captura de Logs</h2>
          <div className="flex items-center gap-2">
            {!capturing && (
              <>
                <select
                  value={captureMinutes}
                  onChange={(e) => setCaptureMinutes(parseInt(e.target.value))}
                  className="px-3 py-1.5 border border-gray-300 rounded-md text-sm"
                >
                  {[1, 2, 3, 4, 5].map((m) => (
                    <option key={m} value={m}>
                      {m} min
                    </option>
                  ))}
                </select>
                <button
                  onClick={startCapture}
                  className="px-4 py-1.5 text-sm bg-blue-600 text-white rounded-md hover:bg-blue-700"
                >
                  Capturar
                </button>
                <button
                  onClick={fetchLogs}
                  className="px-4 py-1.5 text-sm border rounded-md hover:bg-gray-50"
                >
                  Ver Logs
                </button>
              </>
            )}
            {capturing && (
              <span className="text-sm text-yellow-600 flex items-center gap-2">
                <span className="w-2 h-2 bg-yellow-500 rounded-full animate-pulse" />
                Capturando...
              </span>
            )}
          </div>
        </div>

        {logs && (
          <div>
            <div className="text-xs text-gray-500 mb-2">
              {logs.capture_started && <span>Início: {formatDate(logs.capture_started)}</span>}
              {logs.capture_ended && <span className="ml-4">Fim: {formatDate(logs.capture_ended)}</span>}
              <span className="ml-4">{logs.lines?.length || 0} linhas</span>
            </div>
            <div className="bg-gray-900 text-gray-100 rounded-md p-4 max-h-96 overflow-y-auto font-mono text-xs">
              {(!logs.lines || logs.lines.length === 0) ? (
                <p className="text-gray-500">Nenhum log capturado.</p>
              ) : (
                logs.lines.map((line, i) => (
                  <div key={i} className="py-0.5">
                    <span className="text-gray-500">{formatDate(line.timestamp)}</span>{' '}
                    <span
                      className={
                        line.level === 'ERROR'
                          ? 'text-red-400'
                          : line.level === 'WARN'
                            ? 'text-yellow-400'
                            : 'text-blue-400'
                      }
                    >
                      [{line.level}]
                    </span>{' '}
                    {line.message}
                  </div>
                ))
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function StatCard({
  label,
  value,
  color = 'default',
}: {
  label: string;
  value: number | string;
  color?: string;
}) {
  const colorClass = {
    green: 'text-green-600',
    yellow: 'text-yellow-600',
    red: 'text-red-600',
    default: 'text-gray-900',
  }[color] || 'text-gray-900';

  return (
    <div>
      <p className="text-xs text-gray-500">{label}</p>
      <p className={`text-lg font-bold ${colorClass}`}>
        {typeof value === 'number' ? value.toLocaleString() : value}
      </p>
    </div>
  );
}

function InfoCard({
  label,
  value,
  mono,
}: {
  label: string;
  value: React.ReactNode;
  mono?: boolean;
}) {
  return (
    <div className="bg-white rounded-lg border p-4">
      <p className="text-sm text-gray-500">{label}</p>
      <p className={`mt-1 text-sm font-medium text-gray-900 ${mono ? 'font-mono' : ''}`}>{value}</p>
    </div>
  );
}
