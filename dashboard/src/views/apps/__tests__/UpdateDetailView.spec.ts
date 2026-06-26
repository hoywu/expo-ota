import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import type { UpdateDetailResp, UpdateStatsResp } from '@/types/admin';

vi.mock('@nuxt/ui/composables/useToast', () => ({
  useToast: () => ({ add: vi.fn<() => void>() }),
}));

vi.mock('@number-flow/vue', () => ({
  default: {
    name: 'NumberFlow',
    props: ['value'],
    template: '<span data-testid="number-flow">{{ value }}</span>',
  },
}));

const routeParamsHolder = vi.hoisted(() => ({
  current: null as { appSlug: string; updateId: string } | null,
}));

const { getUpdate, getUpdateStats, publishUpdate } = vi.hoisted(() => ({
  getUpdate: vi.fn<(...args: unknown[]) => Promise<UpdateDetailResp>>(),
  getUpdateStats: vi.fn<(...args: unknown[]) => Promise<UpdateStatsResp>>(),
  publishUpdate: vi.fn<() => Promise<{ updateId: string }>>(),
}));
vi.mock('@/api/updates', () => ({ getUpdate, getUpdateStats, publishUpdate }));

const push = vi.fn();
vi.mock('vue-router', async (importOriginal) => {
  const { reactive } = await import('vue');
  const actual = await importOriginal<typeof import('vue-router')>();
  routeParamsHolder.current = reactive({ appSlug: 'my-app', updateId: 'update-1' });
  return {
    ...actual,
    useRoute: () => ({
      params: routeParamsHolder.current!,
    }),
    useRouter: () => ({ push }),
  };
});

import UpdateDetailView from '@/views/apps/UpdateDetailView.vue';

const mockStats: UpdateStatsResp = {
  requestedDevices: 5,
  requestsWithoutDeviceId: 2,
  succeededDevices: 4,
  failedDevices: 1,
  durationMinMs: 100,
  durationMaxMs: 500,
  durationAvgMs: 250,
};

const publishedDetail: UpdateDetailResp = {
  id: 'update-1',
  appSlug: 'my-app',
  runtimeVersion: '1.0.0',
  platform: 'ios',
  manifestUuid: '11111111-2222-3333-4444-555555555555',
  status: 'published',
  createdAt: '2026-05-01T00:00:00Z',
  publishedAt: '2026-05-01T01:00:00Z',
  launchAssetKey: 'bundle',
  launchAssetUrl: 'https://example.com/bundle',
  assets: [],
  manifestPreview: {},
  stats: mockStats,
};

const pendingDetail: UpdateDetailResp = {
  ...publishedDetail,
  status: 'pending',
  publishedAt: undefined,
  stats: {
    requestedDevices: 0,
    requestsWithoutDeviceId: 0,
    succeededDevices: 0,
    failedDevices: 0,
  },
};

const stubs = {
  UCard: { template: '<div><slot /></div>' },
  UBadge: { template: '<span><slot /></span>' },
  UButton: {
    props: ['label', 'loading'],
    template: '<button :disabled="loading" @click="$emit(\'click\')">{{ label }}</button>',
  },
  USkeleton: { template: '<div aria-label="loading" />' },
  UAccordion: { template: '<div><slot name="assets" /></div>' },
  UTable: { template: '<div />' },
  CopyButton: { template: '<span />' },
  JsonPreview: { template: '<div />' },
  EmptyState: { template: '<div class="empty-state" />' },
  TimeAgo: { template: '<span />' },
  ConfirmModal: { template: '<div />' },
};

