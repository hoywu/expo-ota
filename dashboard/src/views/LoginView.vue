<script setup lang="ts">
import { ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { useAuthStore } from '@/stores/auth';
import { ApiError } from '@/api/client';

const route = useRoute();
const router = useRouter();
const auth = useAuthStore();

const username = ref('');
const password = ref('');
const loading = ref(false);
const errorMessage = ref<string | null>(null);

async function submit(): Promise<void> {
  if (!username.value.trim() || !password.value) {
    errorMessage.value = 'Username and password are required';
    return;
  }

  loading.value = true;
  errorMessage.value = null;
  try {
    await auth.login(username.value, password.value);
    const redirect = (route.query.redirect as string) || '/apps';
    await router.replace(redirect);
  } catch (e) {
    if (e instanceof ApiError) {
      if (e.status === 429) {
        errorMessage.value = 'Too many attempts, try again later';
      } else {
        errorMessage.value = 'Invalid username or password';
      }
    } else {
      errorMessage.value = 'Something went wrong';
    }
  } finally {
    loading.value = false;
  }
}
</script>

<template>
  <UCard>
    <template #header>
      <div class="text-center">
        <UIcon name="i-lucide-radio-tower" class="size-8 text-primary mx-auto mb-2" />
        <h1 class="text-xl font-semibold text-default">Expo OTA Admin</h1>
        <p class="text-sm text-muted mt-1">Sign in to manage apps and updates</p>
      </div>
    </template>

    <form class="space-y-4" @submit.prevent="submit">
      <UAlert
        v-if="errorMessage"
        color="error"
        variant="subtle"
        :title="errorMessage"
        icon="i-lucide-alert-circle"
      />

      <UFormField label="Username" required>
        <UInput
          v-model="username"
          autocomplete="username"
          placeholder="admin"
          @keydown.enter="submit"
        />
      </UFormField>

      <UFormField label="Password" required>
        <UInput
          v-model="password"
          type="password"
          autocomplete="current-password"
          @keydown.enter="submit"
        />
      </UFormField>

      <UButton type="submit" label="Sign in" block :loading="loading" />
    </form>
  </UCard>
</template>
