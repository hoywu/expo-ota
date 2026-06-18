import { describe, expect, it, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import StatCard from '@/components/StatCard.vue';

vi.mock('@number-flow/vue', () => ({
  default: {
    name: 'NumberFlow',
    props: ['value'],
    template: '<span data-testid="number-flow">{{ value }}</span>',
  },
}));

describe('StatCard', () => {
  it('renders NumberFlow with value', () => {
    const wrapper = mount(StatCard, {
      props: { value: 42, label: 'Devices' },
      global: {
        stubs: {
          UCard: { template: '<div><slot /></div>' },
        },
      },
    });

    expect(wrapper.text()).toContain('Devices');
    expect(wrapper.find('[data-testid="number-flow"]').text()).toBe('42');
  });

  it('updates when value changes', async () => {
    const wrapper = mount(StatCard, {
      props: { value: 1, label: 'Count' },
      global: {
        stubs: {
          UCard: { template: '<div><slot /></div>' },
        },
      },
    });

    await wrapper.setProps({ value: 99 });
    expect(wrapper.find('[data-testid="number-flow"]').text()).toBe('99');
  });

  it('renders optional note below the main value', () => {
    const wrapper = mount(StatCard, {
      props: {
        value: 12,
        label: 'Requested devices',
        noteValue: 3,
        noteLabel: 'requests without device id',
      },
      global: {
        stubs: {
          UCard: { template: '<div><slot /></div>' },
        },
      },
    });

    expect(wrapper.text()).toContain('requests without device id');
    expect(wrapper.findAll('[data-testid="number-flow"]')).toHaveLength(2);
  });
});
