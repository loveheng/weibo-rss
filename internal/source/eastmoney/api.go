package eastmoney

// 本文件对应股吧个人中心的两个上游接口：
//   - postCenterList：博主发帖列表（含正文、配图、所属股吧）；
//   - myreply：博主回复列表（含被回复内容与原帖信息）。

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/loveheng/weibo-rss/internal/source"
)

// pageSize 为每次拉取的条数（RSS 只展示最新一页）。
const pageSize = 20

// emUser 为用户精简信息。
type emUser struct {
	UserID   string `json:"user_id"`
	Nickname string `json:"user_nickname"`
}

// emGuba 为帖子所属股吧信息。
type emGuba struct {
	StockbarCode string `json:"stockbar_code"`
	StockbarName string `json:"stockbar_name"`
}

// postItem 为一条博主帖子。
type postItem struct {
	PostID           int64    `json:"post_id"`
	PostTitle        string   `json:"post_title"`
	PostContent      string   `json:"post_content"`
	PostPublishTime  string   `json:"post_publish_time"`
	PostFrom         string   `json:"post_from"`
	PostPicURL       []string `json:"post_pic_url"`
	PostVideoURL     string   `json:"post_video_url"`
	PostLikeCount    int      `json:"post_like_count"`
	PostCommentCount int      `json:"post_comment_count"`
	PostUser         emUser   `json:"post_user"`
	PostGuba         emGuba   `json:"post_guba"`
}

// postListResp 为 postCenterList 的精简响应结构。
type postListResp struct {
	Re     bool       `json:"re"`
	Result []postItem `json:"result"`
}

// replyItem 为一条博主回复。
type replyItem struct {
	ReplyID          int64   `json:"reply_id"`
	ReplyText        string  `json:"reply_text"`
	ReplyPublishTime string  `json:"reply_publish_time"`
	ReplyPicture     string  `json:"reply_picture"`
	ReplyIPAddress   string  `json:"reply_ip_address"`
	SourceReplyID    int64   `json:"source_reply_id"`
	SourceReplyText  string  `json:"source_reply_text"`
	SourceReplyUser  string  `json:"source_reply_user_nickname"`
	SourceReplyTime  *string `json:"source_reply_time"`
	SourcePostID     int64   `json:"source_post_id"`
	SourcePostState  int     `json:"source_post_state"`
	SourcePostTitle  string  `json:"source_post_title"`
	ReplyUser        emUser  `json:"reply_user"`
	ReplyGuba        emGuba  `json:"reply_guba"`
}

// replyListResp 为 myreply 的精简响应结构。
type replyListResp struct {
	Re     bool `json:"re"`
	Result struct {
		Count int         `json:"count"`
		List  []replyItem `json:"list"`
	} `json:"result"`
}

// fetchPosts 拉取博主发帖列表；用户不存在时包装 source.ErrNotFound。
func (s *Service) fetchPosts(ctx context.Context, uid string) ([]postItem, error) {
	u := fmt.Sprintf("https://i.eastmoney.com/api/guba/postCenterList?uid=%s&pagenum=1&pagesize=%d&type=1&filterType=0&onlyYt=0", uid, pageSize)
	var r postListResp
	if err := s.fetcher.JSON(ctx, s.runner, http.MethodGet, u, s.headers(uid), &r); err != nil {
		return nil, err
	}
	if !r.Re {
		return nil, fmt.Errorf("%w: uid %s", source.ErrNotFound, uid)
	}
	return r.Result, nil
}

// fetchReplies 拉取博主回复列表；用户不存在时包装 source.ErrNotFound。
func (s *Service) fetchReplies(ctx context.Context, uid string) ([]replyItem, error) {
	u := fmt.Sprintf("https://i.eastmoney.com/api/guba/myreply?pageindex=1&uid=%s&checkauth=true", uid)
	var r replyListResp
	if err := s.fetcher.JSON(ctx, s.runner, http.MethodGet, u, s.headers(uid), &r); err != nil {
		return nil, err
	}
	if !r.Re {
		return nil, fmt.Errorf("%w: uid %s", source.ErrNotFound, uid)
	}
	return r.Result.List, nil
}

// headers 为请求附带浏览器 XHR 特征头部。
func (s *Service) headers(uid string) map[string]string {
	return map[string]string{
		"Accept":           "application/json, text/javascript, */*; q=0.01",
		"X-Requested-With": "XMLHttpRequest",
		"Referer":          "https://i.eastmoney.com/" + uid,
	}
}

// emCST 为东方财富时间所属的东八区。
var emCST = time.FixedZone("CST", 8*60*60)

// parseEmTime 解析 "2006-01-02 15:04:05" 格式的北京时间，失败返回零值。
func parseEmTime(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04:05", s, emCST)
	if err != nil {
		return time.Time{}
	}
	return t
}
