import axios, { AxiosRequestConfig } from "axios";
import { Agent } from "https";
import config from "../../../config";
import { createRiskControl } from "../../antiCrawl";
import { applyProxy } from "../../proxy";

export const TIME_OUT = 3000 * 3;
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

// 微博源特有的风控钩子：仅 403/418 视为风控
export const riskControl = createRiskControl({
  riskyStatuses: [403, 418],
});

// 绑定源特有配置的防风控方法，保持原有 import 路径兼容
export const requestWithRetry = riskControl.requestWithRetry;
export const handleForbiddenErr = riskControl.handleForbiddenErr;
export { waitMs } from "../../antiCrawl";

export const buildAxiosConfig = (overrides?: AxiosRequestConfig): AxiosRequestConfig => {
  return {
    timeout: TIME_OUT,
    headers: getCommonHeaders(),
    ...overrides,
  } as AxiosRequestConfig;
};

export const createWeiboInstance = () => {
  const httpsAgent = new Agent({ keepAlive: true });
  const instance = axios.create(
    applyProxy({
      ...buildAxiosConfig(),
      httpsAgent,
    }, config.weiboProxy),
  );
  startVisitorCookieRotation(instance);
  return instance;
};
