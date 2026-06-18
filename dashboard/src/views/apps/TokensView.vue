<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useRoute } from 'vue-router';
import CopyButton from '@/components/CopyButton.vue';
import TimeAgo from '@/components/TimeAgo.vue';
import EmptyState from '@/components/EmptyState.vue';
import ConfirmModal from '@/components/ConfirmModal.vue';
import * as tokensApi from '@/api/tokens';
import { handleApiError, datetimeLocalToRFC3339 } from '@/utils/format';
import type { TokenItem } from '@/types/admin';

const route = useRoute();
const toast = useToast();

const appSlug = computed(() => route.params.appSlug as string);

const items = ref<TokenItem[]>([]);
const loading = ref(true);

const createOpen = ref(false);
const createName = ref('');
const createExpires = ref('');
const createLoading = ref(false);

const createdToken = ref<string | null>(null);
const tokenModalOpen = ref(false);

const revokeOpen = ref(false);
const revokeTarget = ref<TokenItem | null>(null);
const revokeLoading = ref(false);

async function load(): Promise<void> {
  loading.value = true;
  try {
    const resp = await tokensApi.listTokens(appSlug.value);
    items.value = resp.items;
  } catch (e) {
    handleApiError(e, toast);
  } finally {
    loading.value = false;
  }
}

onMounted(load);

async function createToken(): Promise<void> {
  if (!createName.value.trim()) {
    toast.add({ title: 'Name is required', color: 'error', duration: 5000 });
    return;
  }
  createLoading.value = true;
  try {
    const resp = await tokensApi.createToken(appSlug.value, {
      name: createName.value.trim(),
      expiresAt: datetimeLocalToRFC3339(createExpires.value),
    });
    createdToken.value = resp.token;
    tokenModalOpen.value = true;
    createOpen.value = false;
    createName.value = '';
    createExpires.value = '';
    toast.add({ title: 'Token created', color: 'success', duration: 3000 });
    await load();
  } catch (e) {
    handleApiError(e, toast);
  } finally {
    createLoading.value = false;
  }
}

function openRevoke(row: TokenItem): void {
  revokeTarget.value = row;
  revokeOpen.value = true;
}

async function confirmRevoke(): Promise<void> {
  if (!revokeTarget.value) return;
  revokeLoading.value = true;
  try {
    await tokensApi.revokeToken(appSlug.value, revokeTarget.value.id);
    toast.add({ title: 'Token revoked', color: 'success', duration: 3000 });
    revokeOpen.value = false;
    await load();
  } catch (e) {
    handleApiError(e, toast);
  } finally {
    revokeLoading.value = false;
  }
}

const columns = [
  { accessorKey: 'name', header: 'Name' },
  { accessorKey: 'scopes', header: 'Scopes' },
  { accessorKey: 'createdBy', header: 'Created by' },
  { accessorKey: 'lastUsedAt', header: 'Last used' },
  { accessorKey: 'expiresAt', header: 'Expires' },
  { accessorKey: 'createdAt', header: 'Created' },
  { accessorKey: 'status', header: 'Status' },
  { id: 'actions', header: '' },
];
</script>

<template>
  <div>
    <div class="flex items-center justify-between mb-6">
      <div>
        <h2 class="text-2xl font-semibold text-default">API Tokens</h2>
        <p class="text-sm text-muted mt-1">
          Tokens are for CI plan / finalize only. Publish pending updates from the Updates page.
          Plaintext is shown once at creation.
        </p>
      </div>
      <UButton label="Create token" icon="i-lucide-plus" @click="createOpen = true" />
    </div>

    <UAlert
      color="neutral"
      variant="subtle"
      icon="i-lucide-terminal"
      title="CI integration"
      class="mb-6 font-mono text-xs"
    >
      <pre class="whitespace-pre-wrap">
OTA_API=https://ota.example.com OTA_TOKEN=ota_pat_xxx OTA_APP_SLUG={{ appSlug }} ...
bun run cli/publish.ts</pre
      >
    </UAlert>

    <div v-if="loading" class="space-y-2">
      <USkeleton v-for="i in 4" :key="i" class="h-12 w-full" />
    </div>

    <EmptyState
      v-else-if="items.length === 0"
      title="No API tokens"
      description="Create a token for your CI pipeline to upload updates (plan / finalize)."
      action-label="Create token"
      icon="i-lucide-key"
      @action="createOpen = true"
    />

    <UTable v-else :data="items" :columns="columns">
      <template #scopes-cell="{ row }">
        {{ row.original.scopes.join(', ') }}
      </template>
      <template #lastUsedAt-cell="{ row }">
        <TimeAgo v-if="row.original.lastUsedAt" :iso="row.original.lastUsedAt" />
        <span v-else class="text-muted">—</span>
      </template>
      <template #expiresAt-cell="{ row }">
        <TimeAgo v-if="row.original.expiresAt" :iso="row.original.expiresAt" />
        <span v-else class="text-muted">Never</span>
      </template>
      <template #createdAt-cell="{ row }">
        <TimeAgo :iso="row.original.createdAt" />
      </template>
      <template #status-cell="{ row }">
        <UBadge v-if="row.original.revokedAt" color="neutral" variant="subtle">revoked</UBadge>
        <UBadge v-else color="success" variant="subtle">active</UBadge>
      </template>
      <template #actions-cell="{ row }">
        <UButton
          v-if="!row.original.revokedAt"
          label="Revoke"
          size="xs"
          color="error"
          variant="ghost"
          @click="openRevoke(row.original)"
        />
      </template>
    </UTable>

    <UModal v-model:open="createOpen" title="Create API token" :dismissible="false">
      <template #body>
        <div class="space-y-4">
          <UFormField label="Name" required>
            <UInput v-model="createName" placeholder="CI production" />
          </UFormField>
          <UFormField label="Expires at (optional)">
            <UInput v-model="createExpires" type="datetime-local" />
          </UFormField>
        </div>
      </template>
      <template #footer>
        <div class="flex justify-end gap-2">
          <UButton label="Cancel" color="neutral" variant="outline" @click="createOpen = false" />
          <UButton label="Create" :loading="createLoading" @click="createToken" />
        </div>
      </template>
    </UModal>

    <UModal
      v-model:open="tokenModalOpen"
      title="Save your token"
      :dismissible="false"
      :close="false"
    >
      <template #body>
        <UAlert
          color="warning"
          variant="subtle"
          icon="i-lucide-alert-triangle"
          title="Copy this token now"
          description="You won't be able to see it again."
          class="mb-4"
        />
        <div class="flex items-center gap-2">
          <code class="flex-1 text-xs font-mono bg-elevated p-3 rounded break-all">{{
            createdToken
          }}</code>
          <CopyButton v-if="createdToken" :value="createdToken" />
        </div>
      </template>
      <template #footer>
        <UButton
          label="I've copied it"
          block
          @click="
            tokenModalOpen = false;
            createdToken = null;
          "
        />
      </template>
    </UModal>

    <ConfirmModal
      v-model:open="revokeOpen"
      :title="`Revoke token &quot;${revokeTarget?.name}&quot;?`"
      description="CI jobs using this token will fail."
      confirm-label="Revoke"
      confirm-color="error"
      :loading="revokeLoading"
      @confirm="confirmRevoke"
    />
  </div>
</template>
