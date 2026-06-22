import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import type { SystemStatsResp } from '@/types/admin';

// useToast depends on Nuxt's useState which is unavailable in pure component tests.
vi.mock('@nuxt/ui/composables/useToast', () => ({
  useToast: () => ({ add: vi.fn<() => void>() }),
}));

// NumberFlow pulls in a lot of runtime; stub it via StatCard's dependency.
vi.mock('@number-flow/vue', () => ({
  default: {
    name: 'NumberFlow',
    props: ['value'],
    template: '<span data-testid="number-flow">{{ value }}</span>',
  },
}));

const { getSystemStats } = vi.hoisted(() => ({
  getSystemStats: vi.fn<() => Promise<SystemStatsResp>>(),
}));
vi.mock('@/api/observability', () => ({ getSystemStats }));

// Import after mocks are registered.
import ObservabilityView from '@/views/admin/ObservabilityView.vue';

const mockStats: SystemStatsResp = {
  heapAllocBytes: 10 * 1024 * 1024,
  heapInUseBytes: 12 * 1024 * 1024,
  heapSysBytes: 20 * 1024 * 1024,
  stackInUseBytes: 1 * 1024 * 1024,
  numGC: 42,
  numGoroutine: 7,
  goVersion: 'go1.24.0',
  uptimeSeconds: 3600,
};

const stubs = {
  UCard: { template: '<div><slot /></div>' },
  UBadge: { template: '<span><slot /></span>' },
  UIcon: { template: '<span />' },
  EmptyState: { template: '<div class="empty-state" />' },
};

describe('ObservabilityView', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    getSystemStats.mockReset();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('shows skeleton while loading', async () => {
    getSystemStats.mockReturnValue(new Promise(() => {})); // never resolves

    const wrapper = mount(ObservabilityView, { global: { stubs } });
    await flushPromises();

    // USkeleton renders with aria-label="loading". 1 header + 6 cards = 7.
    expect(wrapper.findAll('[aria-label="loading"]')).toHaveLength(7);
  });

  it('renders stat cards after successful load', async () => {
    getSystemStats.mockResolvedValue(mockStats);

    const wrapper = mount(ObservabilityView, { global: { stubs } });
    await flushPromises();

    // 6 stat cards × 1 NumberFlow each = 6 number-flow nodes
    expect(wrapper.findAll('[data-testid="number-flow"]')).toHaveLength(6);
    // GoVersion rendered inline
    expect(wrapper.text()).toContain('go1.24.0');
    // HeapAlloc displayed as MB (10 MB)
    expect(wrapper.text()).toContain('10');
    expect(wrapper.text()).toContain('MB');
    // Goroutines count rendered
    expect(wrapper.text()).toContain('7');
  });

  it('shows empty state on fetch failure', async () => {
    getSystemStats.mockRejectedValue(new Error('network'));

    const wrapper = mount(ObservabilityView, { global: { stubs } });
    await flushPromises();

    expect(wrapper.find('.empty-state').exists()).toBe(true);
  });

  it('polls every 5 seconds', async () => {
    getSystemStats.mockResolvedValue(mockStats);

    mount(ObservabilityView, { global: { stubs } });
    await flushPromises();

    // Initial load
    expect(getSystemStats).toHaveBeenCalledTimes(1);

    vi.advanceTimersByTime(5000);
    await flushPromises();
    expect(getSystemStats).toHaveBeenCalledTimes(2);

    vi.advanceTimersByTime(5000);
    await flushPromises();
    expect(getSystemStats).toHaveBeenCalledTimes(3);
  });

  it('stops polling on unmount', async () => {
    getSystemStats.mockResolvedValue(mockStats);

    const wrapper = mount(ObservabilityView, { global: { stubs } });
    await flushPromises();
    expect(getSystemStats).toHaveBeenCalledTimes(1);

    wrapper.unmount();

    vi.advanceTimersByTime(15000);
    await flushPromises();
    // No additional calls after unmount
    expect(getSystemStats).toHaveBeenCalledTimes(1);
  });
});
