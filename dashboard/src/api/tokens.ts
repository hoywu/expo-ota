import { apiRequest } from './client';
import type { CreateTokenReq, CreateTokenResp, ListTokensResp } from '@/types/admin';

export function listTokens(appSlug: string): Promise<ListTokensResp> {
  return apiRequest<ListTokensResp>(`/apps/${encodeURIComponent(appSlug)}/api-tokens`);
}

export function createToken(appSlug: string, data: CreateTokenReq): Promise<CreateTokenResp> {
  return apiRequest<CreateTokenResp>(`/apps/${encodeURIComponent(appSlug)}/api-tokens`, {
    method: 'POST',
    body: data,
  });
}

export function revokeToken(appSlug: string, tokenId: string): Promise<void> {
  return apiRequest<void>(
    `/apps/${encodeURIComponent(appSlug)}/api-tokens/${encodeURIComponent(tokenId)}`,
    { method: 'DELETE' }
  );
}
