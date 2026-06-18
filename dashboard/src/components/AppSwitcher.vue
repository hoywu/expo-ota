<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useRouter, useRoute } from 'vue-router';
import { useAppsStore } from '@/stores/apps';

const router = useRouter();
const route = useRoute();
const appsStore = useAppsStore();

const loading = ref(false);

onMounted(async () => {
  if (appsStore.items.length === 0) {
    loading.value = true;
    try {
      await appsStore.list();
    } finally {
      loading.value = false;
    }
  }
});

const items = computed(() =>
  appsStore.items.map((app) => ({
    label: app.name,
    value: app.appSlug,
    description: app.appSlug,
  }))
);

const currentSlug = computed(() => route.params.appSlug as string | undefined);

function switchApp(slug: string): void {
  if (!slug || slug === currentSlug.value) return;

  const path = route.path;
  if (currentSlug.value && path.includes(`/apps/${currentSlug.value}`)) {
    router.push(path.replace(`/apps/${currentSlug.value}`, `/apps/${slug}`));
  } else {
    router.push(`/apps/${slug}/updates`);
  }
}
</script>

<template>
  <USelectMenu
    v-if="items.length > 0"
    :items="items"
    :model-value="currentSlug"
    value-key="value"
    placeholder="Select app"
    :loading="loading"
    class="dashboard-filter-select-md w-auto shrink-0 min-w-56"
    @update:model-value="switchApp($event as string)"
  />
</template>
