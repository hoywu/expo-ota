<script setup lang="ts">
import { ref } from 'vue';
import { useRouter } from 'vue-router';
import { useAuthStore } from '@/stores/auth';
import { validatePassword } from '@/utils/validation';
import * as usersApi from '@/api/users';
import { handleApiError } from '@/utils/format';

defineProps<{
  changePasswordOpen?: boolean;
  collapsed?: boolean;
}>();

const emit = defineEmits<{
  'update:changePasswordOpen': [value: boolean];
}>();

const auth = useAuthStore();
const router = useRouter();
const toast = useToast();

const passwordOpen = ref(false);
const password = ref('');
const confirmPassword = ref('');
const passwordError = ref<string | null>(null);
const passwordLoading = ref(false);

function openChangePassword(): void {
  passwordOpen.value = true;
  emit('update:changePasswordOpen', true);
}

async function submitPassword(): Promise<void> {
  passwordError.value = validatePassword(password.value);
  if (passwordError.value) return;
  if (password.value !== confirmPassword.value) {
    passwordError.value = 'Passwords do not match';
    return;
  }
  if (!auth.user) return;

  passwordLoading.value = true;
  try {
    await usersApi.changePassword(auth.user.userId, { password: password.value });
    toast.add({ title: 'Password updated', color: 'success', duration: 3000 });
    password.value = '';
    confirmPassword.value = '';
    passwordOpen.value = false;
    emit('update:changePasswordOpen', false);
  } catch (e) {
    handleApiError(e, toast);
  } finally {
    passwordLoading.value = false;
  }
}

async function logout(): Promise<void> {
  await auth.logout();
  router.push('/login');
}

const menuItems = [
  [
    {
      label: 'Change password',
      icon: 'i-lucide-key-round',
      onSelect: openChangePassword,
    },
    {
      label: 'Sign out',
      icon: 'i-lucide-log-out',
      onSelect: logout,
    },
  ],
];
</script>

<template>
  <UDropdownMenu :items="menuItems">
    <UButton
      color="neutral"
      variant="ghost"
      block
      :icon="collapsed ? 'i-lucide-user' : undefined"
      :label="collapsed ? undefined : (auth.user?.username ?? 'Account')"
      :trailing-icon="collapsed ? undefined : 'i-lucide-chevron-down'"
    />
  </UDropdownMenu>

  <UModal
    :open="passwordOpen"
    title="Change password"
    @update:open="
      passwordOpen = $event;
      emit('update:changePasswordOpen', $event);
    "
  >
    <template #body>
      <div class="space-y-4">
        <UFormField label="New password" :error="passwordError ?? undefined">
          <UInput
            v-model="password"
            type="password"
            autocomplete="new-password"
            class="form-control"
          />
        </UFormField>
        <UFormField label="Confirm password">
          <UInput
            v-model="confirmPassword"
            type="password"
            autocomplete="new-password"
            class="form-control"
          />
        </UFormField>
      </div>
    </template>
    <template #footer>
      <div class="flex justify-end gap-2">
        <UButton label="Cancel" color="neutral" variant="outline" @click="passwordOpen = false" />
        <UButton label="Save" :loading="passwordLoading" @click="submitPassword" />
      </div>
    </template>
  </UModal>
</template>
