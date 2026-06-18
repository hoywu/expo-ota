<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import CopyButton from '@/components/CopyButton.vue';
import TimeAgo from '@/components/TimeAgo.vue';
import EmptyState from '@/components/EmptyState.vue';
import ConfirmModal from '@/components/ConfirmModal.vue';
import { useAppsStore } from '@/stores/apps';
import * as updatesApi from '@/api/updates';
import { handleApiError, manifestUrl, truncateMiddle } from '@/utils/format';
import { buildLatestPublishedIds } from '@/utils/updates';
import type { UpdateListItem } from '@/types/admin';

const route = useRoute();
const router = useRouter();
const appsStore = useAppsStore();
const toast = useToast();

const appSlug = computed(() => route.params.appSlug as string);

const items = ref<UpdateListItem[]>([]);
const nextCursor = ref<string | undefined>();
const loading = ref(true);
const loadingMore = ref(false);

const platform = ref<string | undefined>();
const runtimeVersion = ref('');
const status = ref<string | undefined>();

const cleanupOpen = ref(false);
const cleanupKeepN = ref(3);
const cleanupLoading = ref(false);

const actionLoadingId = ref<string | null>(null);
const republishOpen = ref(false);
const republishTarget = ref<UpdateListItem | null>(null);

const runtimeVersions = computed(() =>
  [...new Set(items.value.map((u) => u.runtimeVersion))].sort()
);

const platformOptions = [
  { label: 'All', value: undefined },
  { label: 'iOS', value: 'ios' },
  { label: 'Android', value: 'android' },
];

const statusOptions = [
  { label: 'All', value: undefined },
  { label: 'Pending', value: 'pending' },
  { label: 'Published', value: 'published' },
];

async function load(reset = true): Promise<void> {
  if (reset) {
    loading.value = true;
    items.value = [];
    nextCursor.value = undefined;
  } else {
    loadingMore.value = true;
  }

  try {
    const resp = await updatesApi.listUpdates(appSlug.value, {
      platform: platform.value,
      runtimeVersion: runtimeVersion.value || undefined,
      status: status.value,
      limit: 20,
      cursor: reset ? undefined : nextCursor.value,
    });
    if (reset) {
      items.value = resp.items;
    } else {
      items.value = [...items.value, ...resp.items];
    }
    nextCursor.value = resp.nextCursor;
  } catch (e) {
    handleApiError(e, toast);
  } finally {
    loading.value = false;
    loadingMore.value = false;
  }
}

watch([platform, status], () => load(true));
watch(runtimeVersion, () => load(true));

onMounted(() => load(true));

async function publishUpdate(row: UpdateListItem): Promise<void> {
  actionLoadingId.value = row.id;
  try {
    await updatesApi.publishUpdate(appSlug.value, row.id);
    toast.add({ title: 'Update published', color: 'success', duration: 3000 });
    await load(true);
  } catch (e) {
    handleApiError(e, toast);
  } finally {
    actionLoadingId.value = null;
  }
}

function openRepublish(row: UpdateListItem): void {
  republishTarget.value = row;
  republishOpen.value = true;
}

async function confirmRepublish(): Promise<void> {
  if (!republishTarget.value) return;
  actionLoadingId.value = republishTarget.value.id;
  try {
    const resp = await updatesApi.rollbackUpdate(appSlug.value, republishTarget.value.id);
    toast.add({ title: 'Draft created from previous update', color: 'success', duration: 3000 });
    republishOpen.value = false;
    await router.push(`/apps/${appSlug.value}/updates/${resp.updateId}`);
  } catch (e) {
    handleApiError(e, toast);
  } finally {
    actionLoadingId.value = null;
  }
}

async function confirmCleanup(): Promise<void> {
  cleanupLoading.value = true;
  try {
    const resp = await updatesApi.cleanupUpdates(appSlug.value, {
      keepLatestN: cleanupKeepN.value,
    });
    toast.add({
      title: `Deleted ${resp.deletedUpdateIds.length} updates`,
      description: `${resp.orphanAssetCount} orphan assets queued for GC`,
      color: 'success',
      duration: 5000,
    });
    cleanupOpen.value = false;
    await load(true);
  } catch (e) {
    handleApiError(e, toast);
  } finally {
    cleanupLoading.value = false;
  }
}

const latestPublishedIds = computed(() => buildLatestPublishedIds(items.value));

function isLatestPublished(row: UpdateListItem): boolean {
  return latestPublishedIds.value.has(row.id);
}

const tableMeta = {
  class: {
    tr: (row: { original: UpdateListItem }) =>
      isLatestPublished(row.original) ? 'bg-success/5 ring-1 ring-inset ring-success/25' : '',
  },
};

const columns = [
  { accessorKey: 'status', header: 'Status' },
  { accessorKey: 'runtimeVersion', header: 'Runtime' },
  { accessorKey: 'platform', header: 'Platform' },
  { accessorKey: 'message', header: 'Message' },
  { accessorKey: 'manifestUuid', header: 'Manifest UUID', size: 240, minSize: 200 },
  { accessorKey: 'createdAt', header: 'Created' },
  { id: 'actions', header: '' },
];
</script>

