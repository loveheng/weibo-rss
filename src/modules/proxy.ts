/**
 * 公共代理模块
 *
 * 为各 RSS 数据源提供统一的出站代理配置：
 * - config 中 <src>Proxy 为标准 URL 形式，如 http://user:pass@host:port
 * - 解析为 axios 的 proxy 配置并应用到 axios 实例
 */
import { AxiosRequestConfig } from "axios";

/**
 * 解析代理 URL 为 axios proxy 配置；格式非法时返回 undefined
 */
export const parseProxy = (proxyUrl?: string): AxiosRequestConfig["proxy"] | undefined => {
  if (!proxyUrl) {
    return undefined;
  }
  try {
    const url = new URL(proxyUrl);
    return {
      host: url.hostname,
      port: parseInt(url.port) || 2080,
      protocol: url.protocol.replace(":", ""),
    };
  } catch (err) {
    // ignore malformed proxy config
    return undefined;
  }
};

/**
 * 将 config 中的代理配置应用到 axios 实例配置
 */
export const applyProxy = (axiosConfig: AxiosRequestConfig, proxyUrl?: string): AxiosRequestConfig => {
  const proxy = parseProxy(proxyUrl);
  if (proxy) {
    axiosConfig.proxy = proxy;
  }
  return axiosConfig;
};
