export interface ErrorResp {
  code: string;
  message: string;
}

export interface LoginReq {
  username: string;
  password: string;
}

export interface LoginResp {
  accessToken: string;
  refreshToken: string;
  expiresIn: number;
}

export interface RefreshReq {
  refreshToken: string;
}

export interface RefreshResp {
  accessToken: string;
  refreshToken: string;
  expiresIn: number;
}

export interface MeResp {
  userId: string;
  username: string;
  createdAt: string;
  lastLoginAt?: string;
}

export interface AppResp {
  id: string;
  appSlug: string;
  name: string;
  description?: string;
  createdAt: string;
}

export interface ListAppsResp {
  items: AppResp[];
}

export interface CreateAppReq {
  appSlug: string;
  name: string;
  description?: string;
}

export interface UpdateAppReq {
  name?: string;
  description?: string;
}

export interface UpdateListItem {
  id: string;
  runtimeVersion: string;
  platform: string;
  manifestUuid: string;
  status: string;
  message?: string;
  createdAt: string;
  publishedAt?: string;
}

export interface ListUpdatesResp {
  items: UpdateListItem[];
  nextCursor?: string;
}

export interface UpdateAssetItem {
  key: string;
  sha256: string;
  size: number;
  url: string;
  fileExt?: string;
}

export interface UpdateStatsResp {
  requestedDevices: number;
  requestsWithoutDeviceId: number;
  succeededDevices: number;
  failedDevices: number;
  durationMinMs?: number;
  durationMaxMs?: number;
  durationAvgMs?: number;
}

export interface ClientEventItem {
  eventId: string;
  eventType: string;
  occurredAt: string;
  receivedAt: string;
  deviceId: string;
  appVersion?: string;
  osVersion?: string;
  durationMs?: number;
  errorCode?: string;
  errorMessage?: string;
  platform?: string;
  runtimeVersion?: string;
}

export interface ListUpdateClientEventsResp {
  items: ClientEventItem[];
}

export interface UpdateDetailResp {
  id: string;
  appSlug: string;
  runtimeVersion: string;
  platform: string;
  manifestUuid: string;
  status: string;
  message?: string;
  gitCommitHash?: string;
  createdAt: string;
  publishedAt?: string;
  launchAssetKey: string;
  launchAssetUrl: string;
  assets: UpdateAssetItem[];
  manifestPreview: Record<string, unknown>;
  stats: UpdateStatsResp;
}

export interface CleanupReq {
  keepLatestN: number;
}

export interface CleanupResp {
  deletedUpdateIds: string[];
  orphanAssetCount: number;
}

export interface RollbackResp {
  updateId: string;
  manifestUuid: string;
  createdAt: string;
}

export interface PublishResp {
  updateId: string;
  manifestUuid: string;
  status: string;
  publishedAt: string;
}

export interface TokenItem {
  id: string;
  name: string;
  createdBy: string;
  scopes: string[];
  lastUsedAt?: string;
  expiresAt?: string;
  createdAt: string;
  revokedAt?: string;
}

export interface ListTokensResp {
  items: TokenItem[];
}

export interface CreateTokenReq {
  name: string;
  expiresAt?: string;
}

export interface CreateTokenResp {
  id: string;
  name: string;
  token: string;
  expiresAt?: string;
  createdAt: string;
}

export interface SigningKeyResp {
  keyId: string;
  algorithm: string;
  publicKeyPem: string;
  enabled: boolean;
  createdAt: string;
  disabledAt?: string;
  hasPrivateKey: boolean;
}

export interface ListSigningKeysResp {
  items: SigningKeyResp[];
}

export interface GenerateSigningKeyReq {
  keyId: string;
}

export interface ImportSigningKeyReq {
  keyId: string;
  algorithm?: string;
  publicKeyPem: string;
  privateKeyPem: string;
}

export interface PatchSigningKeyReq {
  enabled: boolean;
}

export interface UserItem {
  id: string;
  username: string;
  createdAt: string;
  lastLoginAt?: string;
  disabledAt?: string;
}

export interface ListUsersResp {
  items: UserItem[];
}

export interface CreateUserReq {
  username: string;
  password: string;
}

export interface ChangePasswordReq {
  password: string;
}

export interface AuditLogItem {
  id: string;
  appSlug?: string;
  actorUserId?: string;
  action: string;
  targetType?: string;
  targetId?: string;
  requestId?: string;
  ip?: string;
  userAgent?: string;
  payload?: Record<string, unknown>;
  occurredAt: string;
}

export interface ListAuditLogsResp {
  items: AuditLogItem[];
  nextCursor?: string;
}

export interface ListUpdatesParams {
  platform?: string;
  runtimeVersion?: string;
  status?: string;
  limit?: number;
  cursor?: string;
}

export interface ListAuditLogsParams {
  action?: string;
  actor?: string;
  from?: string;
  to?: string;
  limit?: number;
  cursor?: string;
}

export interface SystemStatsResp {
  heapAllocBytes: number;
  heapInUseBytes: number;
  heapSysBytes: number;
  stackInUseBytes: number;
  numGC: number;
  numGoroutine: number;
  goVersion: string;
  uptimeSeconds: number;
}
