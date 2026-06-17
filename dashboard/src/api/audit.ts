import { apiRequest, buildQuery } from './client';
import type { ListAuditLogsParams, ListAuditLogsResp } from '@/types/admin';

export function listAuditLogs(
  appSlug: string,
  params: ListAuditLogsParams = {}
): Promise<ListAuditLogsResp> {
  const qs = buildQuery({
    action: params.action,
    actor: params.actor,
    from: params.from,
    to: params.to,
    limit: params.limit,
    cursor: params.cursor,
  });
  return apiRequest<ListAuditLogsResp>(`/apps/${encodeURIComponent(appSlug)}/audit-logs${qs}`);
}
