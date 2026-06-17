import { describe, expect, it, beforeEach, vi } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';
import { useAuthStore } from '@/stores/auth';
import { clearSession, getAccessToken } from '@/api/client';

describe('auth store', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    clearSession();
    vi.stubGlobal('fetch', vi.fn());
  });

  it('sets session on login', async () => {
    vi.mocked(fetch)
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({ accessToken: 'access', refreshToken: 'refresh', expiresIn: 3600 }),
          { status: 200 }
        )
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({ userId: 'u1', username: 'admin', createdAt: '2026-01-01T00:00:00Z' }),
          { status: 200 }
        )
      );

    const store = useAuthStore();
    await store.login('admin', 'password1234');

    expect(store.isAuthenticated).toBe(true);
    expect(getAccessToken()).toBe('access');
    expect(store.user?.username).toBe('admin');
  });

  it('clears session on logout', async () => {
    vi.mocked(fetch).mockResolvedValue(new Response('{}', { status: 200 }));

    const store = useAuthStore();
    await store.login('admin', 'password1234');
    await store.logout();

    expect(store.isAuthenticated).toBe(false);
    expect(store.user).toBeNull();
    expect(getAccessToken()).toBeNull();
  });
});
