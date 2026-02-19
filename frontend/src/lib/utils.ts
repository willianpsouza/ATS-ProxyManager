export function cn(...classes: (string | boolean | undefined | null)[]): string {
  return classes.filter(Boolean).join(' ');
}

export function formatDate(date: string | undefined | null): string {
  if (!date) return '-';
  return new Date(date).toLocaleString('pt-BR', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

export function formatRelative(date: string | undefined | null): string {
  if (!date) return '-';
  const now = Date.now();
  const d = new Date(date).getTime();
  const diff = now - d;

  if (diff < 60_000) return 'agora';
  if (diff < 3_600_000) return `${Math.floor(diff / 60_000)}min atrás`;
  if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)}h atrás`;
  return formatDate(date);
}

export function roleLabel(role: string): string {
  const map: Record<string, string> = { root: 'Root', admin: 'Admin', regular: 'Regular' };
  return map[role] || role;
}

export function statusLabel(status: string): string {
  const map: Record<string, string> = {
    draft: 'Rascunho',
    pending_approval: 'Pendente',
    approved: 'Aprovada',
    active: 'Ativa',
  };
  return map[status] || status;
}

export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`;
}
