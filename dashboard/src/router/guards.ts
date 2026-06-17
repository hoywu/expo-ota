import type { Router } from 'vue-router';
import { getAccessToken, ApiError } from '@/api/client';
import { useAuthStore } from '@/stores/auth';
import { useAppsStore } from '@/stores/apps';

export async function setupRouterGuards(router: Router): Promise<void> {
  router.beforeEach(async (to) => {
    const auth = useAuthStore();
    const appsStore = useAppsStore();

    const isLogin = to.path === '/login';
    const requiresAuth = to.meta.requiresAuth === true;

    if (isLogin && getAccessToken()) {
      return { path: '/apps' };
    }

    if (requiresAuth && !getAccessToken()) {
      return { path: '/login', query: { redirect: to.fullPath } };
    }

    if (requiresAuth && !auth.initialized) {
      await auth.fetchMe();
      if (!getAccessToken()) {
        return { path: '/login', query: { redirect: to.fullPath } };
      }
    }

    const appSlug = to.params.appSlug as string | undefined;
    if (appSlug && requiresAuth) {
      try {
        await appsStore.get(appSlug);
      } catch (e) {
        if (e instanceof ApiError && e.status === 404) {
          return { path: '/apps', query: { flash: 'app-not-found' } };
        }
        throw e;
      }
    } else if (!appSlug) {
      appsStore.clearCurrent();
    }

    return true;
  });
}
