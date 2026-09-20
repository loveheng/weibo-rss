import { describe, expect, test } from '@jest/globals';
import { RequestCollapsing } from './requestCollapsing';

describe('RequestCollapsing', () => {
  test('concurrent same key shares result', async () => {
    const collapsing = new RequestCollapsing();
    let calls = 0;
    const producer = async () => {
      calls += 1;
      return 'shared';
    };

    const results = await Promise.all([
      collapsing.run('key', producer),
      collapsing.run('key', producer),
      collapsing.run('key', producer),
    ]);

    expect(results).toEqual(['shared', 'shared', 'shared']);
    expect(calls).toBe(1);
  });

  test('different keys run independently', async () => {
    const collapsing = new RequestCollapsing();
    const results = await Promise.all([
      collapsing.run('a', async () => 'a'),
      collapsing.run('b', async () => 'b'),
    ]);
    expect(results).toEqual(['a', 'b']);
  });

  test('error is propagated to all waiters', async () => {
    const collapsing = new RequestCollapsing();
    const producer = async () => {
      throw new Error('boom');
    };

    await expect(collapsing.run('err', producer)).rejects.toThrow('boom');
    await expect(collapsing.run('err', producer)).rejects.toThrow('boom');
  });
});
