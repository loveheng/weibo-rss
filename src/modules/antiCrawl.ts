/**
 * 公共防风控（anti-crawl）模块
 *
 * 为各 RSS 数据源（weibo / instagram / ...）提供统一的：
 * - 请求重试（指数退避 + 随机抖动）
 * - 风控响应识别（401/403/418/429 等）
 * - 风控触发后的回调（Throttler 熔断、刷新 Cookie 等源特有逻辑，通过 hooks 注入）
 */
import { AxiosError, AxiosResponse } from "axios";

/** 默认识别为风控/限流的 HTTP 状态码 */
export const DEFAULT_RISKY_STATUS = [401, 403, 418, 429];

/**
 * 源特有风控钩子：
 * - riskyStatuses: 覆盖该源识别为风控的状态码（如 weibo 的 418、instagram 的 401）
 * - onRiskDetected: 命中风控状态码时触发（如刷新访客 Cookie），在熔断回调之前执行
 */
export interface RiskControlHooks {
  riskyStatuses?: number[];
  onRiskDetected?: (err: AxiosError) => Promise<void> | void;
}

const getRiskyStatuses = (hooks?: RiskControlHooks): number[] =>
  hooks?.riskyStatuses ?? DEFAULT_RISKY_STATUS;

const isRiskyStatus = (status: number | undefined, hooks?: RiskControlHooks) =>
  !!status && getRiskyStatuses(hooks).includes(status);

export const waitMs = (ms: number) => {
  return new Promise((resolve) => {
    setTimeout(resolve, ms);
  });
};

/**
 * 带重试的请求：指数退避 + 随机抖动；命中风控状态码时立即停止重试
 */
export const requestWithRetry = async <T>(
  requestor: () => Promise<AxiosResponse<T>>,
  hooks?: RiskControlHooks,
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
      if (isRiskyStatus(status, hooks)) {
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

/**
 * 风控错误统一处理：
 * 命中风控状态码时，先执行源特有钩子（如刷新 Cookie），再执行熔断回调（disable）
 */
export const handleForbiddenErr = (
  err: AxiosError,
  cb: () => Promise<void>,
  hooks?: RiskControlHooks,
) => {
  if (isRiskyStatus(err.response?.status, hooks)) {
    if (hooks?.onRiskDetected) {
      return Promise.resolve(hooks.onRiskDetected(err)).then(() => cb());
    }
    return cb();
  } else {
    return Promise.reject(err);
  }
};

/**
 * 创建一套绑定源特有 hooks 的快捷方法，免去每处调用都传 hooks
 */
export const createRiskControl = (hooks?: RiskControlHooks) => ({
  requestWithRetry: <T>(requestor: () => Promise<AxiosResponse<T>>, retries?: number, baseDelay?: number) =>
    requestWithRetry(requestor, hooks, retries, baseDelay),
  handleForbiddenErr: (err: AxiosError, cb: () => Promise<void>) =>
    handleForbiddenErr(err, cb, hooks),
});
