'use client';

import { useEffect, useState, useCallback } from 'react';
import { useParams, useRouter } from 'next/navigation';
import toast from 'react-hot-toast';
import { api } from '@/lib/api';
import type { Config, ConfigPreview, RuleAction, DomainRule, IPRangeRule, ParentProxy, ClientACLRule, Proxy, ApiError } from '@/types';
import { StatusBadge } from '@/components/status-badge';
import { ConfirmDialog } from '@/components/confirm-dialog';
import { Loading } from '@/components/loading';
import { formatDate } from '@/lib/utils';

const DOMAIN_RE = /^(\*\.)?[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?)+$/;
const IPV4_RE = /^(\d{1,3}\.){3}\d{1,3}$/;
const CIDR_RE = /^(\d{1,3}\.){3}\d{1,3}\/\d{1,2}$/;

function isValidDomain(d: string): boolean {
  if (!d || d === '*.' || d === '*') return false;
  return DOMAIN_RE.test(d);
}

function isValidCIDR(c: string): boolean {
  if (!c) return false;
  if (c === '0.0.0.0' || c.startsWith('0.0.0.0/')) return false;
  return IPV4_RE.test(c) || CIDR_RE.test(c) || c === '::1' || c.includes(':');
}

function isValidIPv4(addr: string): boolean {
  if (!addr) return false;
  return IPV4_RE.test(addr);
}

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
  const [cloning, setCloning] = useState(false);

  // Edit state
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [defaultAction, setDefaultAction] = useState<RuleAction>('direct');
  const [domains, setDomains] = useState<Omit<DomainRule, 'id'>[]>([]);
  const [ipRanges, setIpRanges] = useState<Omit<IPRangeRule, 'id'>[]>([]);
  const [parentProxies, setParentProxies] = useState<Omit<ParentProxy, 'id'>[]>([]);
  const [clientACL, setClientACL] = useState<Omit<ClientACLRule, 'id'>[]>([]);
  const [selectedProxyIds, setSelectedProxyIds] = useState<string[]>([]);
  const [availableProxies, setAvailableProxies] = useState<Proxy[]>([]);

  const load = useCallback(async () => {
    try {
      const data = await api.configs.get(id);
      setConfig(data);
      setName(data.name);
      setDescription(data.description || '');
      setDefaultAction(data.default_action || 'direct');
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
      setClientACL(
        (data.client_acl || []).map((a) => ({ cidr: a.cidr, action: a.action, priority: a.priority }))
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
        default_action: defaultAction,
        domains,
        ip_ranges: ipRanges,
        parent_proxies: parentProxies,
        client_acl: clientACL,
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

  async function handleClone() {
    setCloning(true);
    try {
      const cloned = await api.configs.clone(id);
      toast.success(`Nova versão v${cloned.version} criada como rascunho`);
      router.push(`/configs/${cloned.id}`);
    } catch (err) {
      toast.error((err as ApiError).message || 'Erro ao clonar config');
    } finally {
      setCloning(false);
    }
  }

  if (loading) return <Loading />;
  if (!config) return <p className="text-gray-500">Config não encontrada.</p>;

  const isDraft = config.status === 'draft';
  const isPending = config.status === 'pending_approval';
  const canClone = config.status === 'active' || config.status === 'approved';

  return (
    <div className="max-w-4xl">
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <h1 className="text-2xl font-bold text-gray-900">{config.name}</h1>
          <StatusBadge status={config.status} />
          <span className="text-sm text-gray-500">v{config.version}</span>
        </div>
        <div className="flex gap-2">
          {canClone && (
            <button
              onClick={handleClone}
              disabled={cloning}
              className="px-4 py-2 text-sm bg-indigo-600 text-white rounded-md hover:bg-indigo-700 disabled:opacity-50 transition-colors"
            >
              {cloning ? 'Clonando...' : 'Nova Versão'}
            </button>
          )}
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
          defaultAction={defaultAction}
          setDefaultAction={setDefaultAction}
          domains={domains}
          setDomains={setDomains}
          ipRanges={ipRanges}
          setIpRanges={setIpRanges}
          parentProxies={parentProxies}
          setParentProxies={setParentProxies}
          clientACL={clientACL}
          setClientACL={setClientACL}
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
  const [preview, setPreview] = useState<ConfigPreview | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [previewOpen, setPreviewOpen] = useState(false);

  async function loadPreview() {
    if (preview) {
      setPreviewOpen(!previewOpen);
      return;
    }
    setPreviewLoading(true);
    try {
      const data = await api.configs.preview(config.id);
      setPreview(data);
      setPreviewOpen(true);
    } catch (err) {
      toast.error((err as ApiError).message || 'Erro ao carregar preview');
    } finally {
      setPreviewLoading(false);
    }
  }

  return (
    <div className="space-y-6">
      {/* Default Action */}
      <div className="bg-white rounded-lg border p-5">
        <h2 className="text-base font-semibold text-gray-900 mb-3">Comportamento Padrão</h2>
        <p className="text-sm text-gray-700">
          Tráfego sem regra específica:{' '}
          <span className="font-medium">
            {config.default_action === 'parent' ? 'Parent Proxy' : 'Direct Connect'}
          </span>
        </p>
      </div>

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

      {/* Client ACL */}
      <div className="bg-white rounded-lg border p-5">
        <h2 className="text-base font-semibold text-gray-900 mb-3">ACL de Clientes (ip_allow)</h2>
        {!config.client_acl?.length ? (
          <p className="text-sm text-gray-500">Nenhuma regra de ACL.</p>
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
              {config.client_acl.map((acl, i) => (
                <tr key={i}>
                  <td className="py-2 font-mono text-xs">{acl.cidr}</td>
                  <td className="py-2">
                    <span className={`text-xs font-medium ${acl.action === 'allow' ? 'text-green-600' : 'text-red-500'}`}>
                      {acl.action}
                    </span>
                  </td>
                  <td className="py-2">{acl.priority}</td>
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

      {/* Config File Preview */}
      <div className="bg-white rounded-lg border p-5">
        <div className="flex items-center justify-between mb-3">
          <h2 className="text-base font-semibold text-gray-900">Preview dos Arquivos</h2>
          <button
            onClick={loadPreview}
            disabled={previewLoading}
            className="px-3 py-1 text-sm text-blue-600 border border-blue-300 rounded-md hover:bg-blue-50 transition-colors disabled:opacity-50"
          >
            {previewLoading ? 'Carregando...' : previewOpen ? 'Ocultar' : 'Visualizar'}
          </button>
        </div>
        <p className="text-xs text-gray-500 mb-3">
          Mostra como os arquivos de configuração seriam gerados para o ATS.
        </p>
        {previewOpen && preview && (
          <div className="space-y-4">
            <div>
              <h3 className="text-sm font-medium text-gray-700 mb-1">parent.config</h3>
              <pre className="bg-gray-900 text-green-400 text-xs p-4 rounded-md overflow-x-auto whitespace-pre-wrap font-mono">
                {preview.parent_config || '(vazio)'}
              </pre>
            </div>
            <div>
              <h3 className="text-sm font-medium text-gray-700 mb-1">sni.yaml</h3>
              <pre className="bg-gray-900 text-green-400 text-xs p-4 rounded-md overflow-x-auto whitespace-pre-wrap font-mono">
                {preview.sni_yaml || '(vazio)'}
              </pre>
            </div>
            <div>
              <h3 className="text-sm font-medium text-gray-700 mb-1">ip_allow.yaml</h3>
              <pre className="bg-gray-900 text-green-400 text-xs p-4 rounded-md overflow-x-auto whitespace-pre-wrap font-mono">
                {preview.ip_allow_yaml || '(vazio)'}
              </pre>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function EditForm({
  name, setName,
  description, setDescription,
  defaultAction, setDefaultAction,
  domains, setDomains,
  ipRanges, setIpRanges,
  parentProxies, setParentProxies,
  clientACL, setClientACL,
  selectedProxyIds, setSelectedProxyIds,
  availableProxies,
  saving, onSave, onCancel,
}: {
  name: string; setName: (v: string) => void;
  description: string; setDescription: (v: string) => void;
  defaultAction: RuleAction; setDefaultAction: (v: RuleAction) => void;
  domains: Omit<DomainRule, 'id'>[]; setDomains: (v: Omit<DomainRule, 'id'>[]) => void;
  ipRanges: Omit<IPRangeRule, 'id'>[]; setIpRanges: (v: Omit<IPRangeRule, 'id'>[]) => void;
  parentProxies: Omit<ParentProxy, 'id'>[]; setParentProxies: (v: Omit<ParentProxy, 'id'>[]) => void;
  clientACL: Omit<ClientACLRule, 'id'>[]; setClientACL: (v: Omit<ClientACLRule, 'id'>[]) => void;
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
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Comportamento Padrão</label>
          <select
            value={defaultAction}
            onChange={(e) => setDefaultAction(e.target.value as RuleAction)}
            className="w-full px-3 py-2 border border-gray-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
          >
            <option value="direct">Direct Connect</option>
            <option value="parent">Parent Proxy</option>
          </select>
          <p className="text-xs text-gray-500 mt-1">
            Define o que acontece com tráfego que não corresponde a nenhuma regra específica.
          </p>
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
            <div key={i} className="flex gap-2 items-start">
              <div className="flex-1">
                <input
                  value={d.domain}
                  onChange={(e) => {
                    const next = [...domains];
                    next[i] = { ...next[i], domain: e.target.value };
                    setDomains(next);
                  }}
                  className={`w-full px-3 py-2 border rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 ${d.domain && !isValidDomain(d.domain) ? 'border-red-500 bg-red-50' : 'border-gray-300'}`}
                  placeholder="*.example.com"
                />
                {d.domain && !isValidDomain(d.domain) && (
                  <p className="text-xs text-red-500 mt-0.5">Formato inválido (use *.example.com ou host.example.com)</p>
                )}
              </div>
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
                className="w-8 h-8 flex items-center justify-center text-red-500 hover:bg-red-50 rounded text-lg mt-1"
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
            <div key={i} className="flex gap-2 items-start">
              <div className="flex-1">
                <input
                  value={r.cidr}
                  onChange={(e) => {
                    const next = [...ipRanges];
                    next[i] = { ...next[i], cidr: e.target.value };
                    setIpRanges(next);
                  }}
                  className={`w-full px-3 py-2 border rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 ${r.cidr && !isValidCIDR(r.cidr) ? 'border-red-500 bg-red-50' : 'border-gray-300'}`}
                  placeholder="10.0.0.0/8"
                />
                {r.cidr && !isValidCIDR(r.cidr) && (
                  <p className="text-xs text-red-500 mt-0.5">CIDR/IP inválido ou 0.0.0.0 não permitido</p>
                )}
              </div>
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
                className="w-8 h-8 flex items-center justify-center text-red-500 hover:bg-red-50 rounded text-lg mt-1"
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
            <div key={i} className="flex gap-2 items-start">
              <div className="flex-1">
                <input
                  value={p.address}
                  onChange={(e) => {
                    const next = [...parentProxies];
                    next[i] = { ...next[i], address: e.target.value };
                    setParentProxies(next);
                  }}
                  className={`w-full px-3 py-2 border rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 ${p.address && !isValidIPv4(p.address) ? 'border-red-500 bg-red-50' : 'border-gray-300'}`}
                  placeholder="10.96.215.26"
                />
                {p.address && !isValidIPv4(p.address) && (
                  <p className="text-xs text-red-500 mt-0.5">Endereço IPv4 inválido</p>
                )}
              </div>
              <div>
                <input
                  type="number"
                  value={p.port}
                  onChange={(e) => {
                    const next = [...parentProxies];
                    next[i] = { ...next[i], port: parseInt(e.target.value) || 0 };
                    setParentProxies(next);
                  }}
                  min={1024}
                  max={65535}
                  className={`w-24 px-3 py-2 border rounded-md text-sm ${p.port < 1024 || p.port > 65535 ? 'border-red-500 bg-red-50' : 'border-gray-300'}`}
                />
                {(p.port < 1024 || p.port > 65535) && (
                  <p className="text-xs text-red-500 mt-0.5">1024-65535</p>
                )}
              </div>
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
              <label className="flex items-center gap-1 text-sm whitespace-nowrap mt-2">
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
                className="w-8 h-8 flex items-center justify-center text-red-500 hover:bg-red-50 rounded text-lg mt-1"
              >
                &times;
              </button>
            </div>
          ))}
        </div>
      </div>

      {/* Client ACL */}
      <div className="bg-white rounded-lg border p-5">
        <div className="flex items-center justify-between mb-3">
          <h2 className="text-base font-semibold text-gray-900">ACL de Clientes (ip_allow)</h2>
          <button
            type="button"
            onClick={() =>
              setClientACL([
                ...clientACL,
                { cidr: '', action: 'allow', priority: (clientACL.length + 1) * 10 },
              ])
            }
            className="px-3 py-1 text-sm text-blue-600 border border-blue-300 rounded-md hover:bg-blue-50"
          >
            + Adicionar
          </button>
        </div>
        <div className="space-y-2">
          {clientACL.map((acl, i) => (
            <div key={i} className="flex gap-2 items-start">
              <div className="flex-1">
                <input
                  value={acl.cidr}
                  onChange={(e) => {
                    const next = [...clientACL];
                    next[i] = { ...next[i], cidr: e.target.value };
                    setClientACL(next);
                  }}
                  className={`w-full px-3 py-2 border rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 ${acl.cidr && !isValidCIDR(acl.cidr) ? 'border-red-500 bg-red-50' : 'border-gray-300'}`}
                  placeholder="10.0.0.0/8"
                />
                {acl.cidr && !isValidCIDR(acl.cidr) && (
                  <p className="text-xs text-red-500 mt-0.5">CIDR/IP inválido ou 0.0.0.0 não permitido</p>
                )}
              </div>
              <select
                value={acl.action}
                onChange={(e) => {
                  const next = [...clientACL];
                  next[i] = { ...next[i], action: e.target.value as 'allow' | 'deny' };
                  setClientACL(next);
                }}
                className="w-32 px-3 py-2 border border-gray-300 rounded-md text-sm"
              >
                <option value="allow">Allow</option>
                <option value="deny">Deny</option>
              </select>
              <input
                type="number"
                value={acl.priority}
                onChange={(e) => {
                  const next = [...clientACL];
                  next[i] = { ...next[i], priority: parseInt(e.target.value) || 0 };
                  setClientACL(next);
                }}
                className="w-24 px-3 py-2 border border-gray-300 rounded-md text-sm"
              />
              <button
                onClick={() => setClientACL(clientACL.filter((_, idx) => idx !== i))}
                className="w-8 h-8 flex items-center justify-center text-red-500 hover:bg-red-50 rounded text-lg mt-1"
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
