<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';
import TimeAgo from '@/components/TimeAgo.vue';
import EmptyState from '@/components/EmptyState.vue';
import ConfirmModal from '@/components/ConfirmModal.vue';
import { useAuthStore } from '@/stores/auth';
import * as usersApi from '@/api/users';
import { handleApiError } from '@/utils/format';
import { normalizeUsername, validatePassword } from '@/utils/validation';
import type { UserItem } from '@/types/admin';

const auth = useAuthStore();
const router = useRouter();
const toast = useToast();

const items = ref<UserItem[]>([]);
const loading = ref(true);

const createOpen = ref(false);
const createUsername = ref('');
const createPassword = ref('');
const createLoading = ref(false);

const passwordOpen = ref(false);
const passwordTarget = ref<UserItem | null>(null);
const newPassword = ref('');
const passwordLoading = ref(false);

const disableOpen = ref(false);
const disableTarget = ref<UserItem | null>(null);
const disableLoading = ref(false);

async function load(): Promise<void> {
  loading.value = true;
  try {
    const resp = await usersApi.listUsers();
    items.value = resp.items;
  } catch (e) {
    handleApiError(e, toast);
  } finally {
    loading.value = false;
  }
}

onMounted(load);

async function createUser(): Promise<void> {
  const pwError = validatePassword(createPassword.value);
  if (!createUsername.value.trim() || pwError) {
    toast.add({ title: pwError ?? 'Username is required', color: 'error', duration: 5000 });
    return;
  }
  createLoading.value = true;
  try {
    await usersApi.createUser({
      username: normalizeUsername(createUsername.value),
      password: createPassword.value,
    });
    toast.add({ title: 'User created', color: 'success', duration: 3000 });
    createOpen.value = false;
    createUsername.value = '';
    createPassword.value = '';
    await load();
  } catch (e) {
    handleApiError(e, toast);
  } finally {
    createLoading.value = false;
  }
}

function openPassword(row: UserItem): void {
  passwordTarget.value = row;
  newPassword.value = '';
  passwordOpen.value = true;
}

async function savePassword(): Promise<void> {
  const pwError = validatePassword(newPassword.value);
  if (!passwordTarget.value || pwError) {
    toast.add({ title: pwError ?? 'Invalid password', color: 'error', duration: 5000 });
    return;
  }
  passwordLoading.value = true;
  try {
    await usersApi.changePassword(passwordTarget.value.id, { password: newPassword.value });
    toast.add({ title: 'Password updated', color: 'success', duration: 3000 });
    passwordOpen.value = false;
  } catch (e) {
    handleApiError(e, toast);
  } finally {
    passwordLoading.value = false;
  }
}

function openDisable(row: UserItem): void {
  disableTarget.value = row;
  disableOpen.value = true;
}

async function confirmDisable(): Promise<void> {
  if (!disableTarget.value) return;
  disableLoading.value = true;
  try {
    await usersApi.disableUser(disableTarget.value.id);
    toast.add({ title: 'User disabled', color: 'success', duration: 3000 });
    disableOpen.value = false;
    if (disableTarget.value.id === auth.user?.userId) {
      await auth.logout();
      await router.push('/login');
      return;
    }
    await load();
  } catch (e) {
    handleApiError(e, toast);
  } finally {
    disableLoading.value = false;
  }
}

async function enableUser(row: UserItem): Promise<void> {
  try {
    await usersApi.enableUser(row.id);
    toast.add({ title: 'User enabled', color: 'success', duration: 3000 });
    await load();
  } catch (e) {
    handleApiError(e, toast);
  }
}

const columns = [
  { accessorKey: 'username', header: 'Username' },
  { accessorKey: 'createdAt', header: 'Created' },
  { accessorKey: 'lastLoginAt', header: 'Last login' },
  { accessorKey: 'status', header: 'Status' },
  { id: 'actions', header: '' },
];
</script>

<template>
  <div>
    <div class="flex items-center justify-end mb-6">
      <UButton label="Create user" icon="i-lucide-user-plus" @click="createOpen = true" />
    </div>

    <div v-if="loading" class="space-y-2">
      <USkeleton v-for="i in 4" :key="i" class="h-12 w-full" />
    </div>

    <EmptyState
      v-else-if="items.length === 0"
      title="No users"
      description="Create an admin account."
      icon="i-lucide-users"
    />

    <UCard v-else variant="subtle" :ui="{ body: 'p-0 sm:p-0' }">
      <UTable :data="items" :columns="columns">
        <template #createdAt-cell="{ row }">
          <TimeAgo :iso="row.original.createdAt" />
        </template>
        <template #lastLoginAt-cell="{ row }">
          <TimeAgo v-if="row.original.lastLoginAt" :iso="row.original.lastLoginAt" />
          <span v-else class="text-muted">—</span>
        </template>
        <template #status-cell="{ row }">
          <UBadge v-if="row.original.disabledAt" color="error" variant="subtle">disabled</UBadge>
          <UBadge v-else color="success" variant="subtle">active</UBadge>
        </template>
        <template #actions-cell="{ row }">
          <div class="flex justify-end gap-1">
            <UButton
              label="Change password"
              size="xs"
              color="neutral"
              variant="ghost"
              @click="openPassword(row.original)"
            />
            <UButton
              v-if="!row.original.disabledAt"
              label="Disable"
              size="xs"
              color="error"
              variant="ghost"
              @click="openDisable(row.original)"
            />
            <UButton v-else label="Enable" size="xs" @click="enableUser(row.original)" />
          </div>
        </template>
      </UTable>
    </UCard>

    <UModal v-model:open="createOpen" title="Create user">
      <template #body>
        <div class="space-y-4">
          <UFormField label="Username" required>
            <UInput v-model="createUsername" autocomplete="off" class="form-control" />
          </UFormField>
          <UFormField label="Password" required hint="At least 10 chars with letters and numbers">
            <UInput
              v-model="createPassword"
              type="password"
              autocomplete="new-password"
              class="form-control"
            />
          </UFormField>
        </div>
      </template>
      <template #footer>
        <div class="flex justify-end gap-2">
          <UButton label="Cancel" color="neutral" variant="outline" @click="createOpen = false" />
          <UButton label="Create" :loading="createLoading" @click="createUser" />
        </div>
      </template>
    </UModal>

    <UModal v-model:open="passwordOpen" :title="`Change password for ${passwordTarget?.username}`">
      <template #body>
        <UFormField label="New password">
          <UInput
            v-model="newPassword"
            type="password"
            autocomplete="new-password"
            class="form-control"
          />
        </UFormField>
      </template>
      <template #footer>
        <div class="flex justify-end gap-2">
          <UButton label="Cancel" color="neutral" variant="outline" @click="passwordOpen = false" />
          <UButton label="Save" :loading="passwordLoading" @click="savePassword" />
        </div>
      </template>
    </UModal>

    <ConfirmModal
      v-model:open="disableOpen"
      :title="`Disable user &quot;${disableTarget?.username}&quot;?`"
      :description="disableTarget?.id === auth.user?.userId ? 'You will be logged out.' : undefined"
      confirm-label="Disable"
      confirm-color="error"
      :loading="disableLoading"
      @confirm="confirmDisable"
    />
  </div>
</template>
