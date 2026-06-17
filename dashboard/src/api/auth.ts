import { apiRequest } from './client';
import type { LoginReq, LoginResp, MeResp, RefreshResp } from '@/types/admin';

export function login(data: LoginReq): Promise<LoginResp> {
  return apiRequest<LoginResp>('/login', { method: 'POST', body: data, auth: false });
}

export function refresh(refreshToken: string): Promise<RefreshResp> {
  return apiRequest<RefreshResp>('/refresh', {
    method: 'POST',
    body: { refreshToken },
    auth: false,
  });
}

export function logout(): Promise<void> {
  return apiRequest<void>('/logout', { method: 'POST' });
}

export function fetchMe(): Promise<MeResp> {
  return apiRequest<MeResp>('/me');
}
