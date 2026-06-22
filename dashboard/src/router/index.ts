import { createRouter, createWebHistory } from 'vue-router';
import { setupRouterGuards } from './guards';

import LoginLayout from '@/layouts/LoginLayout.vue';
import DashboardLayout from '@/layouts/DashboardLayout.vue';

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/login',
      component: LoginLayout,
      children: [
        {
          path: '',
          name: 'login',
          component: () => import('@/views/LoginView.vue'),
        },
      ],
    },
    {
      path: '/',
      redirect: '/apps',
    },
    {
      path: '/',
      component: DashboardLayout,
      meta: { requiresAuth: true },
      children: [
        {
          path: 'apps',
          name: 'apps',
          component: () => import('@/views/apps/AppListView.vue'),
        },
        {
          path: 'apps/new',
          name: 'app-create',
          component: () => import('@/views/apps/AppCreateView.vue'),
        },
        {
          path: 'apps/:appSlug',
          redirect: (to) => `/apps/${to.params.appSlug}/updates`,
        },
        {
          path: 'apps/:appSlug/updates',
          name: 'updates',
          component: () => import('@/views/apps/UpdatesListView.vue'),
        },
        {
          path: 'apps/:appSlug/updates/:updateId',
          name: 'update-detail',
          component: () => import('@/views/apps/UpdateDetailView.vue'),
        },
        {
          path: 'apps/:appSlug/tokens',
          name: 'tokens',
          component: () => import('@/views/apps/TokensView.vue'),
        },
        {
          path: 'apps/:appSlug/signing-key',
          name: 'signing-key',
          component: () => import('@/views/apps/SigningKeyView.vue'),
        },
        {
          path: 'apps/:appSlug/audit-logs',
          name: 'audit-logs',
          component: () => import('@/views/apps/AuditLogsView.vue'),
        },
        {
          path: 'admin/users',
          name: 'users',
          component: () => import('@/views/admin/UsersView.vue'),
        },
        {
          path: 'admin/observability',
          name: 'observability',
          component: () => import('@/views/admin/ObservabilityView.vue'),
        },
      ],
    },
    {
      path: '/:pathMatch(.*)*',
      name: 'not-found',
      component: () => import('@/views/NotFoundView.vue'),
    },
  ],
});

setupRouterGuards(router);

export default router;