describe('UpdateDetailView', () => {
  const routeParams = () => routeParamsHolder.current!;
  let wrapper: ReturnType<typeof mount> | null = null;

  beforeEach(() => {
    vi.useFakeTimers();
    routeParams().appSlug = 'my-app';
    routeParams().updateId = 'update-1';
    getUpdate.mockReset();
    getUpdateStats.mockReset();
    publishUpdate.mockReset();
    push.mockReset();
  });

  afterEach(() => {
    wrapper?.unmount();
    wrapper = null;
    vi.useRealTimers();
  });

  it('polls stats every 5 seconds for published updates', async () => {
    getUpdate.mockResolvedValue(publishedDetail);
    getUpdateStats.mockResolvedValue({
      ...mockStats,
      requestedDevices: 6,
    });

    wrapper = mount(UpdateDetailView, { global: { stubs } });
    await flushPromises();

    expect(getUpdate).toHaveBeenCalledTimes(1);
    expect(getUpdateStats).not.toHaveBeenCalled();

    vi.advanceTimersByTime(5000);
    await flushPromises();
    expect(getUpdateStats).toHaveBeenCalledTimes(1);

    vi.advanceTimersByTime(5000);
    await flushPromises();
    expect(getUpdateStats).toHaveBeenCalledTimes(2);
  });

  it('does not poll stats for pending updates', async () => {
    getUpdate.mockResolvedValue(pendingDetail);

    wrapper = mount(UpdateDetailView, { global: { stubs } });
    await flushPromises();

    vi.advanceTimersByTime(15000);
    await flushPromises();
    expect(getUpdateStats).not.toHaveBeenCalled();
  });

  it('stops polling on unmount', async () => {
    getUpdate.mockResolvedValue(publishedDetail);
    getUpdateStats.mockResolvedValue(mockStats);

    wrapper = mount(UpdateDetailView, { global: { stubs } });
    await flushPromises();

    wrapper.unmount();
    wrapper = null;

    vi.advanceTimersByTime(15000);
    await flushPromises();
    expect(getUpdateStats).not.toHaveBeenCalled();
  });

  it('keeps showing data when stats refresh fails', async () => {
    getUpdate.mockResolvedValue(publishedDetail);
    getUpdateStats.mockRejectedValue(new Error('network'));

    wrapper = mount(UpdateDetailView, { global: { stubs } });
    await flushPromises();

    vi.advanceTimersByTime(5000);
    await flushPromises();

    expect(push).not.toHaveBeenCalled();
    expect(wrapper.text()).toContain('stale');
    expect(wrapper.text()).toContain('5');
  });

  it('reloads when update id changes', async () => {
    getUpdate.mockResolvedValueOnce(publishedDetail).mockResolvedValueOnce({
      ...publishedDetail,
      id: 'update-2',
      stats: { ...mockStats, requestedDevices: 42 },
    });

    wrapper = mount(UpdateDetailView, { global: { stubs } });
    await flushPromises();
    expect(getUpdate).toHaveBeenCalledTimes(1);

    routeParams().updateId = 'update-2';
    await flushPromises();

    expect(getUpdate).toHaveBeenCalledTimes(2);
    expect(getUpdate).toHaveBeenLastCalledWith('my-app', 'update-2', expect.any(AbortSignal));
  });

  it('reloads when app slug changes', async () => {
    getUpdate.mockResolvedValueOnce(publishedDetail).mockResolvedValueOnce({
      ...publishedDetail,
      appSlug: 'other-app',
      stats: { ...mockStats, requestedDevices: 42 },
    });

    wrapper = mount(UpdateDetailView, { global: { stubs } });
    await flushPromises();

    routeParams().appSlug = 'other-app';
    await flushPromises();

    expect(getUpdate).toHaveBeenCalledTimes(2);
    expect(getUpdate).toHaveBeenLastCalledWith('other-app', 'update-1', expect.any(AbortSignal));
  });

  it('starts polling after publish', async () => {
    getUpdate.mockResolvedValueOnce(pendingDetail).mockResolvedValueOnce(publishedDetail);
    publishUpdate.mockResolvedValue({ updateId: 'update-1' });
    getUpdateStats.mockResolvedValue({ ...mockStats, requestedDevices: 7 });

    wrapper = mount(UpdateDetailView, { global: { stubs } });
    await flushPromises();
    expect(getUpdateStats).not.toHaveBeenCalled();

    await wrapper.get('button', { text: 'Publish' }).trigger('click');
    await flushPromises();

    vi.advanceTimersByTime(5000);
    await flushPromises();
    expect(getUpdateStats).toHaveBeenCalledTimes(1);
  });

  it('updates displayed stats after a successful poll', async () => {
    getUpdate.mockResolvedValue(publishedDetail);
    getUpdateStats.mockResolvedValue({ ...mockStats, requestedDevices: 42 });

    wrapper = mount(UpdateDetailView, { global: { stubs } });
    await flushPromises();
    expect(wrapper.text()).toContain('5');

    vi.advanceTimersByTime(5000);
    await flushPromises();

    expect(wrapper.text()).toContain('42');
    expect(wrapper.text()).toContain('live');
  });

  it('recovers from stale to live after a failed then successful poll', async () => {
    getUpdate.mockResolvedValue(publishedDetail);
    getUpdateStats
      .mockRejectedValueOnce(new Error('network'))
      .mockResolvedValueOnce({ ...mockStats, requestedDevices: 8 });

    wrapper = mount(UpdateDetailView, { global: { stubs } });
    await flushPromises();

    vi.advanceTimersByTime(5000);
    await flushPromises();
    expect(wrapper.text()).toContain('stale');

    vi.advanceTimersByTime(5000);
    await flushPromises();
    expect(wrapper.text()).toContain('live');
    expect(wrapper.text()).not.toContain('stale');
    expect(wrapper.text()).toContain('8');
  });

  it('ignores stale async stats responses when a newer poll completes first', async () => {
    getUpdate.mockResolvedValue(publishedDetail);
    let resolveSlow: (value: UpdateStatsResp) => void = () => {};
    const slowPromise = new Promise<UpdateStatsResp>((resolve) => {
      resolveSlow = resolve;
    });
    getUpdateStats.mockReturnValueOnce(slowPromise).mockResolvedValueOnce({
      ...mockStats,
      requestedDevices: 8888,
    });

    wrapper = mount(UpdateDetailView, { global: { stubs } });
    await flushPromises();

    vi.advanceTimersByTime(5000);
    await flushPromises();

    vi.advanceTimersByTime(5000);
    await flushPromises();
    expect(wrapper.text()).toContain('8888');

    resolveSlow({ ...mockStats, requestedDevices: 7777 });
    await flushPromises();

    expect(wrapper.text()).toContain('8888');
    expect(wrapper.text()).not.toContain('7777');
  });
});
