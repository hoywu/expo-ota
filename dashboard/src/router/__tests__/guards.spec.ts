import { describe, expect, it, vi, beforeEach } from 'vitest';
import { createMemoryHistory, createRouter } from 'vue-router';
import { createPinia, setActivePinia } from 'pinia';
import { setupRouterGuards } from '@/router/guards';
import * as client from '@/api/client';

vi.mock('@/stores/apps', () => ({
  useAppsStore: () => ({
    get: vi.fn<(slug: string) => Promise<unknown>>(),
    clearCurrent: vi.fn<() => void>(),
  }),
}));

describe('router guards', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    client.clearSession();
    vi.restoreAllMocks();
  });

  it('redirects unauthenticated users to login', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/login', name: 'login', component: { template: '<div />' } },
        {
          path: '/apps',
          name: 'apps',
          component: { template: '<div />' },
          meta: { requiresAuth: true },
        },
      ],
    });

    await setupRouterGuards(router);
    await router.push('/apps');
    await router.isReady();

    expect(router.currentRoute.value.path).toBe('/login');
    expect(router.currentRoute.value.query.redirect).toBe('/apps');
  });

  it('redirects authenticated users away from login', async () => {
    vi.spyOn(client, 'getAccessToken').mockReturnValue('access-token');

    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/login', name: 'login', component: { template: '<div />' } },
        {
          path: '/apps',
          name: 'apps',
          component: { template: '<div />' },
          meta: { requiresAuth: true },
        },
      ],
    });

    await setupRouterGuards(router);
    await router.replace('/login');
    await router.isReady();

    expect(router.currentRoute.value.path).toBe('/apps');
  });
});
