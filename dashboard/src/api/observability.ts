import { apiRequest } from './client';
import type { SystemStatsResp } from '@/types/admin';

export function getSystemStats(): Promise<SystemStatsResp> {
  return apiRequest<SystemStatsResp>('/system/stats');
}
