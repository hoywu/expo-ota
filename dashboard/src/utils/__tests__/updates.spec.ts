import { describe, expect, it } from 'vitest';
import { buildLatestPublishedIds } from '@/utils/updates';
import type { UpdateListItem } from '@/types/admin';

function item(
  overrides: Partial<UpdateListItem> & Pick<UpdateListItem, 'id' | 'runtimeVersion' | 'platform'>
): UpdateListItem {
  return {
    manifestUuid: 'uuid',
    status: 'published',
    createdAt: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

describe('buildLatestPublishedIds', () => {
  it('returns the newest published update per stream', () => {
    const items = [
      item({
        id: 'a',
        runtimeVersion: '1.0.0',
        platform: 'ios',
        publishedAt: '2026-01-02T00:00:00Z',
      }),
      item({
        id: 'b',
        runtimeVersion: '1.0.0',
        platform: 'ios',
        publishedAt: '2026-01-03T00:00:00Z',
      }),
      item({
        id: 'c',
        runtimeVersion: '1.0.0',
        platform: 'android',
        publishedAt: '2026-01-01T00:00:00Z',
      }),
    ];

    expect(buildLatestPublishedIds(items)).toEqual(new Set(['b', 'c']));
  });

  it('ignores pending updates and published rows without publishedAt', () => {
    const items = [
      item({
        id: 'a',
        runtimeVersion: '1.0.0',
        platform: 'ios',
        status: 'pending',
        publishedAt: undefined,
      }),
      item({
        id: 'b',
        runtimeVersion: '1.0.0',
        platform: 'ios',
        publishedAt: '2026-01-03T00:00:00Z',
      }),
      item({
        id: 'c',
        runtimeVersion: '1.0.0',
        platform: 'ios',
        status: 'published',
        publishedAt: undefined,
      }),
    ];

    expect(buildLatestPublishedIds(items)).toEqual(new Set(['b']));
  });
});
