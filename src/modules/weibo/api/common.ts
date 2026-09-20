import axios, { AxiosError, AxiosRequestConfig, AxiosResponse } from "axios";
import { Agent } from "https";
import config from "../../../config";

export const TIME_OUT = 3000;
export const MOCK_UA =
  "Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Mobile Safari/537.36";

export const BASE_HEADERS = {
  "User-Agent": MOCK_UA,
  "Referer": "https://m.weibo.cn/",
  "Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
  Accept: "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
};

export const getCommonHeaders = (): Record<string, string> => {
  const headers: Record<string, string> = { ...BASE_HEADERS };
  if (config.weiboCookie) {
    headers["Cookie"] = config.weiboCookie;
  }
  return headers;
};

let visitorCookieTimer: NodeJS.Timeout | undefined;

export const refreshVisitorCookie = async (instance: ReturnType<typeof axios.create>) => {
  try {
    const res = await instance({
      method: "POST",
      url: "https://visitor.passport.weibo.cn/visitor/genvisitor2",
      headers: {
        "Content-Type": "application/x-www-form-urlencoded",
        "User-Agent": MOCK_UA,
      },
      data: {
        cb: "visitor_gray_callback",
        tid: "",
        from: "weibo",
      },
    });
    const cookies = res.headers["set-cookie"] as string[] | undefined;
    const sub = cookies?.filter((cookie) => cookie.startsWith("SUB")).map((cookie) => cookie.split(";")[0]).join(";");
    if (sub) {
      instance.defaults.headers.common["Cookie"] = sub;
    }
  } catch (err) {
    // 静默失败，依赖后续请求触发重试
  }
};

export const startVisitorCookieRotation = (instance: ReturnType<typeof axios.create>, intervalMs = 30 * 60 * 1000) => {
  refreshVisitorCookie(instance);
  if (visitorCookieTimer) {
    clearInterval(visitorCookieTimer);
  }
  visitorCookieTimer = setInterval(() => {
    refreshVisitorCookie(instance);
  }, intervalMs);
};

export const stopVisitorCookieRotation = () => {
  if (visitorCookieTimer) {
    clearInterval(visitorCookieTimer);
    visitorCookieTimer = undefined;
  }
};

export const handleForbiddenErr = (err: AxiosError, cb: () => Promise<void>) => {
  if (err.response && [418, 403].includes(err.response.status)) {
    return cb();
  } else {
    return Promise.reject(err);
  }
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
      if (status && [403, 418].includes(status)) {
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
    headers: getCommonHeaders(),
    ...overrides,
  } as AxiosRequestConfig;
};

export const createWeiboInstance = () => {
  const httpsAgent = new Agent({ keepAlive: true });
  const instance = axios.create({
    ...buildAxiosConfig(),
    httpsAgent,
  });
  startVisitorCookieRotation(instance);
  return instance;
};
