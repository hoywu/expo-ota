import { apiRequest, buildQuery } from './client';
import type {
  CleanupReq,
  CleanupResp,
  ListUpdatesParams,
  ListUpdatesResp,
  PublishResp,
  RollbackResp,
  UpdateDetailResp,
} from '@/types/admin';

export function listUpdates(
  appSlug: string,
  params: ListUpdatesParams = {}
): Promise<ListUpdatesResp> {
  const qs = buildQuery({
    platform: params.platform,
    runtimeVersion: params.runtimeVersion,
    status: params.status,
    limit: params.limit,
    cursor: params.cursor,
  });
  return apiRequest<ListUpdatesResp>(`/apps/${encodeURIComponent(appSlug)}/updates${qs}`);
}

export function getUpdate(appSlug: string, updateId: string): Promise<UpdateDetailResp> {
  return apiRequest<UpdateDetailResp>(
    `/apps/${encodeURIComponent(appSlug)}/updates/${encodeURIComponent(updateId)}`
  );
}

export function deleteUpdate(appSlug: string, updateId: string): Promise<void> {
  return apiRequest<void>(
    `/apps/${encodeURIComponent(appSlug)}/updates/${encodeURIComponent(updateId)}`,
    { method: 'DELETE' }
  );
}

export function publishUpdate(appSlug: string, updateId: string): Promise<PublishResp> {
  return apiRequest<PublishResp>(
    `/apps/${encodeURIComponent(appSlug)}/updates/${encodeURIComponent(updateId)}/publish`,
    { method: 'POST' }
  );
}

export function rollbackUpdate(appSlug: string, updateId: string): Promise<RollbackResp> {
  return apiRequest<RollbackResp>(
    `/apps/${encodeURIComponent(appSlug)}/updates/${encodeURIComponent(updateId)}/rollback`,
    { method: 'POST' }
  );
}

export function cleanupUpdates(appSlug: string, data: CleanupReq): Promise<CleanupResp> {
  return apiRequest<CleanupResp>(`/apps/${encodeURIComponent(appSlug)}/updates/cleanup`, {
    method: 'POST',
    body: data,
  });
}
