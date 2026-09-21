/**
 * 公共缓存策略模块
 *
 * 为各 RSS 数据源提供统一的缓存三件套：
 * - cache.memo（按 key + TTL 缓存接口结果）
 * - requestCollapsing.run（相同请求合并，防击穿）
 * - key 前缀约定
 *
 * 各源通过 CachePolicy / FeedCachePolicy 接口自定义自己的缓存策略：
 * - key 前缀、各接口 TTL、XML TTL、是否启用请求合并等
 */
import { CacheInterface } from "../types";
import { RequestCollapsing } from "./requestCollapsing";

/**
 * 数据源级缓存策略（供新增 RSS 源自定义）
 */
export interface CachePolicy {
  /** 缓存 key 前缀，如 'instagram-info-' */
  keyPrefix: string;
  /** 用户信息类结果的 TTL（秒），0 或不传表示使用默认值 */
  infoTTL?: number;
  /** 内容列表类结果的 TTL（秒） */
  listTTL?: number;
}

/**
 * RSS XML 输出级缓存策略
 */
export interface FeedCachePolicy {
  /** XML 缓存 key 前缀，如 'instagram-xml-' */
  xmlKeyPrefix: string;
  /** XML 缓存 TTL（秒），默认取 config.cacheTTL.rssXml */
  xmlTTL?: number;
  /** 是否启用请求合并（默认启用） */
  collapse?: boolean;
}

const DEFAULT_INFO_TTL = 15 * 60;
const DEFAULT_LIST_TTL = 15 * 60;

/**
 * 带策略的 memo：按 policy 自动拼接 key、选择 TTL
 */
export const memoWithPolicy = async <T>(
  cache: CacheInterface,
  policy: CachePolicy,
  kind: "info" | "list",
  ident: string,
  cb: () => T,
): Promise<Awaited<T>> => {
  const ttl = kind === "info" ? (policy.infoTTL ?? DEFAULT_INFO_TTL) : (policy.listTTL ?? DEFAULT_LIST_TTL);
  return await cache.memo(cb, `${policy.keyPrefix}${kind === "info" ? "info-" : "list-"}${ident}`, ttl);
};

/**
 * RSS 路由通用的「请求合并 + XML 缓存」封装：
 * collapse 未命中且 memo 未命中时才真正执行 fetchFeed
 */
export const cachedFeed = async (
  cache: CacheInterface,
  requestCollapsing: RequestCollapsing,
  policy: FeedCachePolicy,
  ident: string,
  fetchFeed: () => Promise<string | undefined>,
): Promise<{ xmlData: string | undefined; cacheMiss: boolean }> => {
  let cacheMiss = false;

  const run = async () => {
    return await cache.memo(async () => {
      const xml = await fetchFeed();
      if (xml !== undefined) {
        cacheMiss = true;
      }
      return xml;
    }, `${policy.xmlKeyPrefix}${ident}`, policy.xmlTTL ?? 0);
  };

  const xmlData = policy.collapse === false
    ? await run()
    : await requestCollapsing.run(`${policy.xmlKeyPrefix}${ident}`, run);

  return { xmlData, cacheMiss };
};
