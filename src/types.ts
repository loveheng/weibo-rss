import { Tracer } from "tracer";
import { WeiboData } from "./modules/weibo/weibo";
import { InstagramData } from "./modules/instagram/instagram";
import { RequestCollapsing } from "./modules/requestCollapsing";

export interface RSSKoaContext {
  cache: CacheInterface;
  weibo: WeiboData;
  instagram: InstagramData;
  requestCollapsing: RequestCollapsing;
}

export interface RSSKoaState {
  // cache hit
  hit: 0 | 1;
}

export type LoggerInterface = Tracer.Logger<string>;

export interface CacheInterface {
  set: (key: string, value: any, expire: number) => Promise<void>;
  get: (key: string) => any;
  memo: <T>(cb: () => T, key: string, expire: number) => Promise<Awaited<T>>;
}

export interface WeiboStatus {
  // eg: '4851725594528288'
  id: string;
  // eg: '4851725594528288'
  mid: string;
  // eg: 'MlHCnj00U'
  bid: string;
  // time
  created_at: string;
  // text
  text: string;
  isLongText: boolean;
  user: {
    id: number,
    screen_name: string,
    profile_url: string;
    description: string;
    // more...
    [x: string]: any;
  },
  // pics
  pic_ids: string[];
  thumbnail_pic: string;
  bmiddle_pic: string;
  original_pic: string;
  pics: {
    pid: string,
    url: string,
    size: 'orj360' | 'large',
    large: {
      size: 'orj360' | 'large',
      url: string,
    }
  }[];
  // video and other data
  page_info?: {
    type: 'video' | 'search_topic';
    // eg: 'http://t.cn/A6K3ITkN'
    url_ori: string;
    // eg: '搜狐新闻的微博视频'
    page_title: string;
    // eg: '2022触动瞬间'
    title: string;
    // eg: '搜狐新闻的微博视频'
    content1: string;
    content2: string;
    // more...
    [x: string]: any;
  };
  retweeted_status?: WeiboStatus;
  [x: string]: any;
}

export interface WeiboUserData {
  uid: string,
  screenName: string,
  description: string,
  containerId?: string,
  statusList?: WeiboStatus[],
}

export interface InstagramMedia {
  id: string;
  // 帖子短码，如 'C0abcdefg'
  shortcode: string;
  display_url?: string;
  video_url?: string;
  is_video: boolean;
  taken_at_timestamp: number;
  edge_media_to_caption?: {
    edges?: {
      node?: {
        text: string;
      };
    }[];
  };
  edge_sidecar_to_children?: {
    edges?: {
      node?: {
        display_url?: string;
        video_url?: string;
        is_video: boolean;
      };
    }[];
  };
  [x: string]: any;
}

export interface InstagramUserData {
  id: string;
  username: string;
  name: string;
  description: string;
  profileImageUrl?: string;
  publicMetrics?: {
    followers_count: number;
    following_count: number;
    post_count: number;
  };
  media: InstagramMedia[];
}

export interface TwitterStatus {
  id: string;
  text: string;
  created_at: string;
  publicMetrics: {
    retweet_count: number;
    reply_count: number;
    like_count: number;
    quote_count: number;
  };
  entities?: {
    urls?: any[];
    hashtags?: any[];
    mentions?: any[];
  };
}

export interface TwitterUserData {
  id: string;
  username: string;
  name: string;
  description: string;
  profileImageUrl?: string;
  publicMetrics?: {
    followers_count: number;
    following_count: number;
    tweet_count: number;
    listed_count: number;
  };
  tweets: TwitterStatus[];
}
