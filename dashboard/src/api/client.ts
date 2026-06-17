const ACCESS_TOKEN_KEY = 'ota_access_token';
const REFRESH_TOKEN_KEY = 'ota_refresh_token';
const EXPIRES_AT_KEY = 'ota_expires_at';

export const API_BASE = import.meta.env.VITE_API_BASE ?? '/api/admin';

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
  }
}

export function getAccessToken(): string | null {
  return sessionStorage.getItem(ACCESS_TOKEN_KEY);
}

export function getRefreshToken(): string | null {
  return sessionStorage.getItem(REFRESH_TOKEN_KEY);
}

export function getExpiresAt(): number | null {
  const raw = sessionStorage.getItem(EXPIRES_AT_KEY);
  return raw ? Number(raw) : null;
}

export function setTokens(accessToken: string, refreshToken: string, expiresIn: number): void {
  sessionStorage.setItem(ACCESS_TOKEN_KEY, accessToken);
  sessionStorage.setItem(REFRESH_TOKEN_KEY, refreshToken);
  sessionStorage.setItem(EXPIRES_AT_KEY, String(Date.now() + expiresIn * 1000));
}

export function clearSession(): void {
  sessionStorage.removeItem(ACCESS_TOKEN_KEY);
  sessionStorage.removeItem(REFRESH_TOKEN_KEY);
  sessionStorage.removeItem(EXPIRES_AT_KEY);
}

export function isTokenExpiringSoon(thresholdMs = 5 * 60 * 1000): boolean {
  const expiresAt = getExpiresAt();
  if (!expiresAt) return false;
  return expiresAt - Date.now() < thresholdMs;
}

let refreshPromise: Promise<boolean> | null = null;

async function refreshAccessToken(): Promise<boolean> {
  const refreshToken = getRefreshToken();
  if (!refreshToken) return false;

  const res = await fetch(`${API_BASE}/refresh`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ refreshToken }),
  });

  if (!res.ok) {
    clearSession();
    return false;
  }

  const data = (await res.json()) as {
    accessToken: string;
    refreshToken: string;
    expiresIn: number;
  };
  setTokens(data.accessToken, data.refreshToken, data.expiresIn);
  return true;
}

export async function ensureFreshToken(): Promise<boolean> {
  if (!getAccessToken()) return false;
  if (!isTokenExpiringSoon()) return true;

  if (!refreshPromise) {
    refreshPromise = refreshAccessToken().finally(() => {
      refreshPromise = null;
    });
  }
  return refreshPromise;
}

async function parseErrorResponse(res: Response): Promise<ApiError> {
  let code = res.statusText;
  let message = res.statusText;
  try {
    const body = (await res.json()) as { code?: string; message?: string };
    if (body.code) code = body.code;
    if (body.message) message = body.message;
  } catch {
    // ignore non-json body
  }
  return new ApiError(res.status, code, message);
}

export interface RequestOptions extends Omit<RequestInit, 'body'> {
  body?: unknown;
  auth?: boolean;
  retry?: boolean;
}

export async function apiRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { body, auth = true, retry = true, headers: initHeaders, ...rest } = options;

  if (auth) {
    const ok = await ensureFreshToken();
    if (!ok && getAccessToken() === null) {
      throw new ApiError(401, 'Unauthorized', 'unauthorized');
    }
  }

  const headers = new Headers(initHeaders);
  if (body !== undefined) {
    headers.set('Content-Type', 'application/json');
  }
  if (auth) {
    const token = getAccessToken();
    if (token) headers.set('Authorization', `Bearer ${token}`);
  }

  const res = await fetch(`${API_BASE}${path}`, {
    ...rest,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });

  if (res.status === 401 && auth && retry) {
    const refreshed = await refreshAccessToken();
    if (refreshed) {
      return apiRequest<T>(path, { ...options, retry: false });
    }
    clearSession();
    throw new ApiError(401, 'Unauthorized', 'unauthorized');
  }

  if (!res.ok) {
    throw await parseErrorResponse(res);
  }

  if (res.status === 204) {
    return undefined as T;
  }

  const text = await res.text();
  if (!text) {
    return {} as T;
  }
  return JSON.parse(text) as T;
}

export function buildQuery(params: Record<string, string | number | undefined>): string {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== '') {
      search.set(key, String(value));
    }
  }
  const qs = search.toString();
  return qs ? `?${qs}` : '';
}

// Test helpers
export function __resetRefreshPromiseForTests(): void {
  refreshPromise = null;
}

export function __setRefreshPromiseForTests(promise: Promise<boolean> | null): void {
  refreshPromise = promise;
}
