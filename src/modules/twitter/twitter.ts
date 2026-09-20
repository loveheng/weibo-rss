import config from "../../config";
import { CacheInterface, LoggerInterface, TwitterUserData, TwitterStatus } from "../../types";
import { logger } from "../logger";
import {
  createIndexAPI,
  GetTwitterUserInfoFunc,
  GetTwitterUserTweetsFunc,
  UserNotFoundError,
} from "./api/indexAPI";

export { UserNotFoundError };

export class TwitterData {
  private cache: CacheInterface;
  private logger: LoggerInterface;
  private getUserInfo: GetTwitterUserInfoFunc;
  private getUserTweets: GetTwitterUserTweetsFunc;

  constructor(cache: CacheInterface, log: LoggerInterface = logger) {
    this.cache = cache;
    this.logger = log;
    const { getUserInfo, getUserTweets } = createIndexAPI();
    Object.assign(this, {
      getUserInfo,
      getUserTweets,
    });
  }

  /**
   * get user's latest tweets
   */
  fetchUserLatestTweets = async (username: string) => {
    const userInfo = await this.cache.memo(
      () => this.getUserInfo(username),
      `twitter-info-${username}`,
      config.cacheTTL.apiTwitterUserInfo,
    );
    const tweets = await this.cache.memo(
      async () => this.getUserTweets(userInfo.id),
      `twitter-tweets-${userInfo.id}`,
      config.cacheTTL.apiTwitterTweets,
    );

    return {
      ...userInfo,
      tweets,
    } as TwitterUserData;
  };
}

/**
 * convert tweet text to HTML for RSS feed
 */
export const tweetToHTML = (tweet: TwitterStatus) => {
  let tempHTML = tweet.text || "";

  // convert URLs to links
  if (tweet.entities?.urls) {
    tweet.entities.urls.forEach((url: any) => {
      const link = `<a href="${url.expanded_url || url.url}" target="_blank">${url.display_url || url.url}</a>`;
      tempHTML = tempHTML.replace(url.url, link);
    });
  }

  // convert mentions to links
  if (tweet.entities?.mentions) {
    tweet.entities.mentions.forEach((mention: any) => {
      const link = `<a href="https://twitter.com/${mention.username}" target="_blank">@${mention.username}</a>`;
      tempHTML = tempHTML.replace(`@${mention.username}`, link);
    });
  }

  // convert hashtags to links
  if (tweet.entities?.hashtags) {
    tweet.entities.hashtags.forEach((hashtag: any) => {
      const link = `<a href="https://twitter.com/hashtag/${hashtag.tag}" target="_blank">#${hashtag.tag}</a>`;
      tempHTML = tempHTML.replace(`#${hashtag.tag}`, link);
    });
  }

  // preserve line breaks
  tempHTML = tempHTML.replace(/\n/g, "<br>");

  return tempHTML;
};
