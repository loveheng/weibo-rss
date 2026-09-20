/**
 * 并发请求去重（Request Collapsing）
 *
 * 同一 key 的并发请求只会触发一次底层请求，
 * 其余请求复用同一个 Promise 的结果。
 */
export class RequestCollapsing {
  private pending = new Map<string, Promise<any>>();

  async run<T>(key: string, producer: () => Promise<T>): Promise<T> {
    const existing = this.pending.get(key);
    if (existing) {
      return existing as Promise<T>;
    }

    const promise = producer().finally(() => {
      this.pending.delete(key);
    });

    this.pending.set(key, promise);
    return promise;
  }
}
