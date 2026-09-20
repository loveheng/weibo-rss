import { Throttler } from "../../throttler";
import Axios from "axios";
import { Agent } from "https";
import { buildAxiosConfig, getCommonHeaders, handleForbiddenErr, MOCK_UA, requestWithRetry, waitMs } from "./common";
import { logger } from "../../logger";

export class DomainNotFoundError extends Error {
  constructor(domain: string) {
    super(`domain: ${domain}`);
  }
}

export const createDomainAPI = () => {
  const runner = new Throttler("domain");
  const httpsAgent = new Agent({ keepAlive: true });
  const axiosInstance = Axios.create({
    ...buildAxiosConfig(),
    httpsAgent,
  });

  return {
    getUIDByDomain: (domain: string) =>
      runner.runFunc<string>(async (disable) => {
        logger.debug(`[domain] convert ${domain}`);
        await waitMs(Math.floor(Math.random() * 100));
        try {
          const res = await requestWithRetry(() =>
            axiosInstance({
              ...buildAxiosConfig(),
              method: "get",
              url: `https://m.weibo.cn/${domain}?&jumpfrom=weibocom`,
              headers: {
                ...getCommonHeaders(),
                "User-Agent": MOCK_UA,
              },
            }),
          );
          const uid = res.request.path.split("/u/")[1];
          if (!uid) {
            throw new DomainNotFoundError(domain);
          }
          return uid;
        } catch (err) {
          return handleForbiddenErr(err as any, disable);
        }
      }),
  };
};

export type GetUIDByDomainFunc = ReturnType<typeof createDomainAPI>["getUIDByDomain"];
