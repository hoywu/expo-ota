<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useRoute } from 'vue-router';
import CopyButton from '@/components/CopyButton.vue';
import TimeAgo from '@/components/TimeAgo.vue';
import ConfirmModal from '@/components/ConfirmModal.vue';
import * as signingApi from '@/api/signing';
import {
  handleApiError,
  signingKeyDeleteEligible,
  signingKeyDeleteCooldownRemaining,
} from '@/utils/format';
import type { SigningKeyResp } from '@/types/admin';

const route = useRoute();
const toast = useToast();

const appSlug = computed(() => route.params.appSlug as string);

const keys = ref<SigningKeyResp[]>([]);
const loading = ref(true);

const generateOpen = ref(false);
const importOpen = ref(false);
const disableOpen = ref(false);
const deleteOpen = ref(false);
const actionKey = ref<SigningKeyResp | null>(null);

const generateKeyId = ref('main');
const importKeyId = ref('main');
const importPublicPem = ref('');
const importPrivatePem = ref('');

const actionLoading = ref(false);

const currentKey = computed(() => keys.value.find((k) => k.enabled) ?? keys.value[0] ?? null);
const hasKey = computed(() => keys.value.length > 0);
const hasEnabledKey = computed(() => keys.value.some((k) => k.enabled));
const canGenerateOrImport = computed(() => !hasEnabledKey.value);
const deleteCooldown = computed(() =>
  signingKeyDeleteCooldownRemaining(actionKey.value?.disabledAt)
);

async function load(): Promise<void> {
  loading.value = true;
  try {
    const resp = await signingApi.listSigningKeys(appSlug.value);
    keys.value = resp.items;
  } catch (e) {
    handleApiError(e, toast);
  } finally {
    loading.value = false;
  }
}

onMounted(load);

async function generate(): Promise<void> {
  if (!generateKeyId.value.trim()) return;
  actionLoading.value = true;
  try {
    await signingApi.generateSigningKey(appSlug.value, {
      keyId: generateKeyId.value.trim(),
    });
    generateOpen.value = false;
    toast.add({ title: 'Signing key generated', color: 'success', duration: 3000 });
    await load();
  } catch (e) {
    handleApiError(e, toast);
  } finally {
    actionLoading.value = false;
  }
}

async function importKey(): Promise<void> {
  if (
    !importKeyId.value.trim() ||
    !importPublicPem.value.trim() ||
    !importPrivatePem.value.trim()
  ) {
    return;
  }
  actionLoading.value = true;
  try {
    await signingApi.importSigningKey(appSlug.value, {
      keyId: importKeyId.value.trim(),
      publicKeyPem: importPublicPem.value.trim(),
      privateKeyPem: importPrivatePem.value.trim(),
    });
    importOpen.value = false;
    toast.add({ title: 'Signing key imported', color: 'success', duration: 3000 });
    await load();
  } catch (e) {
    handleApiError(e, toast);
  } finally {
    actionLoading.value = false;
  }
}

function openDisable(row: SigningKeyResp): void {
  actionKey.value = row;
  disableOpen.value = true;
}

function openDelete(row: SigningKeyResp): void {
  actionKey.value = row;
  deleteOpen.value = true;
}

async function setEnabled(enabled: boolean, keyId: string): Promise<void> {
  actionLoading.value = true;
  try {
    await signingApi.patchSigningKey(appSlug.value, keyId, { enabled });
    disableOpen.value = false;
    toast.add({
      title: enabled ? 'Signing key enabled' : 'Signing key disabled',
      color: 'success',
      duration: 3000,
    });
    await load();
  } catch (e) {
    handleApiError(e, toast);
  } finally {
    actionLoading.value = false;
  }
}

async function removeKey(): Promise<void> {
  if (!actionKey.value) return;
  actionLoading.value = true;
  try {
    await signingApi.deleteSigningKey(appSlug.value, actionKey.value.keyId);
    deleteOpen.value = false;
    toast.add({ title: 'Signing key deleted', color: 'success', duration: 3000 });
    await load();
  } catch (e) {
    handleApiError(e, toast);
  } finally {
    actionLoading.value = false;
  }
}

