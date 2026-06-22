<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue';
import { useToast } from '@nuxt/ui/composables/useToast';
import StatCard from '@/components/StatCard.vue';
import EmptyState from '@/components/EmptyState.vue';
import * as observabilityApi from '@/api/observability';
import { handleApiError } from '@/utils/format';
import type { SystemStatsResp } from '@/types/admin';

const toast = useToast();

const stats = ref<SystemStatsResp | null>(null);
const loading = ref(true);
const error = ref(false);

const POLL_INTERVAL_MS = 5000;
let pollTimer: ReturnType<typeof setInterval> | null = null;

async function load(): Promise<void> {
  try {
    stats.value = await observabilityApi.getSystemStats();
    error.value = false;
  } catch (e) {
    // Only toast on state transition to avoid spamming during sustained outage.
    if (!error.value) {
      handleApiError(e, toast);
    }
    error.value = true;
  } finally {
    loading.value = false;
  }
}

onMounted(() => {
  load();
  pollTimer = setInterval(load, POLL_INTERVAL_MS);
});

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer);
});

function toMB(bytes: number): number {
  return Math.round((bytes / 1024 / 1024) * 10) / 10;
}
</script>

<template>
  <div>
    <div v-if="loading" class="space-y-4">
      <USkeleton class="h-10 w-full" />
      <div class="grid grid-cols-2 md:grid-cols-3 gap-4">
        <USkeleton v-for="i in 6" :key="i" class="h-24" />
      </div>
    </div>

    <template v-else-if="stats">
      <UCard variant="subtle" class="mb-6">
        <div class="flex flex-wrap items-center gap-3 text-sm">
          <UIcon name="i-lucide-server" class="size-4 text-muted shrink-0" />
          <span class="text-muted">admin-api</span>
          <UBadge :color="error ? 'error' : 'success'" variant="subtle">
            {{ error ? 'degraded' : 'live' }}
          </UBadge>
          <span class="text-muted">·</span>
          <span class="font-mono text-xs">{{ stats.goVersion }}</span>
          <span class="text-muted">·</span>
          <span class="text-muted text-xs">refreshes every 5s</span>
        </div>
      </UCard>

      <div class="grid grid-cols-2 md:grid-cols-3 gap-4">
        <StatCard label="Heap alloc" :value="toMB(stats.heapAllocBytes)" suffix="MB" />
        <StatCard label="Heap in-use" :value="toMB(stats.heapInUseBytes)" suffix="MB" />
        <StatCard label="Stack in-use" :value="toMB(stats.stackInUseBytes)" suffix="MB" />
        <StatCard label="Goroutines" :value="stats.numGoroutine" />
        <StatCard label="GC cycles" :value="stats.numGC" />
        <StatCard label="Uptime" :value="stats.uptimeSeconds" suffix="s" />
      </div>
    </template>

    <EmptyState
      v-else
      title="Failed to load stats"
      description="The admin-api may be unreachable. Refresh to retry."
      icon="i-lucide-alert-triangle"
    />
  </div>
</template>
