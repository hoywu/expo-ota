<script setup lang="ts">
import { formatDateTime, formatRelativeTime } from '@/utils/format';

defineProps<{
  iso: string;
}>();
</script>

<template>
  <UTooltip :text="formatDateTime(iso)">
    <span class="text-muted">{{ formatRelativeTime(iso) }}</span>
  </UTooltip>
</template>
