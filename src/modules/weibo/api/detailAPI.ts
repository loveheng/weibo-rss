import { Throttler } from "../../throttler";
import Axios from "axios";
import { Agent } from "https";
import { buildAxiosConfig, getCommonHeaders, handleForbiddenErr, MOCK_UA, requestWithRetry, waitMs } from "./common";
import { logger } from "../../logger";

export const createDetailAPI = () => {
  const runner = new Throttler("detail");
  const httpsAgent = new Agent({ keepAlive: true });
  const axiosInstance = Axios.create({
    ...buildAxiosConfig(),
    httpsAgent,
  });

  return {
    getWeiboDetail: (id: string) =>
      runner.runFunc(async (disable) => {
        logger.debug(`[getDetail] ${id}`);
        await waitMs(Math.floor(Math.random() * 100));
        try {
          const res = await requestWithRetry(() =>
            axiosInstance({
              ...buildAxiosConfig(),
              method: "get",
              url: `https://m.weibo.cn/statuses/show?id=${id}`,
              headers: {
                ...getCommonHeaders(),
                "Mweibo-Pwa": "1",
                "Referer": `https://m.weibo.cn/detail/${id}`,
                "X-Requested-With": "XMLHttpRequest",
              },
            }),
          );
          return res.data.data;
        } catch (err) {
          return handleForbiddenErr(err as any, disable);
        }
      }),
  };
};

export type GetWeiboDetailFunc = ReturnType<typeof createDetailAPI>["getWeiboDetail"];
