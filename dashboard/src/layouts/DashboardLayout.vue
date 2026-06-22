<script setup lang="ts">
import { computed, ref } from 'vue';
import { useRoute } from 'vue-router';
import type { NavigationMenuItem } from '@nuxt/ui';
import AppSwitcher from '@/components/AppSwitcher.vue';
import UserMenu from '@/components/UserMenu.vue';
import { useAppsStore } from '@/stores/apps';

const route = useRoute();
const appsStore = useAppsStore();

const changePasswordOpen = ref(false);

const appSlug = computed(() => route.params.appSlug as string | undefined);
const inAppContext = computed(() => Boolean(appSlug.value));

const globalNav = computed<NavigationMenuItem[]>(() => [
  {
    label: 'Apps',
    icon: 'i-lucide-layout-grid',
    to: '/apps',
    active: route.path === '/apps' || route.path === '/apps/new',
  },
  {
    label: 'Users',
    icon: 'i-lucide-users',
    to: '/admin/users',
    active: route.path.startsWith('/admin/users'),
  },
  {
    label: 'Observability',
    icon: 'i-lucide-activity',
    to: '/admin/observability',
    active: route.path.startsWith('/admin/observability'),
  },
]);

const appNav = computed<NavigationMenuItem[]>(() => {
  if (!appSlug.value) return [];
  const base = `/apps/${appSlug.value}`;
  return [
    {
      label: 'Updates',
      icon: 'i-lucide-package',
      to: `${base}/updates`,
      active: route.path.includes('/updates'),
    },
    {
      label: 'API Tokens',
      icon: 'i-lucide-key',
      to: `${base}/tokens`,
      active: route.path.endsWith('/tokens'),
    },
    {
      label: 'Signing Key',
      icon: 'i-lucide-shield',
      to: `${base}/signing-key`,
      active: route.path.endsWith('/signing-key'),
    },
    {
      label: 'Audit Logs',
      icon: 'i-lucide-scroll-text',
      to: `${base}/audit-logs`,
      active: route.path.endsWith('/audit-logs'),
    },
  ];
});

const pageTitle = computed(() => {
  if (route.name === 'apps') return 'Apps';
  if (route.name === 'app-create') return 'Create App';
  if (route.name === 'updates') return 'Updates';
  if (route.name === 'update-detail') return 'Update Detail';
  if (route.name === 'tokens') return 'API Tokens';
  if (route.name === 'signing-key') return 'Signing Key';
  if (route.name === 'audit-logs') return 'Audit Logs';
  if (route.name === 'users') return 'Users';
  if (route.name === 'observability') return 'Observability';
  return 'Dashboard';
});

const breadcrumb = computed(() => {
  const items: { label: string; to?: string }[] = [{ label: 'Apps', to: '/apps' }];
  if (appSlug.value && appsStore.currentApp) {
    items.push({
      label: appsStore.currentApp.name,
      to: `/apps/${appSlug.value}/updates`,
    });
  }
  if (route.name === 'update-detail') {
    items.push({ label: 'Update Detail' });
  } else if (
    route.name !== 'apps' &&
    route.name !== 'app-create' &&
    pageTitle.value !== appsStore.currentApp?.name
  ) {
    items.push({ label: pageTitle.value });
  }
  return items;
});
</script>

<template>
  <UDashboardGroup storage="local" storage-key="ota-dashboard">
    <UDashboardSidebar collapsible>
      <template #header="{ collapsed }">
        <div class="flex items-center gap-2 px-1">
          <UIcon name="i-lucide-radio-tower" class="size-5 text-primary shrink-0" />
          <span v-if="!collapsed" class="font-semibold text-default truncate">Expo OTA</span>
        </div>
      </template>

      <template #default="{ collapsed }">
        <UNavigationMenu
          :collapsed="collapsed"
          :items="[globalNav]"
          orientation="vertical"
          class="mb-4"
        />
        <template v-if="inAppContext">
          <p
            v-if="!collapsed"
            class="px-3 py-1 text-xs font-medium text-muted uppercase tracking-wide"
          >
            App
          </p>
          <UNavigationMenu :collapsed="collapsed" :items="[appNav]" orientation="vertical" />
        </template>
      </template>

      <template #footer="{ collapsed }">
        <UserMenu v-model:change-password-open="changePasswordOpen" :collapsed="collapsed" />
      </template>
    </UDashboardSidebar>

    <UDashboardPanel>
      <template #header>
        <UDashboardNavbar :title="pageTitle">
          <template #leading>
            <UDashboardSidebarCollapse />
          </template>
          <template #left>
            <UBreadcrumb
              v-if="breadcrumb.length > 1"
              :items="breadcrumb.map((b) => ({ label: b.label, to: b.to }))"
            />
          </template>
          <template #right>
            <AppSwitcher v-if="inAppContext" />
          </template>
        </UDashboardNavbar>
      </template>

      <template #body>
        <div class="p-4 sm:p-6 max-w-7xl mx-auto w-full">
          <RouterView />
        </div>
      </template>
    </UDashboardPanel>
  </UDashboardGroup>
</template>
