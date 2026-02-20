'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import toast from 'react-hot-toast';
import { api } from '@/lib/api';
import type { DomainRule, IPRangeRule, ParentProxy, ClientACLRule, Proxy, ApiError } from '@/types';

export default function NewConfigPage() {
  const router = useRouter();
  const [saving, setSaving] = useState(false);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [domains, setDomains] = useState<Omit<DomainRule, 'id'>[]>([]);
  const [ipRanges, setIpRanges] = useState<Omit<IPRangeRule, 'id'>[]>([]);
  const [parentProxies, setParentProxies] = useState<Omit<ParentProxy, 'id'>[]>([]);
  const [clientACL, setClientACL] = useState<Omit<ClientACLRule, 'id'>[]>([
    { cidr: '10.0.0.0/8', action: 'allow', priority: 10 },
  ]);
  const [selectedProxyIds, setSelectedProxyIds] = useState<string[]>([]);
  const [availableProxies, setAvailableProxies] = useState<Proxy[]>([]);

  useEffect(() => {
    api.proxies.list().then((res) => setAvailableProxies(res.data || [])).catch(() => {});
  }, []);

  function addDomain() {
    setDomains([...domains, { domain: '', action: 'direct', priority: (domains.length + 1) * 10 }]);
  }

  function removeDomain(i: number) {
    setDomains(domains.filter((_, idx) => idx !== i));
  }

  function updateDomain(i: number, field: string, value: string | number) {
    const next = [...domains];
    next[i] = { ...next[i], [field]: value };
    setDomains(next);
  }

  function addIpRange() {
    setIpRanges([...ipRanges, { cidr: '', action: 'direct', priority: (ipRanges.length + 1) * 10 }]);
  }

  function removeIpRange(i: number) {
    setIpRanges(ipRanges.filter((_, idx) => idx !== i));
  }

  function updateIpRange(i: number, field: string, value: string | number) {
    const next = [...ipRanges];
    next[i] = { ...next[i], [field]: value };
    setIpRanges(next);
  }

  function addParentProxy() {
    setParentProxies([
      ...parentProxies,
      { address: '', port: 3128, priority: parentProxies.length + 1, enabled: true },
    ]);
  }

  function removeParentProxy(i: number) {
    setParentProxies(parentProxies.filter((_, idx) => idx !== i));
  }

  function updateParentProxy(i: number, field: string, value: string | number | boolean) {
    const next = [...parentProxies];
    next[i] = { ...next[i], [field]: value };
    setParentProxies(next);
  }

  function addClientACL() {
    setClientACL([...clientACL, { cidr: '', action: 'allow', priority: (clientACL.length + 1) * 10 }]);
  }

  function removeClientACL(i: number) {
    setClientACL(clientACL.filter((_, idx) => idx !== i));
  }

  function updateClientACL(i: number, field: string, value: string | number) {
    const next = [...clientACL];
    next[i] = { ...next[i], [field]: value };
    setClientACL(next);
  }

  function toggleProxy(id: string) {
    setSelectedProxyIds((prev) =>
      prev.includes(id) ? prev.filter((p) => p !== id) : [...prev, id]
    );
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim()) {
      toast.error('Nome é obrigatório');
      return;
    }
    setSaving(true);
    try {
      const res = await api.configs.create({
        name: name.trim(),
        description: description.trim() || undefined,
        domains,
        ip_ranges: ipRanges,
        parent_proxies: parentProxies,
        client_acl: clientACL,
        proxy_ids: selectedProxyIds,
      });
      toast.success('Config criada com sucesso');
      router.push(`/configs/${res.id}`);
    } catch (err) {
      toast.error((err as ApiError).message || 'Erro ao criar config');
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="max-w-4xl">
      <h1 className="text-2xl font-bold text-gray-900 mb-6">Nova Configuração</h1>

      <form onSubmit={handleSubmit} className="space-y-8">
        {/* Info */}
        <Section title="Informações Gerais">
          <div className="grid grid-cols-1 gap-4">
            <Field label="Nome">
              <input
                value={name}
                onChange={(e) => setName(e.target.value)}
                className="px-3 py-2 border border-gray-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                placeholder="Ex: Production Config"
                required
              />
            </Field>
            <Field label="Descrição">
              <textarea
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                className="px-3 py-2 border border-gray-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                rows={2}
                placeholder="Descrição opcional"
              />
            </Field>
          </div>
        </Section>

        {/* Domain Rules */}
        <Section
          title="Regras de Domínio"
          action={<AddButton onClick={addDomain} label="Adicionar" />}
        >
          {domains.length === 0 ? (
            <p className="text-sm text-gray-500">Nenhuma regra de domínio adicionada.</p>
          ) : (
            <div className="space-y-2">
              {domains.map((d, i) => (
                <div key={i} className="flex gap-2 items-center">
                  <input
                    value={d.domain}
                    onChange={(e) => updateDomain(i, 'domain', e.target.value)}
                    className="px-3 py-2 border border-gray-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent flex-1"
                    placeholder=".example.com"
                  />
                  <select
                    value={d.action}
                    onChange={(e) => updateDomain(i, 'action', e.target.value)}
                    className="px-3 py-2 border border-gray-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent w-32"
                  >
                    <option value="direct">Direct</option>
                    <option value="parent">Parent</option>
                  </select>
                  <input
                    type="number"
                    value={d.priority}
                    onChange={(e) => updateDomain(i, 'priority', parseInt(e.target.value) || 0)}
                    className="px-3 py-2 border border-gray-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent w-24"
                    placeholder="Prioridade"
                  />
                  <button type="button" onClick={() => removeDomain(i)} className="w-8 h-8 flex items-center justify-center text-red-500 hover:bg-red-50 rounded text-lg leading-none">
                    &times;
                  </button>
                </div>
              ))}
            </div>
          )}
        </Section>

        {/* IP Ranges */}
        <Section
          title="Regras de IP (CIDR)"
          action={<AddButton onClick={addIpRange} label="Adicionar" />}
        >
          {ipRanges.length === 0 ? (
            <p className="text-sm text-gray-500">Nenhuma regra de IP adicionada.</p>
          ) : (
            <div className="space-y-2">
              {ipRanges.map((r, i) => (
                <div key={i} className="flex gap-2 items-center">
                  <input
                    value={r.cidr}
                    onChange={(e) => updateIpRange(i, 'cidr', e.target.value)}
                    className="px-3 py-2 border border-gray-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent flex-1"
                    placeholder="10.0.0.0/8"
                  />
                  <select
                    value={r.action}
                    onChange={(e) => updateIpRange(i, 'action', e.target.value)}
                    className="px-3 py-2 border border-gray-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent w-32"
                  >
                    <option value="direct">Direct</option>
                    <option value="parent">Parent</option>
                  </select>
                  <input
                    type="number"
                    value={r.priority}
                    onChange={(e) => updateIpRange(i, 'priority', parseInt(e.target.value) || 0)}
                    className="px-3 py-2 border border-gray-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent w-24"
                    placeholder="Prioridade"
                  />
                  <button type="button" onClick={() => removeIpRange(i)} className="w-8 h-8 flex items-center justify-center text-red-500 hover:bg-red-50 rounded text-lg leading-none">
                    &times;
                  </button>
                </div>
              ))}
            </div>
          )}
        </Section>

        {/* Parent Proxies */}
        <Section
          title="Parent Proxies"
          action={<AddButton onClick={addParentProxy} label="Adicionar" />}
        >
          {parentProxies.length === 0 ? (
            <p className="text-sm text-gray-500">Nenhum parent proxy adicionado.</p>
          ) : (
            <div className="space-y-2">
              {parentProxies.map((p, i) => (
                <div key={i} className="flex gap-2 items-center">
                  <input
                    value={p.address}
                    onChange={(e) => updateParentProxy(i, 'address', e.target.value)}
                    className="px-3 py-2 border border-gray-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent flex-1"
                    placeholder="10.96.215.26"
                  />
                  <input
                    type="number"
                    value={p.port}
                    onChange={(e) => updateParentProxy(i, 'port', parseInt(e.target.value) || 0)}
                    className="px-3 py-2 border border-gray-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent w-24"
                    placeholder="Porta"
                  />
                  <input
                    type="number"
                    value={p.priority}
                    onChange={(e) => updateParentProxy(i, 'priority', parseInt(e.target.value) || 0)}
                    className="px-3 py-2 border border-gray-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent w-24"
                    placeholder="Prioridade"
                  />
                  <label className="flex items-center gap-1 text-sm text-gray-600 whitespace-nowrap">
                    <input
                      type="checkbox"
                      checked={p.enabled}
                      onChange={(e) => updateParentProxy(i, 'enabled', e.target.checked)}
                      className="rounded"
                    />
                    Ativo
                  </label>
                  <button type="button" onClick={() => removeParentProxy(i)} className="w-8 h-8 flex items-center justify-center text-red-500 hover:bg-red-50 rounded text-lg leading-none">
                    &times;
                  </button>
                </div>
              ))}
            </div>
          )}
        </Section>

        {/* Client ACL */}
        <Section
          title="ACL de Clientes (ip_allow)"
          action={<AddButton onClick={addClientACL} label="Adicionar" />}
        >
          {clientACL.length === 0 ? (
            <p className="text-sm text-gray-500">Nenhuma regra de ACL adicionada. Defaults serão usados (127.0.0.1, ::1, 10.0.0.0/8).</p>
          ) : (
            <div className="space-y-2">
              {clientACL.map((acl, i) => (
                <div key={i} className="flex gap-2 items-center">
                  <input
                    value={acl.cidr}
                    onChange={(e) => updateClientACL(i, 'cidr', e.target.value)}
                    className="px-3 py-2 border border-gray-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent flex-1"
                    placeholder="10.0.0.0/8"
                  />
                  <select
                    value={acl.action}
                    onChange={(e) => updateClientACL(i, 'action', e.target.value)}
                    className="px-3 py-2 border border-gray-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent w-32"
                  >
                    <option value="allow">Allow</option>
                    <option value="deny">Deny</option>
                  </select>
                  <input
                    type="number"
                    value={acl.priority}
                    onChange={(e) => updateClientACL(i, 'priority', parseInt(e.target.value) || 0)}
                    className="px-3 py-2 border border-gray-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent w-24"
                    placeholder="Prioridade"
                  />
                  <button type="button" onClick={() => removeClientACL(i)} className="w-8 h-8 flex items-center justify-center text-red-500 hover:bg-red-50 rounded text-lg leading-none">
                    &times;
                  </button>
                </div>
              ))}
            </div>
          )}
        </Section>

        {/* Proxy Assignment */}
        <Section title="Proxies Associados">
          {availableProxies.length === 0 ? (
            <p className="text-sm text-gray-500">Nenhum proxy registrado no sistema.</p>
          ) : (
            <div className="space-y-1">
              {availableProxies.map((proxy) => (
                <label key={proxy.id} className="flex items-center gap-2 p-2 rounded hover:bg-gray-50 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={selectedProxyIds.includes(proxy.id)}
                    onChange={() => toggleProxy(proxy.id)}
                    className="rounded"
                  />
                  <span className="text-sm">{proxy.hostname}</span>
                  <span className={`w-2 h-2 rounded-full ${proxy.is_online ? 'bg-green-500' : 'bg-red-500'}`} />
                </label>
              ))}
            </div>
          )}
        </Section>

        <div className="flex gap-3">
          <button
            type="submit"
            disabled={saving}
            className="px-6 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700 disabled:opacity-50 transition-colors"
          >
            {saving ? 'Salvando...' : 'Criar Config'}
          </button>
          <button
            type="button"
            onClick={() => router.back()}
            className="px-6 py-2 border text-sm rounded-md hover:bg-gray-50"
          >
            Cancelar
          </button>
        </div>
      </form>

    </div>
  );
}

function Section({
  title,
  action,
  children,
}: {
  title: string;
  action?: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <div className="bg-white rounded-lg border p-5">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-base font-semibold text-gray-900">{title}</h2>
        {action}
      </div>
      {children}
    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <label className="block text-sm font-medium text-gray-700 mb-1">{label}</label>
      {children}
    </div>
  );
}

function AddButton({ onClick, label }: { onClick: () => void; label: string }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="px-3 py-1 text-sm text-blue-600 border border-blue-300 rounded-md hover:bg-blue-50 transition-colors"
    >
      + {label}
    </button>
  );
}
