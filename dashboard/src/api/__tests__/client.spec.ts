import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import {
  apiRequest,
  clearSession,
  setTokens,
  __resetRefreshPromiseForTests,
  ApiError,
} from '@/api/client';

describe('api/client', () => {
  beforeEach(() => {
    clearSession();
    __resetRefreshPromiseForTests();
    vi.stubGlobal('fetch', vi.fn());
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('parses ErrorResp on failure', async () => {
    vi.mocked(fetch).mockResolvedValueOnce(
      new Response(JSON.stringify({ code: 'Bad Request', message: 'invalid slug' }), {
        status: 400,
        statusText: 'Bad Request',
      })
    );

    await expect(apiRequest('/apps', { auth: false })).rejects.toMatchObject({
      status: 400,
      message: 'invalid slug',
    } satisfies Partial<ApiError>);
  });

  it('attempts refresh on 401 and retries', async () => {
    setTokens('old-access', 'refresh-token', 3600);

    vi.mocked(fetch)
      .mockResolvedValueOnce(new Response('', { status: 401, statusText: 'Unauthorized' }))
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            accessToken: 'new-access',
            refreshToken: 'new-refresh',
            expiresIn: 3600,
          }),
          { status: 200 }
        )
      )
      .mockResolvedValueOnce(new Response(JSON.stringify({ userId: 'u1' }), { status: 200 }));

    const result = await apiRequest<{ userId: string }>('/me');
    expect(result.userId).toBe('u1');
    expect(fetch).toHaveBeenCalledTimes(3);
  });
});
