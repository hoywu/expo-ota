import { defineStore } from 'pinia';
import { ref } from 'vue';
import * as appsApi from '@/api/apps';
import type { AppResp } from '@/types/admin';

export const useAppsStore = defineStore('apps', () => {
  const items = ref<AppResp[]>([]);
  const currentApp = ref<AppResp | null>(null);

  async function list(): Promise<AppResp[]> {
    const resp = await appsApi.listApps();
    items.value = resp.items;
    return items.value;
  }

  async function get(appSlug: string): Promise<AppResp> {
    const app = await appsApi.getApp(appSlug);
    currentApp.value = app;
    return app;
  }

  async function create(data: Parameters<typeof appsApi.createApp>[0]): Promise<AppResp> {
    const app = await appsApi.createApp(data);
    items.value = [app, ...items.value.filter((a) => a.id !== app.id)];
    currentApp.value = app;
    return app;
  }

  async function update(
    appSlug: string,
    data: Parameters<typeof appsApi.updateApp>[1]
  ): Promise<AppResp> {
    const app = await appsApi.updateApp(appSlug, data);
    items.value = items.value.map((a) => (a.appSlug === appSlug ? app : a));
    if (currentApp.value?.appSlug === appSlug) {
      currentApp.value = app;
    }
    return app;
  }

  async function remove(appSlug: string): Promise<void> {
    await appsApi.deleteApp(appSlug);
    items.value = items.value.filter((a) => a.appSlug !== appSlug);
    if (currentApp.value?.appSlug === appSlug) {
      currentApp.value = null;
    }
  }

  function clearCurrent(): void {
    currentApp.value = null;
  }

  return {
    items,
    currentApp,
    list,
    get,
    create,
    update,
    remove,
    clearCurrent,
  };
});
