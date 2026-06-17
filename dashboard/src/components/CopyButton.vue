<script setup lang="ts">
import { ref } from 'vue';

const props = defineProps<{
  value: string;
  label?: string;
}>();

const copied = ref(false);

async function copy(): Promise<void> {
  await navigator.clipboard.writeText(props.value);
  copied.value = true;
  setTimeout(() => {
    copied.value = false;
  }, 2000);
}
</script>

<template>
  <UButton
    :icon="copied ? 'i-lucide-check' : 'i-lucide-copy'"
    :aria-label="label ?? 'Copy to clipboard'"
    color="neutral"
    variant="ghost"
    size="xs"
    @click="copy"
  />
</template>
