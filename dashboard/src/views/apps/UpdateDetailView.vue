<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import CopyButton from '@/components/CopyButton.vue';
import StatCard from '@/components/StatCard.vue';
import JsonPreview from '@/components/JsonPreview.vue';
import EmptyState from '@/components/EmptyState.vue';
import TimeAgo from '@/components/TimeAgo.vue';
import ConfirmModal from '@/components/ConfirmModal.vue';
import * as updatesApi from '@/api/updates';
import { handleApiError, formatBytes, truncateMiddle } from '@/utils/format';
import type { UpdateDetailResp } from '@/types/admin';

const route = useRoute();
const router = useRouter();
const toast = useToast();

const appSlug = computed(() => route.params.appSlug as string);
const updateId = computed(() => route.params.updateId as string);

const detail = ref<UpdateDetailResp | null>(null);
const loading = ref(true);
const actionLoading = ref(false);
const deleteOpen = ref(false);
const republishOpen = ref(false);

async function load(): Promise<void> {
  loading.value = true;
  try {
    detail.value = await updatesApi.getUpdate(appSlug.value, updateId.value);
  } catch (e) {
    handleApiError(e, toast);
    router.push(`/apps/${appSlug.value}/updates`);
  } finally {
    loading.value = false;
  }
}

onMounted(load);

async function publish(): Promise<void> {
  actionLoading.value = true;
  try {
    await updatesApi.publishUpdate(appSlug.value, updateId.value);
    toast.add({ title: 'Update published', color: 'success', duration: 3000 });
    await load();
  } catch (e) {
    handleApiError(e, toast);
  } finally {
    actionLoading.value = false;
  }
}

async function republish(): Promise<void> {
  actionLoading.value = true;
  try {
    const resp = await updatesApi.rollbackUpdate(appSlug.value, updateId.value);
    toast.add({ title: 'Draft created from previous update', color: 'success', duration: 3000 });
    republishOpen.value = false;
    await router.push(`/apps/${appSlug.value}/updates/${resp.updateId}`);
  } catch (e) {
    handleApiError(e, toast);
  } finally {
    actionLoading.value = false;
  }
}

async function remove(): Promise<void> {
  actionLoading.value = true;
  try {
    await updatesApi.deleteUpdate(appSlug.value, updateId.value);
    toast.add({ title: 'Update deleted', color: 'success', duration: 3000 });
    deleteOpen.value = false;
    await router.push(`/apps/${appSlug.value}/updates`);
  } catch (e) {
    handleApiError(e, toast);
  } finally {
    actionLoading.value = false;
  }
}

const assetColumns = [
  { accessorKey: 'key', header: 'Key' },
  { accessorKey: 'sha256', header: 'SHA256' },
  { accessorKey: 'size', header: 'Size' },
  { accessorKey: 'url', header: 'URL' },
];
</script>

