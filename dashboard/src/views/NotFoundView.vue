<script setup lang="ts">
import { useRouter } from 'vue-router';

const router = useRouter();
</script>

<template>
  <div class="flex flex-col items-center justify-center min-h-[50vh] text-center p-6">
    <p class="text-6xl font-bold text-muted">404</p>
    <h1 class="text-xl font-semibold text-default mt-4">Page not found</h1>
    <p class="text-muted mt-2">The page you are looking for does not exist.</p>
    <UButton class="mt-6" label="Go to Apps" @click="router.push('/apps')" />
  </div>
</template>
