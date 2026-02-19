import { cn } from '@/lib/utils';

const statusStyles: Record<string, string> = {
  draft: 'bg-gray-100 text-gray-700',
  pending_approval: 'bg-yellow-100 text-yellow-800',
  approved: 'bg-blue-100 text-blue-800',
  active: 'bg-green-100 text-green-800',
  online: 'bg-green-100 text-green-800',
  offline: 'bg-red-100 text-red-800',
};

const statusLabels: Record<string, string> = {
  draft: 'Rascunho',
  pending_approval: 'Pendente',
  approved: 'Aprovada',
  active: 'Ativa',
  online: 'Online',
  offline: 'Offline',
};

export function StatusBadge({ status }: { status: string }) {
  return (
    <span
      className={cn(
        'inline-flex items-center px-2 py-0.5 rounded text-xs font-medium',
        statusStyles[status] || 'bg-gray-100 text-gray-700'
      )}
    >
      {statusLabels[status] || status}
    </span>
  );
}
