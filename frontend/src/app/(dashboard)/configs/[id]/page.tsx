'use client';

import { useEffect, useState, useCallback } from 'react';
import { useParams, useRouter } from 'next/navigation';
import toast from 'react-hot-toast';
import { api } from '@/lib/api';
import type { Config, DomainRule, IPRangeRule, ParentProxy, Proxy, ApiError } from '@/types';
import { StatusBadge } from '@/components/status-badge';
import { ConfirmDialog } from '@/components/confirm-dialog';
import { Loading } from '@/components/loading';
import { formatDate } from '@/lib/utils';

export default function ConfigDetailPage() {
  const params = useParams();
  const router = useRouter();
  const id = params.id as string;

  const [config, setConfig] = useState<Config | null>(null);
  const [loading, setLoading] = useState(true);
  const [editing, setEditing] = useState(false);
  const [saving, setSaving] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);
  const [confirmAction, setConfirmAction] = useState<string | null>(null);

  // Edit state
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [domains, setDomains] = useState<Omit<DomainRule, 'id'>[]>([]);
  const [ipRanges, setIpRanges] = useState<Omit<IPRangeRule, 'id'>[]>([]);
  const [parentProxies, setParentProxies] = useState<Omit<ParentProxy, 'id'>[]>([]);
  const [selectedProxyIds, setSelectedProxyIds] = useState<string[]>([]);
  const [availableProxies, setAvailableProxies] = useState<Proxy[]>([]);

  const load = useCallback(async () => {
    try {
      const data = await api.configs.get(id);
      setConfig(data);
      setName(data.name);
      setDescription(data.description || '');
      setDomains(
        (data.domains || []).map((d) => ({ domain: d.domain, action: d.action, priority: d.priority }))
      );
      setIpRanges(
        (data.ip_ranges || []).map((r) => ({ cidr: r.cidr, action: r.action, priority: r.priority }))
      );
      setParentProxies(
        (data.parent_proxies || []).map((p) => ({
          address: p.address,
          port: p.port,
          priority: p.priority,
          enabled: p.enabled,
        }))
      );
      setSelectedProxyIds((data.proxies || []).map((p) => p.id));
    } catch (err) {
      toast.error((err as ApiError).message || 'Erro ao carregar config');
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    load();
    api.proxies.list().then((res) => setAvailableProxies(res.data || [])).catch(() => {});
  }, [load]);

  async function handleSave() {
    setSaving(true);
    try {
      await api.configs.update(id, {
        name: name.trim(),
        description: description.trim() || undefined,
        domains,
        ip_ranges: ipRanges,
        parent_proxies: parentProxies,
        proxy_ids: selectedProxyIds,
      });
      toast.success('Config atualizada');
      setEditing(false);
      load();
    } catch (err) {
      toast.error((err as ApiError).message || 'Erro ao salvar');
    } finally {
      setSaving(false);
    }
  }

  async function handleAction(action: string) {
    setActionLoading(true);
    try {
      switch (action) {
        case 'submit':
          await api.configs.submit(id);
          toast.success('Config submetida para aprovação');
          break;
        case 'approve':
          await api.configs.approve(id);
          toast.success('Config aprovada e ativada');
          break;
        case 'reject':
          await api.configs.reject(id);
          toast.success('Config rejeitada');
          break;
      }
      load();
    } catch (err) {
      toast.error((err as ApiError).message || `Erro ao ${action}`);
    } finally {
      setActionLoading(false);
      setConfirmAction(null);
    }
  }

  if (loading) return <Loading />;
  if (!config) return <p className="text-gray-500">Config não encontrada.</p>;

  const isDraft = config.status === 'draft';
  const isPending = config.status === 'pending_approval';

  return (
    <div className="max-w-4xl">
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <h1 className="text-2xl font-bold text-gray-900">{config.name}</h1>
          <StatusBadge status={config.status} />
          <span className="text-sm text-gray-500">v{config.version}</span>
        </div>
        <div className="flex gap-2">
          {isDraft && !editing && (
            <>
              <button
                onClick={() => setEditing(true)}
                className="px-4 py-2 text-sm border rounded-md hover:bg-gray-50"
              >
                Editar
              </button>
              <button
                onClick={() => setConfirmAction('submit')}
                disabled={actionLoading}
                className="px-4 py-2 text-sm bg-yellow-500 text-white rounded-md hover:bg-yellow-600 disabled:opacity-50"
              >
                Submeter
              </button>
            </>
          )}
          {isPending && (
            <>
              <button
                onClick={() => setConfirmAction('approve')}
                disabled={actionLoading}
                className="px-4 py-2 text-sm bg-green-600 text-white rounded-md hover:bg-green-700 disabled:opacity-50"
              >
                Aprovar
              </button>
              <button
                onClick={() => setConfirmAction('reject')}
                disabled={actionLoading}
                className="px-4 py-2 text-sm bg-red-600 text-white rounded-md hover:bg-red-700 disabled:opacity-50"
              >
                Rejeitar
              </button>
            </>
          )}
        </div>
      </div>

      {config.description && !editing && (
        <p className="text-sm text-gray-600 mb-6">{config.description}</p>
      )}

      <div className="text-xs text-gray-500 mb-6 flex gap-4">
        <span>Modificado: {formatDate(config.modified_at)} por {config.modified_by?.username || '-'}</span>
        {config.approved_at && (
          <span>Aprovado: {formatDate(config.approved_at)} por {config.approved_by?.username || '-'}</span>
        )}
      </div>

      {editing ? (
        <EditForm
          name={name}
          setName={setName}
          description={description}
          setDescription={setDescription}
          domains={domains}
          setDomains={setDomains}
          ipRanges={ipRanges}
          setIpRanges={setIpRanges}
          parentProxies={parentProxies}
          setParentProxies={setParentProxies}
          selectedProxyIds={selectedProxyIds}
          setSelectedProxyIds={setSelectedProxyIds}
          availableProxies={availableProxies}
          saving={saving}
          onSave={handleSave}
          onCancel={() => {
            setEditing(false);
            load();
          }}
        />
      ) : (
        <ReadOnlyView config={config} />
      )}

      <ConfirmDialog
        open={confirmAction === 'submit'}
        title="Submeter Config"
        message="Deseja submeter esta configuração para aprovação?"
        confirmLabel="Submeter"
        onConfirm={() => handleAction('submit')}
        onCancel={() => setConfirmAction(null)}
      />
      <ConfirmDialog
        open={confirmAction === 'approve'}
        title="Aprovar Config"
        message="Ao aprovar, esta configuração será ativada e distribuída para os proxies associados."
        confirmLabel="Aprovar"
        onConfirm={() => handleAction('approve')}
        onCancel={() => setConfirmAction(null)}
      />
      <ConfirmDialog
        open={confirmAction === 'reject'}
        title="Rejeitar Config"
        message="A configuração voltará para o status de rascunho."
        confirmLabel="Rejeitar"
        variant="danger"
        onConfirm={() => handleAction('reject')}
        onCancel={() => setConfirmAction(null)}
      />
    </div>
  );
}

