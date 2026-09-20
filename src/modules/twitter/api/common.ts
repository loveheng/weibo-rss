import axios, { AxiosError, AxiosRequestConfig, AxiosResponse } from "axios";
import config from "../../../config";
import { logger } from "../../logger";

export const TIME_OUT = 3000 * 3;
export const MOCK_UA =
  "Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Mobile Safari/537.36";

export const BASE_HEADERS = {
  "User-Agent": MOCK_UA,
  Accept: "application/json",
};

export const getTwitterHeaders = (): Record<string, string> => {
  const headers: Record<string, string> = { ...BASE_HEADERS };
  const bearerToken = config.twitterBearerToken || process.env.TWITTER_BEARER_TOKEN;
  if (bearerToken) {
    headers["Authorization"] = `Bearer ${bearerToken}`;
  }
  return headers;
};

export const waitMs = (ms: number) => {
  return new Promise((resolve) => {
    setTimeout(resolve, ms);
  });
};

export const requestWithRetry = async <T>(
  requestor: () => Promise<AxiosResponse<T>>,
  retries = 2,
  baseDelay = 1000,
): Promise<AxiosResponse<T>> => {
  let attempt = 0;
  let lastError: AxiosError | Error | null = null;

  while (attempt <= retries) {
    try {
      return await requestor();
    } catch (err) {
      lastError = err as AxiosError | Error;
      const status = (err as AxiosError).response?.status;
      if (status && [403, 429].includes(status)) {
        break;
      }
      if (attempt < retries) {
        const delay = baseDelay * Math.pow(2, attempt) + Math.floor(Math.random() * 300);
        await waitMs(delay);
      }
      attempt += 1;
    }
  }

  return Promise.reject(lastError);
};

export const buildAxiosConfig = (overrides?: AxiosRequestConfig): AxiosRequestConfig => {
  return {
    timeout: TIME_OUT,
    headers: getTwitterHeaders(),
    ...overrides,
  } as AxiosRequestConfig;
};

export const createTwitterInstance = () => {
  const proxyUrl = config.twitterProxy || process.env.TWITTER_PROXY;
  
  const axiosConfig: AxiosRequestConfig = {
    ...buildAxiosConfig(),
    // 禁用 IPv6，强制使用 IPv4
    family: 4,
    // 创建自定义代理配置
    ...(proxyUrl ? {
      proxy: {
        host: proxyUrl.includes('://') 
          ? proxyUrl.split('://')[1].split(':')[0] 
          : proxyUrl.split(':')[0],
        port: proxyUrl.includes('://')
          ? parseInt(proxyUrl.split('://')[1].split(':')[1] || '2080')
          : parseInt(proxyUrl.split(':')[1] || '2080'),
        protocol: proxyUrl.includes('://') 
          ? proxyUrl.split('://')[0] 
          : 'http'
      }
    } : {})
  };
  
  const instance = axios.create(axiosConfig);
  return instance;
};

export const handleTwitterErr = (err: AxiosError, cb: () => Promise<void>) => {
  const status = err.response?.status;
  if (status && [403, 429].includes(status)) {
    return cb();
  } else {
    return Promise.reject(err);
  }
};
