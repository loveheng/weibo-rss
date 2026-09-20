import { Throttler } from "../../throttler";
import Axios from "axios";
import { Agent } from "https";
import { buildAxiosConfig, getCommonHeaders, handleForbiddenErr, MOCK_UA, requestWithRetry, TIME_OUT, waitMs } from "./common";
import { WeiboStatus, WeiboUserData } from "../../../types";
import { logger } from "../../logger";

export class UserNotFoundError extends Error {
  constructor(uid: string) {
    super(`uid: ${uid}`);
  }
}

export const createIndexAPI = () => {
  const runner = new Throttler("index");
  const axiosInstance = Axios.create({
    ...buildAxiosConfig(),
  });

  const refreshVisitorCookie = async () => {
    try {
      const res = await axiosInstance({
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
      const sub = cookies?.filter(cookie => cookie.startsWith("SUB")).map(cookie => cookie.split(";")[0]).join(";");
      if (sub) {
        axiosInstance.defaults.headers.common["Cookie"] = sub;
        logger.debug("[visitor] cookie refreshed");
      }
    } catch (err) {
      logger.warn("[visitor] refresh cookie failed", err);
    }
  };

  // 启动时先获取一次访客 Cookie
  refreshVisitorCookie();

  return {
    getIndexUserInfo: (uid: string) =>
      runner.runFunc<WeiboUserData>(async (disable) => {
        logger.debug(`[getInfo] ${uid}`);
        await waitMs(Math.floor(Math.random() * 100));
        try {
          const res = await requestWithRetry(() =>
            axiosInstance({
              method: "get",
              url: `https://m.weibo.cn/api/container/getIndex?type=uid&value=${uid}`,
              headers: {
                ...getCommonHeaders(),
                "Mweibo-Pwa": "1",
                "Referer": `https://m.weibo.cn/u/${uid}`,
                "X-Requested-With": "XMLHttpRequest",
              },
            }),
          );
          const data = res.data;
          if (data.ok !== 1) {
            return Promise.reject(new UserNotFoundError(uid));
          }
          return {
            uid,
            screenName: data.data.userInfo.screen_name,
            description: data.data.userInfo.description,
            containerId: data.data.tabsInfo.tabs[1].containerid,
          };
        } catch (err) {
          const axiosErr = err as any;
          if (axiosErr?.response?.status && [418, 403].includes(axiosErr.response.status)) {
            await refreshVisitorCookie();
          }
          return handleForbiddenErr(axiosErr, disable);
        }
      }),

    getWeiboContentList: (uid: string, containerId: string) =>
      runner.runFunc<WeiboStatus[]>(async (disable) => {
        logger.debug(`[getContList] ${uid} ${containerId}`);
        await waitMs(Math.floor(Math.random() * 100));
        try {
          const res = await requestWithRetry(() =>
            axiosInstance({
              method: "get",
              url: `https://m.weibo.cn/api/container/getIndex?type=uid&value=${uid}&containerid=${containerId}`,
              headers: {
                ...getCommonHeaders(),
                "Mweibo-Pwa": "1",
                "Referer": `https://m.weibo.cn/u/${uid}`,
                "X-Requested-With": "XMLHttpRequest",
              },
            }),
          );
          const data = res.data;
          return data.data.cards
            .filter((item: any) => item.mblog)
            .map((item: any) => item.mblog);
        } catch (err) {
          const axiosErr = err as any;
          if (axiosErr?.response?.status && [418, 403].includes(axiosErr.response.status)) {
            await refreshVisitorCookie();
          }
          return handleForbiddenErr(axiosErr, disable);
        }
      }),
  };
};

export type GetIndexUserInfoFunc = ReturnType<typeof createIndexAPI>["getIndexUserInfo"];

export type GetWeiboContentListFunc = ReturnType<typeof createIndexAPI>["getWeiboContentList"];
