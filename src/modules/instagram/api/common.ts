import axios, { AxiosRequestConfig } from "axios";
import { Agent } from "https";
import config from "../../../config";
import { createRiskControl } from "../../antiCrawl";
import { applyProxy } from "../../proxy";

export const TIME_OUT = 3000 * 3;
export const MOCK_UA =
  "Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Mobile Safari/537.36";

// Instagram web api 的公共 app id
export const IG_APP_ID = "936619743392459";

export const BASE_HEADERS = {
  "User-Agent": MOCK_UA,
  "x-ig-app-id": IG_APP_ID,
  "Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
  Accept: "*/*",
};

export const getCommonHeaders = (): Record<string, string> => {
  const headers: Record<string, string> = { ...BASE_HEADERS };
  if (config.instagramCookie) {
    headers["Cookie"] = config.instagramCookie;
  }
  return headers;
};

// Instagram 源特有的风控钩子：401/403/429 视为风控
export const riskControl = createRiskControl({
  riskyStatuses: [401, 403, 429],
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

export const createInstagramInstance = () => {
  const httpsAgent = new Agent({ keepAlive: true });
  return axios.create(
    applyProxy({
      ...buildAxiosConfig(),
      httpsAgent,
    }, config.instagramProxy),
  );
};