<template>
  <div v-if="loading" class="space-y-4">
    <USkeleton class="h-8 w-64" />
    <div class="grid grid-cols-2 md:grid-cols-3 gap-4">
      <USkeleton v-for="i in 6" :key="i" class="h-24" />
    </div>
  </div>

  <div v-else-if="detail">
    <div class="flex flex-wrap items-start justify-between gap-4 mb-6">
      <div>
        <div class="flex items-center gap-2 mb-1">
          <UBadge :color="detail.status === 'published' ? 'success' : 'warning'" variant="subtle">
            {{ detail.status }}
          </UBadge>
          <span class="text-muted">{{ detail.platform }} · {{ detail.runtimeVersion }}</span>
        </div>
        <h2 class="text-xl font-semibold text-default font-mono">{{ detail.manifestUuid }}</h2>
        <p v-if="detail.message" class="text-muted mt-1">{{ detail.message }}</p>
      </div>
      <div class="flex flex-wrap gap-2">
        <UButton
          v-if="detail.status === 'pending'"
          label="Publish"
          icon="i-lucide-upload"
          :loading="actionLoading"
          @click="publish"
        />
        <UButton
          v-if="detail.status === 'published'"
          label="Republish Previous"
          color="neutral"
          variant="outline"
          @click="republishOpen = true"
        />
        <UButton label="Delete" color="error" variant="outline" @click="deleteOpen = true" />
      </div>
    </div>

    <div class="grid md:grid-cols-2 gap-6 mb-8">
      <UCard variant="subtle">
        <h3 class="font-medium text-default mb-3">Metadata</h3>
        <dl class="space-y-2 text-sm">
          <div class="flex justify-between gap-4">
            <dt class="text-muted">Update ID</dt>
            <dd class="font-mono text-xs">{{ truncateMiddle(detail.id) }}</dd>
          </div>
          <div class="flex justify-between gap-4">
            <dt class="text-muted">Created</dt>
            <dd><TimeAgo :iso="detail.createdAt" /></dd>
          </div>
          <div v-if="detail.publishedAt" class="flex justify-between gap-4">
            <dt class="text-muted">Published</dt>
            <dd><TimeAgo :iso="detail.publishedAt" /></dd>
          </div>
          <div v-if="detail.gitCommitHash" class="flex justify-between gap-4">
            <dt class="text-muted">Git commit</dt>
            <dd class="font-mono text-xs">{{ detail.gitCommitHash }}</dd>
          </div>
          <div class="flex justify-between gap-4 items-center">
            <dt class="text-muted">Launch asset</dt>
            <dd class="flex items-center gap-1">
              <a
                :href="detail.launchAssetUrl"
                target="_blank"
                rel="noopener"
                class="text-primary text-xs truncate min-w-0 max-w-xs"
              >
                {{ detail.launchAssetKey }}
              </a>
              <CopyButton :value="detail.launchAssetUrl" />
            </dd>
          </div>
        </dl>
      </UCard>

      <div>
        <h3 class="font-medium text-default mb-3">Statistics</h3>
        <EmptyState
          v-if="detail.status !== 'published'"
          title="Not published yet"
          description="Stats appear after this update is published and clients request it."
          icon="i-lucide-bar-chart-2"
        />
        <div v-else class="grid grid-cols-2 gap-3">
          <StatCard
            label="Requested devices"
            :value="detail.stats.requestedDevices"
            :note-value="detail.stats.requestsWithoutDeviceId"
            note-label="requests without device id"
          />
          <StatCard label="Succeeded devices" :value="detail.stats.succeededDevices" />
          <StatCard label="Failed devices" :value="detail.stats.failedDevices" />
          <StatCard label="Min duration" :value="detail.stats.durationMinMs ?? 0" suffix="ms" />
          <StatCard label="Max duration" :value="detail.stats.durationMaxMs ?? 0" suffix="ms" />
          <StatCard label="Avg duration" :value="detail.stats.durationAvgMs ?? 0" suffix="ms" />
        </div>
      </div>
    </div>

    <JsonPreview :data="detail.manifestPreview" class="mb-8" />

    <UAccordion
      :items="[{ label: 'Assets', icon: 'i-lucide-package', slot: 'assets' }]"
      :default-value="[]"
      class="mb-8"
    >
      <template #assets>
        <UCard variant="soft" :ui="{ root: 'border border-default', body: 'p-0 sm:p-0' }">
          <UTable
            :data="detail.assets"
            :columns="assetColumns"
            :ui="{ root: 'max-h-[min(24rem,60vh)]' }"
          >
            <template #sha256-cell="{ row }">
              <span class="font-mono text-xs">{{ truncateMiddle(row.original.sha256, 6, 6) }}</span>
            </template>
            <template #size-cell="{ row }">
              {{ formatBytes(row.original.size) }}
            </template>
            <template #url-cell="{ row }">
              <a
                :href="row.original.url"
                target="_blank"
                rel="noopener"
                class="text-primary text-xs"
              >
                Open
              </a>
            </template>
          </UTable>
        </UCard>
      </template>
    </UAccordion>

    <ConfirmModal
      v-model:open="deleteOpen"
      :title="detail.status === 'pending' ? 'Delete draft update?' : 'Delete this update?'"
      :description="
        detail.status === 'published'
          ? 'Published updates must be at least 3 versions behind in their stream.'
          : undefined
      "
      confirm-label="Delete"
      confirm-color="error"
      :loading="actionLoading"
      @confirm="remove"
    />

    <ConfirmModal
      v-model:open="republishOpen"
      title="Republish Previous?"
      description="Creates a new pending draft from this update. You must Publish it to make it live."
      confirm-label="Create draft"
      :loading="actionLoading"
      @confirm="republish"
    />
  </div>
</template>
