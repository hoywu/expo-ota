<script setup lang="ts">
import { computed } from 'vue';
import NumberFlow from '@number-flow/vue';

const props = withDefaults(
  defineProps<{
    value: number;
    label: string;
    suffix?: string;
  }>(),
  { suffix: '' }
);

const displayValue = computed(() => props.value ?? 0);
</script>

<template>
  <UCard variant="subtle">
    <p class="text-sm text-muted mb-1">{{ label }}</p>
    <p class="text-2xl font-semibold text-default tabular-nums">
      <NumberFlow :value="displayValue" />
      <span v-if="suffix" class="text-base font-normal text-muted ml-1">{{ suffix }}</span>
    </p>
  </UCard>
</template>
