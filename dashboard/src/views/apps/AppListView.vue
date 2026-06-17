<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import EmptyState from '@/components/EmptyState.vue';
import TimeAgo from '@/components/TimeAgo.vue';
import ConfirmModal from '@/components/ConfirmModal.vue';
import { useAppsStore } from '@/stores/apps';
import { handleApiError } from '@/utils/format';

const router = useRouter();
const route = useRoute();
const appsStore = useAppsStore();
const toast = useToast();

const loading = ref(true);
const editOpen = ref(false);
const deleteOpen = ref(false);
const deleteLoading = ref(false);
const editLoading = ref(false);
const selectedSlug = ref<string | null>(null);

const editName = ref('');
const editDescription = ref('');

onMounted(async () => {
  if (route.query.flash === 'app-not-found') {
    toast.add({ title: 'App not found', color: 'error', duration: 5000 });
    router.replace({ query: {} });
  }
  try {
    await appsStore.list();
  } catch (e) {
    handleApiError(e, toast);
  } finally {
    loading.value = false;
  }
});

function openEdit(app: (typeof appsStore.items)[0]): void {
  selectedSlug.value = app.appSlug;
  editName.value = app.name;
  editDescription.value = app.description ?? '';
  editOpen.value = true;
}

async function saveEdit(): Promise<void> {
  if (!selectedSlug.value || !editName.value.trim()) return;
  editLoading.value = true;
  try {
    await appsStore.update(selectedSlug.value, {
      name: editName.value.trim(),
      description: editDescription.value.trim() || undefined,
    });
    toast.add({ title: 'App updated', color: 'success', duration: 3000 });
    editOpen.value = false;
  } catch (e) {
    handleApiError(e, toast);
  } finally {
    editLoading.value = false;
  }
}

function openDelete(slug: string): void {
  selectedSlug.value = slug;
  deleteOpen.value = true;
}

async function confirmDelete(): Promise<void> {
  if (!selectedSlug.value) return;
  deleteLoading.value = true;
  try {
    await appsStore.remove(selectedSlug.value);
    toast.add({ title: 'App deleted', color: 'success', duration: 3000 });
    deleteOpen.value = false;
  } catch (e) {
    handleApiError(e, toast);
  } finally {
    deleteLoading.value = false;
  }
}

const columns = [
  { accessorKey: 'appSlug', header: 'App Slug' },
  { accessorKey: 'name', header: 'Name' },
  { accessorKey: 'description', header: 'Description' },
  { accessorKey: 'createdAt', header: 'Created' },
  { id: 'actions', header: '' },
];
</script>

<template>
  <div>
    <div class="flex items-center justify-between mb-6">
      <h2 class="text-2xl font-semibold text-default">Apps</h2>
      <UButton label="New App" icon="i-lucide-plus" to="/apps/new" />
    </div>

    <div v-if="loading" class="space-y-2">
      <USkeleton v-for="i in 5" :key="i" class="h-12 w-full" />
    </div>

    <EmptyState
      v-else-if="appsStore.items.length === 0"
      title="No apps yet"
      description="Create your first app to start publishing OTA updates."
      action-label="Create App"
      icon="i-lucide-layout-grid"
      @action="router.push('/apps/new')"
    />

    <UTable v-else :data="appsStore.items" :columns="columns">
      <template #appSlug-cell="{ row }">
        <button
          type="button"
          class="font-mono text-sm text-primary hover:underline"
          @click="router.push(`/apps/${row.original.appSlug}/updates`)"
        >
          {{ row.original.appSlug }}
        </button>
      </template>
      <template #description-cell="{ row }">
        <span class="text-muted truncate max-w-xs block">{{
          row.original.description || '—'
        }}</span>
      </template>
      <template #createdAt-cell="{ row }">
        <TimeAgo :iso="row.original.createdAt" />
      </template>
      <template #actions-cell="{ row }">
        <div class="flex justify-end gap-1" @click.stop>
          <UButton
            icon="i-lucide-pencil"
            color="neutral"
            variant="ghost"
            size="xs"
            aria-label="Edit app"
            @click="openEdit(row.original)"
          />
          <UButton
            icon="i-lucide-trash-2"
            color="error"
            variant="ghost"
            size="xs"
            aria-label="Delete app"
            @click="openDelete(row.original.appSlug)"
          />
        </div>
      </template>
    </UTable>

    <USlideover v-model:open="editOpen" title="Edit App">
      <template #body>
        <div class="space-y-4 p-4">
          <UFormField label="App Slug">
            <UInput :model-value="selectedSlug ?? ''" disabled />
          </UFormField>
          <UFormField label="Name" required>
            <UInput v-model="editName" />
          </UFormField>
          <UFormField label="Description">
            <UTextarea v-model="editDescription" :rows="3" />
          </UFormField>
        </div>
      </template>
      <template #footer>
        <div class="flex justify-end gap-2 p-4">
          <UButton label="Cancel" color="neutral" variant="outline" @click="editOpen = false" />
          <UButton label="Save" :loading="editLoading" @click="saveEdit" />
        </div>
      </template>
    </USlideover>

    <ConfirmModal
      v-model:open="deleteOpen"
      :title="`Delete app &quot;${selectedSlug}&quot;?`"
      description="This soft-deletes the app. The slug cannot be reused and manifest requests will return 404."
      confirm-label="Delete"
      confirm-color="error"
      :loading="deleteLoading"
      @confirm="confirmDelete"
    />
  </div>
</template>
