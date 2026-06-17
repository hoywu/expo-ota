<script setup lang="ts">
defineProps<{
  open: boolean;
  title: string;
  description?: string;
  confirmLabel?: string;
  confirmColor?: 'primary' | 'error' | 'warning' | 'neutral';
  loading?: boolean;
}>();

const emit = defineEmits<{
  'update:open': [value: boolean];
  confirm: [];
  cancel: [];
}>();

function close(): void {
  emit('update:open', false);
  emit('cancel');
}
</script>

<template>
  <UModal
    :open="open"
    :title="title"
    :description="description"
    @update:open="emit('update:open', $event)"
  >
    <template #footer>
      <div class="flex justify-end gap-2">
        <UButton label="Cancel" color="neutral" variant="outline" @click="close" />
        <UButton
          :label="confirmLabel ?? 'Confirm'"
          :color="confirmColor ?? 'primary'"
          :loading="loading"
          @click="emit('confirm')"
        />
      </div>
    </template>
  </UModal>
</template>
