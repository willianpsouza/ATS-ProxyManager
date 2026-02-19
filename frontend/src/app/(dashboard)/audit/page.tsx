'use client';

import { useEffect, useState } from 'react';
import toast from 'react-hot-toast';
import { api } from '@/lib/api';
import type { AuditLog, PaginatedResponse, ApiError } from '@/types';
import { Pagination } from '@/components/pagination';
import { TableSkeleton } from '@/components/loading';
import { EmptyState } from '@/components/empty-state';
import { formatDate } from '@/lib/utils';

const entityTypes = [
  { label: 'Todos', value: '' },
  { label: 'User', value: 'user' },
  { label: 'Config', value: 'config' },
  { label: 'Proxy', value: 'proxy' },
];

export default function AuditPage() {
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [pagination, setPagination] = useState({ page: 1, limit: 20, total: 0, total_pages: 0 });
  const [loading, setLoading] = useState(true);
  const [entityType, setEntityType] = useState('');
  const [dateFrom, setDateFrom] = useState('');
  const [dateTo, setDateTo] = useState('');

  async function load(page = 1) {
    setLoading(true);
    try {
      const res: PaginatedResponse<AuditLog> = await api.audit.list({
        entity_type: entityType || undefined,
        from: dateFrom || undefined,
        to: dateTo || undefined,
        page,
      });
      setLogs(res.data || []);
      setPagination(res.pagination);
    } catch (err) {
      toast.error((err as ApiError).message || 'Erro ao carregar audit');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load(1);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [entityType, dateFrom, dateTo]);

  return (
    <div>
      <h1 className="text-2xl font-bold text-gray-900 mb-6">Auditoria</h1>

      <div className="flex flex-wrap gap-4 mb-4 items-end">
        <div className="flex gap-2">
          {entityTypes.map((f) => (
            <button
              key={f.value}
              onClick={() => setEntityType(f.value)}
              className={`px-3 py-1.5 text-sm rounded-md border transition-colors ${
                entityType === f.value
                  ? 'bg-blue-50 border-blue-300 text-blue-700'
                  : 'border-gray-300 text-gray-600 hover:bg-gray-50'
              }`}
            >
              {f.label}
            </button>
          ))}
        </div>
        <div className="flex gap-2 items-center">
          <label className="text-sm text-gray-600">De:</label>
          <input
            type="date"
            value={dateFrom}
            onChange={(e) => setDateFrom(e.target.value)}
            className="px-3 py-1.5 border border-gray-300 rounded-md text-sm"
          />
          <label className="text-sm text-gray-600">Até:</label>
          <input
            type="date"
            value={dateTo}
            onChange={(e) => setDateTo(e.target.value)}
            className="px-3 py-1.5 border border-gray-300 rounded-md text-sm"
          />
        </div>
      </div>

      {loading ? (
        <TableSkeleton cols={5} />
      ) : logs.length === 0 ? (
        <EmptyState title="Nenhum registro de auditoria" />
      ) : (
        <>
          <div className="bg-white rounded-lg border overflow-hidden">
            <table className="w-full text-sm">
              <thead className="bg-gray-50 border-b">
                <tr>
                  <th className="text-left px-4 py-3 font-medium text-gray-600">Data</th>
                  <th className="text-left px-4 py-3 font-medium text-gray-600">Usuário</th>
                  <th className="text-left px-4 py-3 font-medium text-gray-600">Ação</th>
                  <th className="text-left px-4 py-3 font-medium text-gray-600">Entidade</th>
                  <th className="text-left px-4 py-3 font-medium text-gray-600">Alterações</th>
                  <th className="text-left px-4 py-3 font-medium text-gray-600">IP</th>
                </tr>
              </thead>
              <tbody className="divide-y">
                {logs.map((log) => (
                  <tr key={log.id} className="hover:bg-gray-50">
                    <td className="px-4 py-3 text-gray-600 whitespace-nowrap">{formatDate(log.created_at)}</td>
                    <td className="px-4 py-3">{log.user?.username || '-'}</td>
                    <td className="px-4 py-3">
                      <span className="inline-flex px-2 py-0.5 rounded bg-gray-100 text-xs font-mono">
                        {log.action}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-gray-600">
                      <span className="capitalize">{log.entity_type}</span>
                      {log.entity_id && (
                        <span className="text-xs text-gray-400 ml-1 font-mono">
                          {log.entity_id.slice(0, 8)}
                        </span>
                      )}
                    </td>
                    <td className="px-4 py-3">
                      {log.changes ? (
                        <div className="space-y-0.5">
                          {Object.entries(log.changes).map(([key, val]) => (
                            <div key={key} className="text-xs">
                              <span className="text-gray-500">{key}:</span>{' '}
                              <span className="text-red-500 line-through">{val.old}</span>{' '}
                              <span className="text-green-600">{val.new}</span>
                            </div>
                          ))}
                        </div>
                      ) : (
                        <span className="text-gray-400 text-xs">-</span>
                      )}
                    </td>
                    <td className="px-4 py-3 text-gray-500 text-xs font-mono">{log.ip_address || '-'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <Pagination pagination={pagination} onPageChange={load} />
        </>
      )}
    </div>
  );
}
