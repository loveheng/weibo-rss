import { Throttler } from "../../throttler";
import Axios from "axios";
import {
  buildAxiosConfig,
  getTwitterHeaders,
  handleTwitterErr,
  MOCK_UA,
  requestWithRetry,
  TIME_OUT,
  waitMs,
} from "./common";
import { TwitterStatus, TwitterUserData } from "../../../types";
import { logger } from "../../logger";

export class UserNotFoundError extends Error {
  constructor(username: string) {
    super(`username: ${username}`);
  }
}

export const createIndexAPI = () => {
  const runner = new Throttler("twitter");
  const axiosInstance = Axios.create({
    ...buildAxiosConfig(),
  });

  return {
    getUserInfo: (username: string) =>
      runner.runFunc<TwitterUserData>(async (disable) => {
        logger.debug(`[twitter/getInfo] ${username}`);
        await waitMs(Math.floor(Math.random() * 100));
        try {
          const res = await requestWithRetry(() =>
            axiosInstance({
              method: "get",
              url: `https://api.twitter.com/2/users/by/username/${username}`,
              headers: {
                ...getTwitterHeaders(),
              },
              params: {
                "user.fields": "description,profile_image_url,public_metrics",
              },
            }),
          );
          const data = res.data;
          if (!data.data) {
            return Promise.reject(new UserNotFoundError(username));
          }
          const user = data.data;
          return {
            id: user.id,
            username: user.username,
            name: user.name,
            description: user.description || "",
            profileImageUrl: user.profile_image_url,
            publicMetrics: user.public_metrics,
          };
        } catch (err) {
          const axiosErr = err as any;
          return handleTwitterErr(axiosErr, disable);
        }
      }),

    getUserTweets: (userId: string) =>
      runner.runFunc<TwitterStatus[]>(async (disable) => {
        logger.debug(`[twitter/getTweets] ${userId}`);
        await waitMs(Math.floor(Math.random() * 100));
        try {
          const res = await requestWithRetry(() =>
            axiosInstance({
              method: "get",
              url: `https://api.twitter.com/2/users/${userId}/tweets`,
              headers: {
                ...getTwitterHeaders(),
              },
              params: {
                "tweet.fields": "created_at,public_metrics,entities",
                "exclude": "retweets,replies",
                max_results: 20,
              },
            }),
          );
          const data = res.data;
          return (data.data || []).map((tweet: any) => ({
            id: tweet.id,
            text: tweet.text,
            created_at: tweet.created_at,
            publicMetrics: tweet.public_metrics,
            entities: tweet.entities,
          }));
        } catch (err) {
          const axiosErr = err as any;
          return handleTwitterErr(axiosErr, disable);
        }
      }),
  };
};

export type GetTwitterUserInfoFunc = ReturnType<typeof createIndexAPI>["getUserInfo"];

export type GetTwitterUserTweetsFunc = ReturnType<typeof createIndexAPI>["getUserTweets"];
