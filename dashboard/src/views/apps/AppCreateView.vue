<script setup lang="ts">
import { ref } from 'vue';
import { useRouter } from 'vue-router';
import { useAppsStore } from '@/stores/apps';
import { validateAppSlug } from '@/utils/validation';
import { handleApiError, manifestUrl } from '@/utils/format';
import { ApiError } from '@/api/client';

const router = useRouter();
const appsStore = useAppsStore();
const toast = useToast();

const appSlug = ref('');
const name = ref('');
const description = ref('');
const slugError = ref<string | null>(null);
const loading = ref(false);

function validateSlug(): void {
  slugError.value = validateAppSlug(appSlug.value);
}

async function submit(): Promise<void> {
  validateSlug();
  if (slugError.value || !name.value.trim()) {
    if (!name.value.trim()) {
      toast.add({ title: 'Name is required', color: 'error', duration: 5000 });
    }
    return;
  }

  loading.value = true;
  try {
    const app = await appsStore.create({
      appSlug: appSlug.value.trim(),
      name: name.value.trim(),
      description: description.value.trim() || undefined,
    });
    toast.add({ title: 'App created', color: 'success', duration: 3000 });
    await router.push(`/apps/${app.appSlug}/updates`);
  } catch (e) {
    if (e instanceof ApiError && e.status === 409) {
      slugError.value = 'This slug is already taken';
    } else {
      handleApiError(e, toast);
    }
  } finally {
    loading.value = false;
  }
}

const previewUrl = () => (appSlug.value ? manifestUrl(appSlug.value.trim()) : '');
</script>

<template>
  <div class="max-w-xl">
    <UAlert
      color="info"
      variant="subtle"
      icon="i-lucide-info"
      title="Slug is permanent"
      description="App slug cannot be changed after creation. Client updates.url will point to the manifest endpoint."
      class="mb-6"
    />

    <form class="space-y-4" @submit.prevent="submit">
      <UFormField
        label="App Slug"
        required
        :error="slugError ?? undefined"
        hint="3–32 chars, lowercase"
      >
        <UInput
          v-model="appSlug"
          placeholder="my-app"
          class="font-mono form-control"
          @blur="validateSlug"
          @input="slugError = null"
        />
      </UFormField>

      <UFormField label="Name" required>
        <UInput v-model="name" placeholder="My App" class="form-control" />
      </UFormField>

      <UFormField label="Description">
        <UTextarea v-model="description" :rows="3" class="form-control" />
      </UFormField>

      <UFormField v-if="appSlug" label="Manifest URL (preview)">
        <UInput :model-value="previewUrl()" readonly class="font-mono text-xs form-control" />
      </UFormField>

      <div class="flex gap-2 pt-2">
        <UButton label="Create" type="submit" :loading="loading" />
        <UButton label="Cancel" color="neutral" variant="outline" to="/apps" />
      </div>
    </form>
  </div>
</template>