<template>
  <div>
    <UCard v-if="appsStore.currentApp" variant="subtle" class="mb-6">
      <div class="flex flex-col sm:flex-row sm:items-center gap-3 justify-between">
        <div>
          <h2 class="text-lg font-semibold text-default">{{ appsStore.currentApp.name }}</h2>
          <p class="font-mono text-sm text-muted">{{ appsStore.currentApp.appSlug }}</p>
        </div>
        <div class="flex items-center gap-2 min-w-0 justify-end">
          <code class="text-xs bg-elevated px-2 py-1 rounded font-mono w-fit max-w-full truncate">
            {{ manifestUrl(appSlug) }}
          </code>
          <CopyButton :value="manifestUrl(appSlug)" label="Copy manifest URL" class="shrink-0" />
        </div>
      </div>
    </UCard>

    <UAlert
      color="info"
      variant="subtle"
      icon="i-lucide-info"
      title="Updates are finalized via CI"
      description="Use cli/publish.ts to finalize drafts, then Publish pending updates from this dashboard."
      class="mb-6"
    />

    <UDashboardToolbar class="mb-4 px-0">
      <template #left>
        <div class="dashboard-filters">
          <USelectMenu
            :items="platformOptions"
            :model-value="platform"
            value-key="value"
            placeholder="Platform"
            class="dashboard-filter-select-sm"
            @update:model-value="platform = $event as string | undefined"
          />
          <USelectMenu
            :items="[
              { label: 'All versions', value: '' },
              ...runtimeVersions.map((v) => ({ label: v, value: v })),
            ]"
            :model-value="runtimeVersion"
            value-key="value"
            placeholder="Runtime version"
            class="dashboard-filter-select-md"
            @update:model-value="runtimeVersion = ($event as string) ?? ''"
          />
          <USelectMenu
            :items="statusOptions"
            :model-value="status"
            value-key="value"
            placeholder="Status"
            class="dashboard-filter-select-sm"
            @update:model-value="status = $event as string | undefined"
          />
        </div>
      </template>
      <template #right>
        <UButton
          label="Cleanup old updates"
          icon="i-lucide-trash-2"
          color="neutral"
          variant="outline"
          @click="cleanupOpen = true"
        />
      </template>
    </UDashboardToolbar>

    <div v-if="loading" class="space-y-2">
      <USkeleton v-for="i in 6" :key="i" class="h-12 w-full" />
    </div>

    <EmptyState
      v-else-if="items.length === 0"
      title="No updates"
      description="Run your CI publish pipeline to create pending updates."
      icon="i-lucide-package"
    />

    <div v-else class="overflow-x-auto">
      <UCard variant="subtle" :ui="{ body: 'p-0 sm:p-0' }">
        <UTable :data="items" :columns="columns" :meta="tableMeta">
          <template #status-cell="{ row }">
            <div class="flex items-center gap-1.5">
              <UBadge
                :color="row.original.status === 'published' ? 'success' : 'warning'"
                variant="subtle"
              >
                {{ row.original.status }}
              </UBadge>
              <UBadge v-if="isLatestPublished(row.original)" color="success" variant="solid">
                Latest
              </UBadge>
            </div>
          </template>
          <template #platform-cell="{ row }">
            <span class="inline-flex items-center gap-1.5 whitespace-nowrap">
              <UIcon
                :name="
                  row.original.platform === 'ios'
                    ? 'i-lucide-smartphone'
                    : 'i-lucide-tablet-smartphone'
                "
                class="size-4 shrink-0"
              />
              {{ row.original.platform }}
            </span>
          </template>
          <template #message-cell="{ row }">
            <span class="truncate max-w-xs block">{{ row.original.message || '—' }}</span>
          </template>
          <template #manifestUuid-cell="{ row }">
            <span
              class="font-mono text-xs inline-flex items-center gap-1 whitespace-nowrap min-w-52"
            >
              {{ truncateMiddle(row.original.manifestUuid, 14, 8) }}
              <CopyButton :value="row.original.manifestUuid" />
            </span>
          </template>
          <template #createdAt-cell="{ row }">
            <TimeAgo :iso="row.original.createdAt" />
          </template>
          <template #actions-cell="{ row }">
            <div class="flex justify-end gap-1">
              <UButton
                label="View"
                size="xs"
                color="neutral"
                variant="ghost"
                :to="`/apps/${appSlug}/updates/${row.original.id}`"
              />
              <UButton
                v-if="row.original.status === 'pending'"
                label="Publish"
                size="xs"
                :loading="actionLoadingId === row.original.id"
                @click="publishUpdate(row.original)"
              />
              <UButton
                v-if="row.original.status === 'published' && !isLatestPublished(row.original)"
                label="Republish Previous"
                size="xs"
                color="neutral"
                variant="outline"
                @click="openRepublish(row.original)"
              />
            </div>
          </template>
        </UTable>
      </UCard>
    </div>

    <div v-if="nextCursor" class="mt-4 text-center">
      <UButton label="Load more" :loading="loadingMore" variant="outline" @click="load(false)" />
    </div>

    <UModal v-model:open="cleanupOpen" title="Cleanup old updates">
      <template #body>
        <UFormField label="Keep latest N published updates per stream">
          <UInput
            v-model.number="cleanupKeepN"
            type="number"
            min="1"
            class="form-control max-w-xs"
          />
        </UFormField>
      </template>
      <template #footer>
        <div class="flex justify-end gap-2">
          <UButton label="Cancel" color="neutral" variant="outline" @click="cleanupOpen = false" />
          <UButton
            label="Cleanup"
            color="error"
            :loading="cleanupLoading"
            @click="confirmCleanup"
          />
        </div>
      </template>
    </UModal>

    <ConfirmModal
      v-model:open="republishOpen"
      title="Republish Previous?"
      description="Creates a new pending draft from this update. You must Publish it to make it live."
      confirm-label="Create draft"
      :loading="actionLoadingId === republishTarget?.id"
      @confirm="confirmRepublish"
    />
  </div>
</template>
