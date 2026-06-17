import { describe, expect, it } from 'vitest';
import { shallowMount } from '@vue/test-utils';
import ConfirmModal from '@/components/ConfirmModal.vue';

describe('ConfirmModal', () => {
  it('emits confirm and cancel', async () => {
    const wrapper = shallowMount(ConfirmModal, {
      props: {
        open: true,
        title: 'Delete?',
      },
    });

    await wrapper.vm.$emit('update:open', false);
    await wrapper.vm.$emit('cancel');

    expect(wrapper.emitted('update:open')?.[0]).toEqual([false]);
    expect(wrapper.emitted('cancel')).toBeTruthy();

    await wrapper.setProps({ open: true });
    await wrapper.vm.$emit('confirm');
    expect(wrapper.emitted('confirm')).toBeTruthy();
  });
});
