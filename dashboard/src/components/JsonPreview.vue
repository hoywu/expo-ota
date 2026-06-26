<script setup lang="ts">
import { computed } from 'vue';

const props = defineProps<{
  data: Record<string, unknown> | unknown;
  class?: string;
}>();

const formatted = computed(() => JSON.stringify(props.data, null, 2));
</script>

<template>
  <UAccordion
    :items="[{ label: 'Manifest preview', icon: 'i-lucide-braces', slot: 'manifest' }]"
    :default-value="[]"
    :class="props.class"
  >
    <template #manifest>
      <pre
        class="text-xs font-mono overflow-auto p-3 bg-elevated rounded-md text-default max-h-[min(24rem,60vh)]"
      >
        {{ formatted }}
      </pre>
    </template>
  </UAccordion>
</template>
