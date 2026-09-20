import { LRUCache } from "lru-cache";
import { CacheInterface, LoggerInterface } from "../types";
import { logger } from "./logger";

// 缓存条目的结构（保持兼容）
export interface CacheObject {
  created: number;
  expire: boolean | number;
  value: any;
};

/**
 * 基于 LRU 算法的纯内存缓存实现
 */
export class MemoryCache implements CacheInterface {
  private cache: LRUCache<string, any>;
  private logger: LoggerInterface;

  constructor(options: { max?: number; ttl?: number } = {}, log: LoggerInterface = logger) {
    this.logger = log;
    this.cache = new LRUCache({
      max: options.max || 1000,
      ttl: options.ttl || 1000 * 60 * 20, // 默认 20 分钟
      updateAgeOnGet: true,
      updateAgeOnHas: false,
    });
  }

  /**
   * 设置缓存
   */
  set(key: string, value: any, expire: number = 0) {
    this.logger.debug(`[cache] set ${key}`);
    const options: { ttl?: number } = {};
    if (expire) {
      options.ttl = expire * 1000; // 秒转毫秒
    }
    this.cache.set(key, value, options);
    return Promise.resolve();
  }

  /**
   * 获取缓存的值
   */
  get(key: string) {
    this.logger.debug(`[cache] get ${key}`);
    return Promise.resolve(this.cache.get(key) ?? null);
  }

  /**
   * memo in cache
   */
  async memo<T>(cb: () => T, key: string, expire = 0): Promise<Awaited<T>> {
    const cacheResp = await this.get(key);
    if (cacheResp !== null) {
      return cacheResp as Awaited<T>;
    }
    const res = await cb();
    this.set(key, res, expire);
    return res;
  }

  /**
   * 获取缓存统计信息（用于监控）
   */
  getStats() {
    return {
      size: this.cache.size,
      max: this.cache.max,
      calculatedSize: this.cache.calculatedSize,
    };
  }
}

/**
 * 工厂方法，保持与原有 LevelCache 类似的调用方式
 */
export const createMemoryCache = (options?: { max?: number; ttl?: number }, log?: LoggerInterface) =>
  new MemoryCache(options, log);
