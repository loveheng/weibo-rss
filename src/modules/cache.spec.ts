import { describe, expect, test } from '@jest/globals';
import { MemoryCache, createMemoryCache } from './cache';

const mockFn = (...args: any[]) => ({} as any);

describe('MemoryCache', () => {
  test('set and get value', async () => {
    const cache = createMemoryCache();
    await cache.set('key', 'value');
    expect(await cache.get('key')).toBe('value');
  });

  test('get returns null for missing key', async () => {
    const cache = createMemoryCache();
    expect(await cache.get('missing')).toBeNull();
  });

  test('memo caches result', async () => {
    const cache = createMemoryCache();
    let calls = 0;
    const fn = () => {
      calls += 1;
      return 'result';
    };
    await expect(cache.memo(fn, 'memo-key', 60)).resolves.toBe('result');
    await expect(cache.memo(fn, 'memo-key', 60)).resolves.toBe('result');
    expect(calls).toBe(1);
  });

  test('getStats returns cache info', async () => {
    const cache = createMemoryCache({ max: 10, ttl: 1000 });
    await cache.set('a', 1);
    const stats = cache.getStats();
    expect(stats).toBeDefined();
    expect((stats as any).max).toBe(10);
  });
});
