'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import toast from 'react-hot-toast';
import { api } from '@/lib/api';
import type { Proxy, ApiError } from '@/types';
import { StatusBadge } from '@/components/status-badge';
import { EmptyState } from '@/components/empty-state';
import { TableSkeleton } from '@/components/loading';
import { ConfirmDialog } from '@/components/confirm-dialog';
import { formatRelative, formatBytes } from '@/lib/utils';

export default function ProxiesPage() {
  const [proxies, setProxies] = useState<Proxy[]>([]);
  const [summary, setSummary] = useState({ total: 0, online: 0, offline: 0 });
  const [loading, setLoading] = useState(true);
  const [deleteTarget, setDeleteTarget] = useState<Proxy | null>(null);
  const [deleting, setDeleting] = useState(false);

  useEffect(() => {
    async function load() {
      try {
        const res = await api.proxies.list();
        setProxies(res.data || []);
        setSummary(res.summary || { total: 0, online: 0, offline: 0 });
      } catch (err) {
        toast.error((err as ApiError).message || 'Erro ao carregar proxies');
      } finally {
        setLoading(false);
      }
    }
    load();
    const interval = setInterval(load, 15000);
    return () => clearInterval(interval);
  }, []);

  async function handleDelete() {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      await api.proxies.delete(deleteTarget.id);
      setProxies((prev) => prev.filter((p) => p.id !== deleteTarget.id));
      setSummary((prev) => ({
        total: prev.total - 1,
        online: deleteTarget.is_online ? prev.online - 1 : prev.online,
        offline: deleteTarget.is_online ? prev.offline : prev.offline - 1,
      }));
      toast.success(`Proxy ${deleteTarget.hostname} removido`);
    } catch (err) {
      toast.error((err as ApiError).message || 'Erro ao remover proxy');
    } finally {
      setDeleting(false);
      setDeleteTarget(null);
    }
  }

  return (
    <div>
      <h1 className="text-2xl font-bold text-gray-900 mb-6">Proxies</h1>

      <div className="flex gap-4 mb-6">
        <div className="bg-white rounded-lg border px-4 py-3">
          <span className="text-sm text-gray-500">Total</span>
          <p className="text-xl font-bold">{summary.total}</p>
        </div>
        <div className="bg-green-50 rounded-lg border border-green-200 px-4 py-3">
          <span className="text-sm text-green-700">Online</span>
          <p className="text-xl font-bold text-green-800">{summary.online}</p>
        </div>
        <div className="bg-red-50 rounded-lg border border-red-200 px-4 py-3">
          <span className="text-sm text-red-700">Offline</span>
          <p className="text-xl font-bold text-red-800">{summary.offline}</p>
        </div>
      </div>

      {loading ? (
        <TableSkeleton />
      ) : proxies.length === 0 ? (
        <EmptyState
          title="Nenhum proxy registrado"
          description="Proxies serão registrados automaticamente quando o helper se conectar."
        />
      ) : (
        <div className="bg-white rounded-lg border overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-gray-50 border-b">
              <tr>
                <th className="text-left px-4 py-3 font-medium text-gray-600">Hostname</th>
                <th className="text-left px-4 py-3 font-medium text-gray-600">Status</th>
                <th className="text-left px-4 py-3 font-medium text-gray-600">Config</th>
                <th className="text-right px-4 py-3 font-medium text-gray-600">Requests (1h)</th>
                <th className="text-right px-4 py-3 font-medium text-gray-600">Erros (1h)</th>
                <th className="text-right px-4 py-3 font-medium text-gray-600">Tráfego (1h)</th>
                <th className="text-right px-4 py-3 font-medium text-gray-600">Cache Hit</th>
                <th className="text-left px-4 py-3 font-medium text-gray-600">Último Sinal</th>
                <th className="text-right px-4 py-3 font-medium text-gray-600">Ações</th>
              </tr>
            </thead>
            <tbody className="divide-y">
              {proxies.map((proxy) => {
                const s = proxy.stats;
                const errRate = s && s.total_requests_1h > 0
                  ? (s.errors_1h / s.total_requests_1h) * 100
                  : 0;
                return (
                  <tr key={proxy.id} className="hover:bg-gray-50">
                    <td className="px-4 py-3">
                      <Link href={`/proxies/${proxy.id}`} className="text-blue-600 hover:underline font-medium">
                        {proxy.hostname}
                      </Link>
                    </td>
                    <td className="px-4 py-3">
                      <StatusBadge status={proxy.is_online ? 'online' : 'offline'} />
                    </td>
                    <td className="px-4 py-3 text-gray-600">
                      {proxy.config ? (
                        <div className="flex items-center gap-2">
                          <Link href={`/configs/${proxy.config.id}`} className="text-blue-600 hover:underline">
                            {proxy.config.name}
                          </Link>
                          <span className="text-xs text-gray-400">v{proxy.config.version}</span>
                          {proxy.config.in_sync ? (
                            <span className="w-1.5 h-1.5 bg-green-500 rounded-full" title="Sincronizado" />
                          ) : (
                            <span className="w-1.5 h-1.5 bg-yellow-500 rounded-full animate-pulse" title="Aguardando sync" />
                          )}
                        </div>
                      ) : (
                        '-'
                      )}
                    </td>
                    <td className="px-4 py-3 text-right font-mono">
                      {s?.total_requests_1h != null ? s.total_requests_1h.toLocaleString() : '-'}
                    </td>
                    <td className="px-4 py-3 text-right">
                      {s?.errors_1h != null ? (
                        <span className={errRate > 0 ? 'text-red-600 font-medium' : 'text-gray-500'}>
                          {s.errors_1h} {errRate > 0 && `(${errRate.toFixed(1)}%)`}
                        </span>
                      ) : '-'}
                    </td>
                    <td className="px-4 py-3 text-right text-gray-600">
                      {s ? (
                        <span title={`In: ${formatBytes(s.bytes_in_1h)} / Out: ${formatBytes(s.bytes_out_1h)}`}>
                          {formatBytes((s.bytes_in_1h || 0) + (s.bytes_out_1h || 0))}
                        </span>
                      ) : '-'}
                    </td>
                    <td className="px-4 py-3 text-right text-gray-600">
                      {s?.cache_hit_rate != null
                        ? `${(s.cache_hit_rate * 100).toFixed(1)}%`
                        : '-'}
                    </td>
                    <td className="px-4 py-3 text-gray-500">{formatRelative(proxy.last_seen)}</td>
                    <td className="px-4 py-3 text-right">
                      {!proxy.is_online && (
                        <button
                          onClick={() => setDeleteTarget(proxy)}
                          className="text-red-600 hover:text-red-800 text-xs font-medium"
                          title="Remover proxy"
                        >
                          Remover
                        </button>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      <ConfirmDialog
        open={!!deleteTarget}
        title="Remover proxy"
        message={`Tem certeza que deseja remover o proxy "${deleteTarget?.hostname}"? Estatísticas e logs associados também serão removidos.`}
        confirmLabel={deleting ? 'Removendo...' : 'Remover'}
        variant="danger"
        onConfirm={handleDelete}
        onCancel={() => setDeleteTarget(null)}
      />
    </div>
  );
}
