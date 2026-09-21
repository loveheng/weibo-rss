import { Throttler } from "../../throttler";
import { createInstagramInstance, getCommonHeaders, handleForbiddenErr, requestWithRetry, waitMs } from "./common";
import { InstagramMedia, InstagramUserData } from "../../../types";
import { logger } from "../../logger";

export class UserNotFoundError extends Error {
  constructor(username: string) {
    super(`username: ${username}`);
  }
}

export const createInstagramAPI = () => {
  // Instagram 风控严格，熔断后冷却 30 分钟再恢复，优先保护出口 IP
  const runner = new Throttler("instagram-web", logger, 30 * 60 * 1000);
  const axiosInstance = createInstagramInstance();

  return {
    getInstagramUserInfo: (username: string) =>
      runner.runFunc<InstagramUserData>(async (disable) => {
        logger.debug(`[instagram/getInfo] ${username}`);
        await waitMs(Math.floor(Math.random() * 100));
        try {
          const res = await requestWithRetry(() =>
            axiosInstance({
              method: "get",
              url: "https://www.instagram.com/api/v1/users/web_profile_info/",
              params: { username },
              headers: {
                ...getCommonHeaders(),
                "Referer": `https://www.instagram.com/${username}/`,
              },
            }),
          );
          const data = res.data;
          const user = data?.data?.user;
          if (!user) {
            return Promise.reject(new UserNotFoundError(username));
          }

          const edges = user.edge_owner_to_timeline_media?.edges || [];
          const media: InstagramMedia[] = edges.map((edge: any) => edge.node).filter(Boolean);

          return {
            id: String(user.id),
            username: user.username,
            name: user.full_name || user.username,
            description: user.biography || "",
            media,
          } as InstagramUserData;
        } catch (err) {
          const axiosErr = err as any;
          return handleForbiddenErr(axiosErr, disable);
        }
      }),
  };
};

export type GetInstagramUserInfoFunc = ReturnType<typeof createInstagramAPI>["getInstagramUserInfo"];
