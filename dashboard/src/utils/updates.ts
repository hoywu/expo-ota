import type { UpdateListItem } from '@/types/admin';

function streamKey(item: Pick<UpdateListItem, 'runtimeVersion' | 'platform'>): string {
  return `${item.runtimeVersion}\0${item.platform}`;
}

/** Latest published update per (runtimeVersion, platform) stream by publishedAt. */
export function buildLatestPublishedIds(items: UpdateListItem[]): Set<string> {
  const latestByStream = new Map<string, { id: string; publishedAt: string }>();

  for (const item of items) {
    if (item.status !== 'published' || !item.publishedAt) continue;

    const key = streamKey(item);
    const existing = latestByStream.get(key);
    if (!existing || item.publishedAt > existing.publishedAt) {
      latestByStream.set(key, { id: item.id, publishedAt: item.publishedAt });
    }
  }

  return new Set([...latestByStream.values()].map((entry) => entry.id));
}
