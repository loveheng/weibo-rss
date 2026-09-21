import config from "../../config";
import { CacheInterface, LoggerInterface, InstagramUserData } from "../../types";
import { logger } from "../logger";
import { memoWithPolicy, CachePolicy } from "../feedCache";
import { createInstagramAPI, GetInstagramUserInfoFunc, UserNotFoundError } from "./api/instagramAPI";

export {
  UserNotFoundError,
};

// Instagram 源的缓存策略：拉长 TTL 减少对上游的请求频次，优先保护出口 IP
const instagramCachePolicy: CachePolicy = {
  keyPrefix: "instagram-",
  infoTTL: 1 * 60 * 60,
};

export class InstagramData {
  private cache: CacheInterface;
  private logger: LoggerInterface;
  private getInstagramUserInfo: GetInstagramUserInfoFunc;

  constructor(cache: CacheInterface, log: LoggerInterface = logger) {
    this.cache = cache;
    this.logger = log;
    const { getInstagramUserInfo } = createInstagramAPI();
    // bind to this
    Object.assign(this, {
      getInstagramUserInfo,
    });
  }

  /**
   * get user's latest posts
   */
  fetchUserLatestPosts = async (username: string) => {
    const userInfo = await memoWithPolicy(
      this.cache,
      instagramCachePolicy,
      "info",
      username,
      () => this.getInstagramUserInfo(username),
    );

    return userInfo as InstagramUserData;
  };
}

const escapeHTML = (text: string) =>
  text.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");

const wrapImageCache = (url: string) => {
  return config.imageCache ? config.imageCache + encodeURIComponent(url) : url;
};

/**
 * convert instagram media to HTML for RSS feed
 */
export const mediaToHTML = (media: any) => {
  const summary = (media.edge_media_to_caption?.edges?.[0]?.node?.text || "").trim();
  let tempHTML = escapeHTML(summary).replace(/\n/g, "<br>");

  const renderVideo = (node: any) => {
    const poster = wrapImageCache(node.display_url);
    tempHTML += `<br><video controls preload="metadata" poster="${poster}"><source src="${node.video_url}" type="video/mp4" /></video>`;
  };

  const renderImage = (node: any) => {
    const url = wrapImageCache(node.display_url);
    tempHTML += `<br><a href="${url}" target="_blank"><img src="${url}"></a>`;
  };

  // 轮播图
  if (media.edge_sidecar_to_children?.edges?.length) {
    for (const edge of media.edge_sidecar_to_children.edges) {
      const node = edge.node;
      if (!node) continue;
      if (node.is_video && node.video_url) {
        renderVideo(node);
      } else if (node.display_url) {
        renderImage(node);
      }
    }
  } else if (media.is_video && media.video_url) {
    renderVideo(media);
  } else if (media.display_url) {
    renderImage(media);
  }

  return tempHTML;
};
