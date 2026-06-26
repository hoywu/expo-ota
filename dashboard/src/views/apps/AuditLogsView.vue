<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import { useRoute } from 'vue-router';
import TimeAgo from '@/components/TimeAgo.vue';
import CopyButton from '@/components/CopyButton.vue';
import EmptyState from '@/components/EmptyState.vue';
import * as auditApi from '@/api/audit';
import { AUDIT_ACTIONS, handleApiError } from '@/utils/format';
import type { AuditLogItem } from '@/types/admin';

const route = useRoute();
const toast = useToast();

const appSlug = computed(() => route.params.appSlug as string);

const items = ref<AuditLogItem[]>([]);
const nextCursor = ref<string | undefined>();
const loading = ref(true);
const loadingMore = ref(false);

const action = ref<(typeof AUDIT_ACTIONS)[number] | undefined>();
const actor = ref('');
const from = ref('');
const to = ref('');

const actionOptions = [
  { label: 'All actions', value: undefined },
  ...AUDIT_ACTIONS.map((a) => ({ label: a, value: a })),
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
    const resp = await auditApi.listAuditLogs(appSlug.value, {
      action: action.value,
      actor: actor.value.trim() || undefined,
      from: from.value ? new Date(from.value).toISOString() : undefined,
      to: to.value ? new Date(to.value).toISOString() : undefined,
      limit: 50,
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

watch(action, () => load(true));

onMounted(() => load(true));

function applyFilters(): void {
  load(true);
}

const payloadOpen = ref(false);
const payloadTarget = ref<AuditLogItem | null>(null);

const payloadFormatted = computed(() =>
  payloadTarget.value?.payload ? JSON.stringify(payloadTarget.value.payload, null, 2) : ''
);

function openPayload(row: AuditLogItem): void {
  payloadTarget.value = row;
  payloadOpen.value = true;
}

const columns = [
  { accessorKey: 'occurredAt', header: 'Time' },
  { accessorKey: 'action', header: 'Action' },
  { accessorKey: 'actorUserId', header: 'Actor' },
  { accessorKey: 'targetType', header: 'Target' },
  { accessorKey: 'ip', header: 'IP' },
  { id: 'payload', header: 'Payload' },
];
</script>

<template>
  <div>
    <UDashboardToolbar class="mb-4 px-0">
      <template #left>
        <div class="dashboard-filters">
          <USelectMenu
            :items="actionOptions"
            :model-value="action"
            value-key="value"
            placeholder="Action"
            class="dashboard-filter-select-md"
            @update:model-value="action = $event as (typeof AUDIT_ACTIONS)[number] | undefined"
          />
          <UInput v-model="actor" placeholder="Actor user ID" class="dashboard-filter-input" />
          <UInput v-model="from" type="datetime-local" class="dashboard-filter-datetime" />
          <UInput v-model="to" type="datetime-local" class="dashboard-filter-datetime" />
          <UButton label="Apply" variant="outline" @click="applyFilters" />
        </div>
      </template>
    </UDashboardToolbar>

    <div v-if="loading" class="space-y-2">
      <USkeleton v-for="i in 8" :key="i" class="h-12 w-full" />
    </div>

    <EmptyState
      v-else-if="items.length === 0"
      title="No audit logs"
      description="Management write actions for this app will appear here."
      icon="i-lucide-scroll-text"
    />

    <UCard v-else variant="subtle" :ui="{ body: 'p-0 sm:p-0' }">
      <UTable :data="items" :columns="columns">
        <template #occurredAt-cell="{ row }">
          <TimeAgo :iso="row.original.occurredAt" />
        </template>
        <template #actorUserId-cell="{ row }">
          <span class="font-mono text-xs">{{ row.original.actorUserId ?? '—' }}</span>
        </template>
        <template #targetType-cell="{ row }">
          <span v-if="row.original.targetType" class="text-xs">
            {{ row.original.targetType }}
            <span v-if="row.original.targetId" class="font-mono text-muted">
              / {{ row.original.targetId }}</span
            >
          </span>
          <span v-else class="text-muted">—</span>
        </template>
        <template #ip-cell="{ row }">
          <span class="text-xs">{{ row.original.ip ?? '—' }}</span>
        </template>
        <template #payload-cell="{ row }">
          <UButton
            v-if="row.original.payload && Object.keys(row.original.payload).length > 0"
            label="View"
            icon="i-lucide-braces"
            size="xs"
            color="neutral"
            variant="ghost"
            @click="openPayload(row.original)"
          />
          <span v-else class="text-muted">—</span>
        </template>
      </UTable>
    </UCard>

    <div v-if="nextCursor" class="mt-4 text-center">
      <UButton label="Load more" :loading="loadingMore" variant="outline" @click="load(false)" />
    </div>

    <UModal v-model:open="payloadOpen" title="Payload" :description="payloadTarget?.action">
      <template #body>
        <div class="flex justify-end mb-2">
          <CopyButton :value="payloadFormatted" label="Copy payload" />
        </div>
        <pre
          class="text-xs font-mono overflow-auto p-3 bg-elevated rounded-md text-default max-h-[min(24rem,60vh)]"
          >{{ payloadFormatted }}</pre
        >
      </template>
      <template #footer>
        <UButton label="Close" color="neutral" variant="outline" @click="payloadOpen = false" />
      </template>
    </UModal>
  </div>
</template>
