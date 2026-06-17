<script setup lang="ts">
defineProps<{
  title: string;
  description?: string;
  icon?: string;
  actionLabel?: string;
}>();

defineEmits<{
  action: [];
}>();
</script>

<template>
  <div class="flex flex-col items-center justify-center py-16 text-center">
    <UIcon :name="icon ?? 'i-lucide-inbox'" class="size-12 text-muted mb-4" />
    <h3 class="text-lg font-medium text-default">{{ title }}</h3>
    <p v-if="description" class="text-sm text-muted mt-1 max-w-md">{{ description }}</p>
    <UButton v-if="actionLabel" class="mt-4" :label="actionLabel" @click="$emit('action')" />
  </div>
</template>
