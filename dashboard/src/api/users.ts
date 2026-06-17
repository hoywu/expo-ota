import { apiRequest } from './client';
import type { ChangePasswordReq, CreateUserReq, ListUsersResp, UserItem } from '@/types/admin';

export function listUsers(): Promise<ListUsersResp> {
  return apiRequest<ListUsersResp>('/users');
}

export function createUser(data: CreateUserReq): Promise<UserItem> {
  return apiRequest<UserItem>('/users', { method: 'POST', body: data });
}

export function changePassword(userId: string, data: ChangePasswordReq): Promise<void> {
  return apiRequest<void>(`/users/${encodeURIComponent(userId)}/password`, {
    method: 'PATCH',
    body: data,
  });
}

export function disableUser(userId: string): Promise<void> {
  return apiRequest<void>(`/users/${encodeURIComponent(userId)}/disable`, { method: 'POST' });
}

export function enableUser(userId: string): Promise<void> {
  return apiRequest<void>(`/users/${encodeURIComponent(userId)}/enable`, { method: 'POST' });
}
