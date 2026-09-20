import { Throttler } from "../../throttler";
import Axios from "axios";
import { Agent } from "https";
import { buildAxiosConfig, getCommonHeaders, handleForbiddenErr, MOCK_UA, requestWithRetry, waitMs } from "./common";
import { logger } from "../../logger";

export const createLongTextAPI = () => {
  const runner = new Throttler("longText");
  const httpsAgent = new Agent({ keepAlive: true });
  const axiosInstance = Axios.create({
    ...buildAxiosConfig(),
    httpsAgent,
  });

  return {
    getWeiboLongText: (id: string) =>
      runner.runFunc<string>(async (disable) => {
        logger.debug(`[longText] ${id}`);
        await waitMs(Math.floor(Math.random() * 100));
        try {
          const res = await requestWithRetry(() =>
            axiosInstance({
              ...buildAxiosConfig(),
              method: "get",
              url: `https://m.weibo.cn/statuses/extend?id=${id}`,
              headers: {
                ...getCommonHeaders(),
                "Mweibo-Pwa": "1",
                "Referer": `https://m.weibo.cn/detail/${id}`,
                "X-Requested-With": "XMLHttpRequest",
              },
            }),
          );
          const data = res.data;
          if (!data.data) {
            throw new Error(JSON.stringify(data));
          }
          return data.data.longTextContent;
        } catch (err) {
          return handleForbiddenErr(err as any, disable);
        }
      }),
  };
};

export type GetWeiboLongTextFunc = ReturnType<typeof createLongTextAPI>["getWeiboLongText"];