function ReadOnlyView({ config }: { config: Config }) {
  return (
    <div className="space-y-6">
      {/* Domain Rules */}
      <div className="bg-white rounded-lg border p-5">
        <h2 className="text-base font-semibold text-gray-900 mb-3">Regras de Domínio</h2>
        {!config.domains?.length ? (
          <p className="text-sm text-gray-500">Nenhuma regra.</p>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-gray-600">
                <th className="pb-2 font-medium">Domínio</th>
                <th className="pb-2 font-medium">Ação</th>
                <th className="pb-2 font-medium">Prioridade</th>
              </tr>
            </thead>
            <tbody className="divide-y">
              {config.domains.map((d, i) => (
                <tr key={i}>
                  <td className="py-2 font-mono text-xs">{d.domain}</td>
                  <td className="py-2">{d.action}</td>
                  <td className="py-2">{d.priority}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* IP Ranges */}
      <div className="bg-white rounded-lg border p-5">
        <h2 className="text-base font-semibold text-gray-900 mb-3">Regras de IP</h2>
        {!config.ip_ranges?.length ? (
          <p className="text-sm text-gray-500">Nenhuma regra.</p>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-gray-600">
                <th className="pb-2 font-medium">CIDR</th>
                <th className="pb-2 font-medium">Ação</th>
                <th className="pb-2 font-medium">Prioridade</th>
              </tr>
            </thead>
            <tbody className="divide-y">
              {config.ip_ranges.map((r, i) => (
                <tr key={i}>
                  <td className="py-2 font-mono text-xs">{r.cidr}</td>
                  <td className="py-2">{r.action}</td>
                  <td className="py-2">{r.priority}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Parent Proxies */}
      <div className="bg-white rounded-lg border p-5">
        <h2 className="text-base font-semibold text-gray-900 mb-3">Parent Proxies</h2>
        {!config.parent_proxies?.length ? (
          <p className="text-sm text-gray-500">Nenhum parent proxy.</p>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-gray-600">
                <th className="pb-2 font-medium">Endereço</th>
                <th className="pb-2 font-medium">Porta</th>
                <th className="pb-2 font-medium">Prioridade</th>
                <th className="pb-2 font-medium">Status</th>
              </tr>
            </thead>
            <tbody className="divide-y">
              {config.parent_proxies.map((p, i) => (
                <tr key={i}>
                  <td className="py-2 font-mono text-xs">{p.address}</td>
                  <td className="py-2">{p.port}</td>
                  <td className="py-2">{p.priority}</td>
                  <td className="py-2">
                    <span className={`text-xs ${p.enabled ? 'text-green-600' : 'text-red-500'}`}>
                      {p.enabled ? 'Ativo' : 'Inativo'}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Assigned Proxies */}
      <div className="bg-white rounded-lg border p-5">
        <h2 className="text-base font-semibold text-gray-900 mb-3">Proxies Associados</h2>
        {!config.proxies?.length ? (
          <p className="text-sm text-gray-500">Nenhum proxy associado.</p>
        ) : (
          <div className="flex flex-wrap gap-2">
            {config.proxies.map((p) => (
              <span
                key={p.id}
                className="inline-flex items-center gap-1.5 px-3 py-1 bg-gray-100 rounded-full text-sm"
              >
                <span className={`w-2 h-2 rounded-full ${p.is_online ? 'bg-green-500' : 'bg-red-500'}`} />
                {p.hostname}
              </span>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function EditForm({
  name, setName,
  description, setDescription,
  domains, setDomains,
  ipRanges, setIpRanges,
  parentProxies, setParentProxies,
  selectedProxyIds, setSelectedProxyIds,
  availableProxies,
  saving, onSave, onCancel,
}: {
  name: string; setName: (v: string) => void;
  description: string; setDescription: (v: string) => void;
  domains: Omit<DomainRule, 'id'>[]; setDomains: (v: Omit<DomainRule, 'id'>[]) => void;
  ipRanges: Omit<IPRangeRule, 'id'>[]; setIpRanges: (v: Omit<IPRangeRule, 'id'>[]) => void;
  parentProxies: Omit<ParentProxy, 'id'>[]; setParentProxies: (v: Omit<ParentProxy, 'id'>[]) => void;
  selectedProxyIds: string[]; setSelectedProxyIds: (v: string[]) => void;
  availableProxies: Proxy[];
  saving: boolean; onSave: () => void; onCancel: () => void;
}) {
  return (
    <div className="space-y-6">
      <div className="bg-white rounded-lg border p-5 space-y-4">
        <h2 className="text-base font-semibold text-gray-900">Informações Gerais</h2>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Nome</label>
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            className="w-full px-3 py-2 border border-gray-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Descrição</label>
          <textarea
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            className="w-full px-3 py-2 border border-gray-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            rows={2}
          />
        </div>
      </div>

      {/* Domain Rules */}
      <div className="bg-white rounded-lg border p-5">
        <div className="flex items-center justify-between mb-3">
          <h2 className="text-base font-semibold text-gray-900">Regras de Domínio</h2>
          <button
            type="button"
            onClick={() => setDomains([...domains, { domain: '', action: 'direct', priority: (domains.length + 1) * 10 }])}
            className="px-3 py-1 text-sm text-blue-600 border border-blue-300 rounded-md hover:bg-blue-50"
          >
            + Adicionar
          </button>
        </div>
        <div className="space-y-2">
          {domains.map((d, i) => (
            <div key={i} className="flex gap-2 items-center">
              <input
                value={d.domain}
                onChange={(e) => {
                  const next = [...domains];
                  next[i] = { ...next[i], domain: e.target.value };
                  setDomains(next);
                }}
                className="flex-1 px-3 py-2 border border-gray-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                placeholder=".example.com"
              />
              <select
                value={d.action}
                onChange={(e) => {
                  const next = [...domains];
                  next[i] = { ...next[i], action: e.target.value as 'direct' | 'parent' };
                  setDomains(next);
                }}
                className="w-32 px-3 py-2 border border-gray-300 rounded-md text-sm"
              >
                <option value="direct">Direct</option>
                <option value="parent">Parent</option>
              </select>
              <input
                type="number"
                value={d.priority}
                onChange={(e) => {
                  const next = [...domains];
                  next[i] = { ...next[i], priority: parseInt(e.target.value) || 0 };
                  setDomains(next);
                }}
                className="w-24 px-3 py-2 border border-gray-300 rounded-md text-sm"
              />
              <button
                onClick={() => setDomains(domains.filter((_, idx) => idx !== i))}
                className="w-8 h-8 flex items-center justify-center text-red-500 hover:bg-red-50 rounded text-lg"
              >
                &times;
              </button>
            </div>
          ))}
        </div>
      </div>

      {/* IP Ranges */}
      <div className="bg-white rounded-lg border p-5">
        <div className="flex items-center justify-between mb-3">
          <h2 className="text-base font-semibold text-gray-900">Regras de IP</h2>
          <button
            type="button"
            onClick={() => setIpRanges([...ipRanges, { cidr: '', action: 'direct', priority: (ipRanges.length + 1) * 10 }])}
            className="px-3 py-1 text-sm text-blue-600 border border-blue-300 rounded-md hover:bg-blue-50"
          >
            + Adicionar
          </button>
        </div>
        <div className="space-y-2">
          {ipRanges.map((r, i) => (
            <div key={i} className="flex gap-2 items-center">
              <input
                value={r.cidr}
                onChange={(e) => {
                  const next = [...ipRanges];
                  next[i] = { ...next[i], cidr: e.target.value };
                  setIpRanges(next);
                }}
                className="flex-1 px-3 py-2 border border-gray-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                placeholder="10.0.0.0/8"
              />
              <select
                value={r.action}
                onChange={(e) => {
                  const next = [...ipRanges];
                  next[i] = { ...next[i], action: e.target.value as 'direct' | 'parent' };
                  setIpRanges(next);
                }}
                className="w-32 px-3 py-2 border border-gray-300 rounded-md text-sm"
              >
                <option value="direct">Direct</option>
                <option value="parent">Parent</option>
              </select>
              <input
                type="number"
                value={r.priority}
                onChange={(e) => {
                  const next = [...ipRanges];
                  next[i] = { ...next[i], priority: parseInt(e.target.value) || 0 };
                  setIpRanges(next);
                }}
                className="w-24 px-3 py-2 border border-gray-300 rounded-md text-sm"
              />
              <button
                onClick={() => setIpRanges(ipRanges.filter((_, idx) => idx !== i))}
                className="w-8 h-8 flex items-center justify-center text-red-500 hover:bg-red-50 rounded text-lg"
              >
                &times;
              </button>
            </div>
          ))}
        </div>
      </div>

      {/* Parent Proxies */}
      <div className="bg-white rounded-lg border p-5">
        <div className="flex items-center justify-between mb-3">
          <h2 className="text-base font-semibold text-gray-900">Parent Proxies</h2>
          <button
            type="button"
            onClick={() =>
              setParentProxies([
                ...parentProxies,
                { address: '', port: 3128, priority: parentProxies.length + 1, enabled: true },
              ])
            }
            className="px-3 py-1 text-sm text-blue-600 border border-blue-300 rounded-md hover:bg-blue-50"
          >
            + Adicionar
          </button>
        </div>
        <div className="space-y-2">
          {parentProxies.map((p, i) => (
            <div key={i} className="flex gap-2 items-center">
              <input
                value={p.address}
                onChange={(e) => {
                  const next = [...parentProxies];
                  next[i] = { ...next[i], address: e.target.value };
                  setParentProxies(next);
                }}
                className="flex-1 px-3 py-2 border border-gray-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                placeholder="10.96.215.26"
              />
              <input
                type="number"
                value={p.port}
                onChange={(e) => {
                  const next = [...parentProxies];
                  next[i] = { ...next[i], port: parseInt(e.target.value) || 0 };
                  setParentProxies(next);
                }}
                className="w-24 px-3 py-2 border border-gray-300 rounded-md text-sm"
              />
              <input
                type="number"
                value={p.priority}
                onChange={(e) => {
                  const next = [...parentProxies];
                  next[i] = { ...next[i], priority: parseInt(e.target.value) || 0 };
                  setParentProxies(next);
                }}
                className="w-24 px-3 py-2 border border-gray-300 rounded-md text-sm"
              />
              <label className="flex items-center gap-1 text-sm whitespace-nowrap">
                <input
                  type="checkbox"
                  checked={p.enabled}
                  onChange={(e) => {
                    const next = [...parentProxies];
                    next[i] = { ...next[i], enabled: e.target.checked };
                    setParentProxies(next);
                  }}
                  className="rounded"
                />
                Ativo
              </label>
              <button
                onClick={() => setParentProxies(parentProxies.filter((_, idx) => idx !== i))}
                className="w-8 h-8 flex items-center justify-center text-red-500 hover:bg-red-50 rounded text-lg"
              >
                &times;
              </button>
            </div>
          ))}
        </div>
      </div>

      {/* Proxy Assignment */}
      <div className="bg-white rounded-lg border p-5">
        <h2 className="text-base font-semibold text-gray-900 mb-3">Proxies Associados</h2>
        {availableProxies.length === 0 ? (
          <p className="text-sm text-gray-500">Nenhum proxy disponível.</p>
        ) : (
          <div className="space-y-1">
            {availableProxies.map((proxy) => (
              <label key={proxy.id} className="flex items-center gap-2 p-2 rounded hover:bg-gray-50 cursor-pointer">
                <input
                  type="checkbox"
                  checked={selectedProxyIds.includes(proxy.id)}
                  onChange={() =>
                    setSelectedProxyIds(
                      selectedProxyIds.includes(proxy.id)
                        ? selectedProxyIds.filter((p) => p !== proxy.id)
                        : [...selectedProxyIds, proxy.id]
                    )
                  }
                  className="rounded"
                />
                <span className="text-sm">{proxy.hostname}</span>
                <span className={`w-2 h-2 rounded-full ${proxy.is_online ? 'bg-green-500' : 'bg-red-500'}`} />
              </label>
            ))}
          </div>
        )}
      </div>

      <div className="flex gap-3">
        <button
          onClick={onSave}
          disabled={saving}
          className="px-6 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700 disabled:opacity-50"
        >
          {saving ? 'Salvando...' : 'Salvar'}
        </button>
        <button
          onClick={onCancel}
          className="px-6 py-2 border text-sm rounded-md hover:bg-gray-50"
        >
          Cancelar
        </button>
      </div>
    </div>
  );
}
