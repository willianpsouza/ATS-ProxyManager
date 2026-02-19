'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import toast from 'react-hot-toast';
import { api } from '@/lib/api';
import type { Config, PaginatedResponse, ConfigStatus, ApiError } from '@/types';
import { StatusBadge } from '@/components/status-badge';
import { Pagination } from '@/components/pagination';
import { EmptyState } from '@/components/empty-state';
import { TableSkeleton } from '@/components/loading';
import { formatDate } from '@/lib/utils';

const statusFilters: { label: string; value: string }[] = [
  { label: 'Todas', value: '' },
  { label: 'Rascunho', value: 'draft' },
  { label: 'Pendente', value: 'pending_approval' },
  { label: 'Ativa', value: 'active' },
];

export default function ConfigsPage() {
  const [configs, setConfigs] = useState<Config[]>([]);
  const [pagination, setPagination] = useState({ page: 1, limit: 20, total: 0, total_pages: 0 });
  const [statusFilter, setStatusFilter] = useState('');
  const [loading, setLoading] = useState(true);

  async function load(page = 1) {
    setLoading(true);
    try {
      const res: PaginatedResponse<Config> = await api.configs.list({
        status: statusFilter || undefined,
        page,
      });
      setConfigs(res.data || []);
      setPagination(res.pagination);
    } catch (err) {
      toast.error((err as ApiError).message || 'Erro ao carregar configs');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load(1);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [statusFilter]);

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Configurações</h1>
        <Link
          href="/configs/new"
          className="px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700 transition-colors"
        >
          Nova Config
        </Link>
      </div>

      <div className="flex gap-2 mb-4">
        {statusFilters.map((f) => (
          <button
            key={f.value}
            onClick={() => setStatusFilter(f.value)}
            className={`px-3 py-1.5 text-sm rounded-md border transition-colors ${
              statusFilter === f.value
                ? 'bg-blue-50 border-blue-300 text-blue-700'
                : 'border-gray-300 text-gray-600 hover:bg-gray-50'
            }`}
          >
            {f.label}
          </button>
        ))}
      </div>

      {loading ? (
        <TableSkeleton />
      ) : configs.length === 0 ? (
        <EmptyState
          title="Nenhuma configuração encontrada"
          description="Crie uma nova configuração para começar."
          action={
            <Link
              href="/configs/new"
              className="px-4 py-2 bg-blue-600 text-white text-sm rounded-md hover:bg-blue-700"
            >
              Nova Config
            </Link>
          }
        />
      ) : (
        <>
          <div className="bg-white rounded-lg border overflow-hidden">
            <table className="w-full text-sm">
              <thead className="bg-gray-50 border-b">
                <tr>
                  <th className="text-left px-4 py-3 font-medium text-gray-600">Nome</th>
                  <th className="text-left px-4 py-3 font-medium text-gray-600">Status</th>
                  <th className="text-left px-4 py-3 font-medium text-gray-600">Versão</th>
                  <th className="text-left px-4 py-3 font-medium text-gray-600">Proxies</th>
                  <th className="text-left px-4 py-3 font-medium text-gray-600">Modificado</th>
                  <th className="text-left px-4 py-3 font-medium text-gray-600">Por</th>
                </tr>
              </thead>
              <tbody className="divide-y">
                {configs.map((config) => (
                  <tr key={config.id} className="hover:bg-gray-50">
                    <td className="px-4 py-3">
                      <Link href={`/configs/${config.id}`} className="text-blue-600 hover:underline font-medium">
                        {config.name}
                      </Link>
                    </td>
                    <td className="px-4 py-3">
                      <StatusBadge status={config.status} />
                    </td>
                    <td className="px-4 py-3 text-gray-600">v{config.version}</td>
                    <td className="px-4 py-3 text-gray-600">{config.proxy_count ?? 0}</td>
                    <td className="px-4 py-3 text-gray-600">{formatDate(config.modified_at)}</td>
                    <td className="px-4 py-3 text-gray-600">{config.modified_by?.username || '-'}</td>
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
