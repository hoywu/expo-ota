import { apiRequest } from './client';
import type {
  GenerateSigningKeyReq,
  ImportSigningKeyReq,
  ListSigningKeysResp,
  PatchSigningKeyReq,
  SigningKeyResp,
} from '@/types/admin';

export function getSigningKey(appSlug: string): Promise<SigningKeyResp> {
  return apiRequest<SigningKeyResp>(`/apps/${encodeURIComponent(appSlug)}/signing-key`);
}

export function listSigningKeys(appSlug: string): Promise<ListSigningKeysResp> {
  return apiRequest<ListSigningKeysResp>(`/apps/${encodeURIComponent(appSlug)}/signing-keys`);
}

export function generateSigningKey(
  appSlug: string,
  data: GenerateSigningKeyReq
): Promise<SigningKeyResp> {
  return apiRequest<SigningKeyResp>(`/apps/${encodeURIComponent(appSlug)}/signing-key/generate`, {
    method: 'POST',
    body: data,
  });
}

export function importSigningKey(
  appSlug: string,
  data: ImportSigningKeyReq
): Promise<SigningKeyResp> {
  return apiRequest<SigningKeyResp>(`/apps/${encodeURIComponent(appSlug)}/signing-key/import`, {
    method: 'POST',
    body: data,
  });
}

export function patchSigningKey(
  appSlug: string,
  data: PatchSigningKeyReq,
  keyId?: string
): Promise<SigningKeyResp> {
  const path = keyId
    ? `/apps/${encodeURIComponent(appSlug)}/signing-keys/${encodeURIComponent(keyId)}`
    : `/apps/${encodeURIComponent(appSlug)}/signing-key`;
  return apiRequest<SigningKeyResp>(path, {
    method: 'PATCH',
    body: data,
  });
}

export function deleteSigningKey(appSlug: string, keyId?: string): Promise<void> {
  const path = keyId
    ? `/apps/${encodeURIComponent(appSlug)}/signing-keys/${encodeURIComponent(keyId)}`
    : `/apps/${encodeURIComponent(appSlug)}/signing-key`;
  return apiRequest<void>(path, {
    method: 'DELETE',
  });
}
