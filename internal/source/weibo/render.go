package weibo

// 本文件对应 TS 版 weibo.ts 的 statusToHTML：微博条目转 RSS 正文 HTML。

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/zgq354/weibo-rss/internal/config"
)

// weiboTimeLayout 为微博 created_at 的时间格式。
const weiboTimeLayout = "Mon Jan 02 15:04:05 -0700 2006"

// ParseWeiboTime 解析微博时间，失败时返回零值。
func ParseWeiboTime(s string) time.Time {
	t, err := time.Parse(weiboTimeLayout, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

var (
	// 表情图转文字：alt 属性为表情名
	emojiAltRe = regexp.MustCompile(`<span class="url-icon"><img alt="?(.*?)"? src=".*?" style="width:1em; height:1em;".*?/></span>`)
	// 去掉外链图标
	urlIconRe = regexp.MustCompile(`<span class='url-icon'><img.*?></span>`)
	// 标题用：去标签、去换行
	tagRe     = regexp.MustCompile(`<[^>]+>`)
	newlineRe = regexp.MustCompile(`\n`)
)

// StatusToHTML 将微博条目转换为 RSS 正文 HTML：
// 表情转文字、外链图标去除、转发块引用、配图（走图片反代）。
func StatusToHTML(cfg config.Config, st *Status) string {
	var b strings.Builder
	b.WriteString(emojiAltRe.ReplaceAllString(urlIconRe.ReplaceAllString(st.Text, ""), "$1"))

	// 转发的微博（可能已被删除：user 为空则省略引用块头部）
	if st.Retweeted != nil {
		b.WriteString("<br><br>")
		if st.Retweeted.User != nil {
			fmt.Fprintf(&b,
				`<div style="border-left: 3px solid gray; padding-left: 1em;">转发 <a href="https://weibo.com/%d" target="_blank">@%s</a>: %s</div>`,
				st.Retweeted.User.ID, st.Retweeted.User.ScreenName, StatusToHTML(cfg, st.Retweeted))
		}
	}

	// 微博配图
	for _, pic := range st.Pics {
		b.WriteString("<br><br>")
		u := cfg.WrapImageURL(pic.Large.URL)
		fmt.Fprintf(&b, `<a href="%s" target="_blank"><img src="%s"></a>`, u, u)
	}
	return b.String()
}

// FeedTitle 生成条目标题：优先 status_title，否则取正文去标签后的前 25 字。
func FeedTitle(st *Status) string {
	if st.StatusTitle != "" {
		return st.StatusTitle
	}
	text := st.Text
	if text == "" {
		return ""
	}
	text = newlineRe.ReplaceAllString(tagRe.ReplaceAllString(text, ""), "")
	runes := []rune(text)
	if len(runes) > 25 {
		runes = runes[:25]
	}
	return string(runes)
}

// itoa 为 strconv.Itoa 的本地别名，避免包级 import 冲突。
func itoa(n int64) string { return strconv.FormatInt(n, 10) }
