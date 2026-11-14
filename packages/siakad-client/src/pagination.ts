import type { SiakadClient } from './index.ts';

export const PAGE_SIZE = 25;

export interface ResourcePage<T> {
  rows: T[];
  offset: number;
  hasMore: boolean;
}

export function emptyPage<T>(): ResourcePage<T> {
  return { rows: [], offset: 0, hasMore: false };
}

export function mergeResources<T>(
  current: T[],
  incoming: T[],
  identify: (row: T) => string
): T[] {
  const rows = new Map(current.map((row) => [identify(row), row]));
  for (const row of incoming) rows.set(identify(row), row);
  return [...rows.values()];
}

// Each click fetches one bounded page. Retained selections do not advance the API cursor.
export async function loadResourcePage<T>(
  client: Pick<SiakadClient, 'get'>,
  path: string,
  previous: ResourcePage<T>,
  identify: (row: T) => string,
  more = false,
  retained: T[] = []
): Promise<ResourcePage<T>> {
  if (more && !previous.hasMore) return previous;
  const offset = more ? previous.offset : 0;
  const fetched = await client.get<T[]>(
    `${path}${path.includes('?') ? '&' : '?'}limit=${PAGE_SIZE}&offset=${offset}`
  );
  const rows = more
    ? mergeResources(previous.rows, fetched, identify)
    : fetched;
  const ids = new Set(rows.map(identify));
  return {
    rows: mergeResources(
      rows,
      retained.filter((row) => !ids.has(identify(row))),
      identify
    ),
    offset: offset + fetched.length,
    hasMore: fetched.length === PAGE_SIZE
  };
}