function downloadPem(key: SigningKeyResp): void {
  const blob = new Blob([key.publicKeyPem], { type: 'application/x-pem-file' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `${appSlug.value}-${key.keyId}-signing-key.pem`;
  a.click();
  URL.revokeObjectURL(url);
}

const columns = [
  { accessorKey: 'keyId', header: 'Key ID' },
  { accessorKey: 'algorithm', header: 'Algorithm' },
  { accessorKey: 'createdAt', header: 'Created' },
  { accessorKey: 'disabledAt', header: 'Disabled' },
  { accessorKey: 'status', header: 'Status' },
  { id: 'actions', header: '' },
];
</script>

<template>
  <div>
    <h2 class="text-2xl font-semibold text-default mb-2">Signing Key</h2>
    <p class="text-sm text-muted mb-6">
      Manage the RSA code signing key for this app. Clients with codeSigningCertificate configured
      will verify manifests.
    </p>

    <div v-if="loading" class="space-y-4">
      <USkeleton class="h-48 w-full" />
    </div>

    <template v-else>
      <UCard v-if="hasKey && currentKey" variant="subtle" class="mb-6">
        <div class="flex flex-wrap items-start justify-between gap-4 mb-4">
          <div>
            <div class="flex items-center gap-2">
              <h3 class="font-medium text-default">{{ currentKey.keyId }}</h3>
              <UBadge :color="currentKey.enabled ? 'success' : 'neutral'" variant="subtle">
                {{ currentKey.enabled ? 'enabled' : 'disabled' }}
              </UBadge>
            </div>
            <p class="text-sm text-muted mt-1">{{ currentKey.algorithm }}</p>
            <p class="text-xs text-muted mt-2">
              Created <TimeAgo :iso="currentKey.createdAt" />
              <span v-if="currentKey.disabledAt">
                · Disabled <TimeAgo :iso="currentKey.disabledAt"
              /></span>
            </p>
          </div>
          <div class="flex flex-wrap gap-2">
            <UButton
              v-if="currentKey.enabled"
              label="Disable"
              color="warning"
              variant="outline"
              @click="openDisable(currentKey)"
            />
            <UButton
              v-else
              label="Enable"
              :loading="actionLoading"
              @click="setEnabled(true, currentKey.keyId)"
            />
            <UTooltip :text="currentKey.disabledAt ? 'Delete disabled key' : 'Disable key first'">
              <UButton
                label="Delete"
                color="error"
                variant="outline"
                :disabled="!signingKeyDeleteEligible(currentKey.disabledAt)"
                @click="openDelete(currentKey)"
              />
            </UTooltip>
          </div>
        </div>

        <UAlert
          v-if="!currentKey.hasPrivateKey"
          color="warning"
          variant="subtle"
          icon="i-lucide-alert-triangle"
          title="Verify-only key"
          description="This key cannot sign manifests. Import or generate a key with a private key."
          class="mb-4"
        />

        <UFormField label="Public key PEM">
          <div class="flex gap-2">
            <UTextarea
              :model-value="currentKey.publicKeyPem"
              readonly
              :rows="8"
              class="font-mono text-xs flex-1"
            />
            <div class="flex flex-col gap-1">
              <CopyButton :value="currentKey.publicKeyPem" label="Copy PEM" />
              <UButton
                icon="i-lucide-download"
                color="neutral"
                variant="ghost"
                aria-label="Download PEM"
                @click="downloadPem(currentKey)"
              />
            </div>
          </div>
        </UFormField>
      </UCard>

      <UAlert
        v-else
        color="neutral"
        variant="subtle"
        icon="i-lucide-shield-off"
        title="No signing key"
        description="Generate or import a key to enable manifest code signing."
        class="mb-6"
      />

      <div v-if="canGenerateOrImport" class="flex flex-wrap gap-2 mb-8">
        <UButton label="Generate key" icon="i-lucide-sparkles" @click="generateOpen = true" />
        <UButton
          label="Import key"
          icon="i-lucide-upload"
          color="neutral"
          variant="outline"
          @click="importOpen = true"
        />
      </div>

      <UCard v-if="keys.length > 0" class="mb-8" :ui="{ body: 'p-0 sm:p-0' }">
        <template #header>
          <h3 class="font-medium text-default">All keys</h3>
        </template>
        <UTable :data="keys" :columns="columns">
          <template #createdAt-cell="{ row }">
            <TimeAgo :iso="row.original.createdAt" />
          </template>
          <template #disabledAt-cell="{ row }">
            <TimeAgo v-if="row.original.disabledAt" :iso="row.original.disabledAt" />
            <span v-else class="text-muted">—</span>
          </template>
          <template #status-cell="{ row }">
            <UBadge :color="row.original.enabled ? 'success' : 'neutral'" variant="subtle">
              {{ row.original.enabled ? 'enabled' : 'disabled' }}
            </UBadge>
          </template>
          <template #actions-cell="{ row }">
            <div class="flex items-center justify-end gap-1">
              <UButton
                v-if="row.original.enabled"
                label="Disable"
                size="xs"
                color="warning"
                variant="ghost"
                :loading="actionLoading"
                @click="openDisable(row.original)"
              />
              <UButton
                v-else
                label="Enable"
                size="xs"
                variant="ghost"
                :loading="actionLoading"
                @click="setEnabled(true, row.original.keyId)"
              />
              <UButton
                label="Delete"
                size="xs"
                color="error"
                variant="ghost"
                :disabled="!signingKeyDeleteEligible(row.original.disabledAt)"
                :loading="actionLoading"
                @click="openDelete(row.original)"
              />
            </div>
          </template>
        </UTable>
      </UCard>

      <UAccordion
        :items="[{ label: 'Client integration', icon: 'i-lucide-book-open', slot: 'guide' }]"
        :default-value="[]"
      >
        <template #guide>
          <ol class="list-decimal list-inside text-sm text-muted space-y-2 p-2">
            <li>Copy the public key PEM from this page</li>
            <li>
              Configure <code class="font-mono text-xs">app.json</code> with
              <code class="font-mono text-xs">updates.codeSigningCertificate</code>
            </li>
            <li>
              Set <code class="font-mono text-xs">codeSigningMetadata.keyid</code> to match your key
              ID
            </li>
            <li>Rebuild the native app</li>
          </ol>
        </template>
      </UAccordion>
    </template>

    <UModal v-model:open="generateOpen" title="Generate signing key" :dismissible="false">
      <template #body>
        <UFormField label="Key ID" required hint="e.g. main">
          <UInput v-model="generateKeyId" />
        </UFormField>
      </template>
      <template #footer>
        <div class="flex justify-end gap-2">
          <UButton label="Cancel" color="neutral" variant="outline" @click="generateOpen = false" />
          <UButton label="Generate" :loading="actionLoading" @click="generate" />
        </div>
      </template>
    </UModal>

    <UModal v-model:open="importOpen" title="Import signing key" :dismissible="false">
      <template #body>
        <div class="space-y-4">
          <UFormField label="Key ID" required>
            <UInput v-model="importKeyId" />
          </UFormField>
          <UFormField label="Public key PEM" required>
            <UTextarea v-model="importPublicPem" :rows="6" class="font-mono text-xs" />
          </UFormField>
          <UFormField label="Private key PEM" required>
            <UTextarea v-model="importPrivatePem" :rows="6" class="font-mono text-xs" />
          </UFormField>
        </div>
      </template>
      <template #footer>
        <div class="flex justify-end gap-2">
          <UButton label="Cancel" color="neutral" variant="outline" @click="importOpen = false" />
          <UButton
            label="Import"
            :loading="actionLoading"
            :disabled="!importKeyId.trim() || !importPublicPem.trim() || !importPrivatePem.trim()"
            @click="importKey"
          />
        </div>
      </template>
    </UModal>

    <ConfirmModal
      v-model:open="disableOpen"
      title="Disable code signing for this app?"
      description="Clients with expo-expect-signature may fail until reconfigured."
      confirm-label="Disable"
      confirm-color="warning"
      :loading="actionLoading"
      @confirm="actionKey && setEnabled(false, actionKey.keyId)"
    />

    <ConfirmModal
      v-model:open="deleteOpen"
      :title="`Permanently delete key '${actionKey?.keyId ?? ''}'?`"
      :description="deleteCooldown ?? undefined"
      confirm-label="Delete"
      confirm-color="error"
      :loading="actionLoading"
      @confirm="removeKey"
    />
  </div>
</template>
