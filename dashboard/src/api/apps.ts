import { apiRequest } from './client';
import type { AppResp, CreateAppReq, ListAppsResp, UpdateAppReq } from '@/types/admin';

export function listApps(): Promise<ListAppsResp> {
  return apiRequest<ListAppsResp>('/apps');
}

export function getApp(appSlug: string): Promise<AppResp> {
  return apiRequest<AppResp>(`/apps/${encodeURIComponent(appSlug)}`);
}

export function createApp(data: CreateAppReq): Promise<AppResp> {
  return apiRequest<AppResp>('/apps', { method: 'POST', body: data });
}

export function updateApp(appSlug: string, data: UpdateAppReq): Promise<AppResp> {
  return apiRequest<AppResp>(`/apps/${encodeURIComponent(appSlug)}`, {
    method: 'PATCH',
    body: data,
  });
}

export function deleteApp(appSlug: string): Promise<void> {
  return apiRequest<void>(`/apps/${encodeURIComponent(appSlug)}`, { method: 'DELETE' });
}
