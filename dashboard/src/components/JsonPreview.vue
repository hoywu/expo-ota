<script setup lang="ts">
import { computed } from 'vue';

const props = defineProps<{
  data: Record<string, unknown> | unknown;
}>();

const formatted = computed(() => JSON.stringify(props.data, null, 2));
</script>

<template>
  <UAccordion
    :items="[{ label: 'Manifest preview', icon: 'i-lucide-braces', slot: 'manifest' }]"
    :default-value="[]"
  >
    <template #manifest>
      <pre class="text-xs font-mono overflow-x-auto p-3 bg-elevated rounded-md text-default">{{
        formatted
      }}</pre>
    </template>
  </UAccordion>
</template>
