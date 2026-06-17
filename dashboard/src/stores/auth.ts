import { defineStore } from 'pinia';
import { ref, computed } from 'vue';
import * as authApi from '@/api/auth';
import { clearSession, getAccessToken, setTokens } from '@/api/client';
import type { MeResp } from '@/types/admin';
import { normalizeUsername } from '@/utils/validation';

export const useAuthStore = defineStore('auth', () => {
  const user = ref<MeResp | null>(null);
  const initialized = ref(false);
  const hasToken = ref(Boolean(getAccessToken()));

  const isAuthenticated = computed(() => hasToken.value && Boolean(getAccessToken()));

  async function login(username: string, password: string): Promise<void> {
    const resp = await authApi.login({
      username: normalizeUsername(username),
      password,
    });
    setTokens(resp.accessToken, resp.refreshToken, resp.expiresIn);
    hasToken.value = true;
    await fetchMe();
  }

  async function fetchMe(): Promise<MeResp | null> {
    if (!getAccessToken()) {
      user.value = null;
      hasToken.value = false;
      initialized.value = true;
      return null;
    }
    try {
      user.value = await authApi.fetchMe();
      hasToken.value = true;
    } catch {
      clearSession();
      user.value = null;
      hasToken.value = false;
    }
    initialized.value = true;
    return user.value;
  }

  async function refresh(): Promise<boolean> {
    const { ensureFreshToken } = await import('@/api/client');
    const ok = await ensureFreshToken();
    if (!ok) {
      user.value = null;
      hasToken.value = false;
    }
    return ok;
  }

  async function logout(): Promise<void> {
    try {
      if (getAccessToken()) {
        await authApi.logout();
      }
    } catch {
      // ignore logout errors
    } finally {
      clearSession();
      user.value = null;
      hasToken.value = false;
    }
  }

  function reset(): void {
    clearSession();
    user.value = null;
    hasToken.value = false;
    initialized.value = false;
  }

  return {
    user,
    initialized,
    isAuthenticated,
    login,
    logout,
    refresh,
    fetchMe,
    reset,
  };
});
