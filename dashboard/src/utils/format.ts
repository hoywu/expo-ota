import { ApiError } from '@/api/client';

export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  const value = bytes / 1024 ** i;
  return `${value.toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
}

export function truncateMiddle(value: string, head = 8, tail = 6): string {
  if (value.length <= head + tail + 3) return value;
  return `${value.slice(0, head)}…${value.slice(-tail)}`;
}

export function manifestUrl(appSlug: string): string {
  return `${window.location.origin}/api/apps/${appSlug}/manifest`;
}

export function formatDateTime(iso: string): string {
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(new Date(iso));
}

export function formatRelativeTime(iso: string): string {
  const diffMs = Date.now() - new Date(iso).getTime();
  const seconds = Math.round(diffMs / 1000);
  if (seconds < 60) return 'just now';
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.round(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.round(hours / 24);
  if (days < 30) return `${days}d ago`;
  return formatDateTime(iso);
}

export function datetimeLocalToRFC3339(value: string): string | undefined {
  if (!value) return undefined;
  return new Date(value).toISOString();
}

export function signingKeyDeleteEligible(disabledAt?: string): boolean {
  return Boolean(disabledAt);
}

export function signingKeyDeleteCooldownRemaining(disabledAt?: string): string | null {
  return disabledAt ? null : 'Disable key first';
}

export const AUDIT_ACTIONS = [
  'create_app',
  'update_app',
  'delete_app',
  'finalize_update',
  'publish_update',
  'rollback_update',
  'delete_update',
  'cleanup_updates',
  'create_api_token',
  'revoke_api_token',
  'generate_signing_key',
  'import_signing_key',
  'patch_signing_key',
  'delete_signing_key',
  'create_user',
  'change_password',
  'disable_user',
  'enable_user',
] as const;

export interface ToastLike {
  add: (opts: {
    title: string;
    description?: string;
    color?: 'error' | 'primary' | 'secondary' | 'success' | 'info' | 'warning' | 'neutral';
    duration?: number;
  }) => unknown;
}

export function handleApiError(error: unknown, toast: ToastLike): string {
  if (error instanceof ApiError) {
    const message = error.message || error.code;
    toast.add({
      title: error.status >= 500 ? 'Something went wrong' : message,
      description: error.status >= 500 ? message : undefined,
      color: 'error',
      duration: 5000,
    });
    return message;
  }
  toast.add({ title: 'Something went wrong', color: 'error', duration: 5000 });
  return 'unknown error';
}
