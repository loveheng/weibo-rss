package weibo

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/loveheng/weibo-rss/internal/config"
)

// 用带噪音的真实结构样本锁定解析行为：
// cards 中混有无 mblog 的卡片（广告/热门），mblog 内含转发与配图。
const sampleListJSON = `{
  "ok": 1,
  "data": {
    "cards": [
      {"card_type": 11, "card_group": []},
      {"card_type": 9, "mblog": {
        "id": "4851725594528288",
        "bid": "MlHCnj00U",
        "created_at": "Tue Sep 20 10:00:00 +0800 2022",
        "text": "hello <span class=\"url-icon\"><img alt=\"[赞]\" src=\"x.png\" style=\"width:1em; height:1em;\"/></span> world",
        "isLongText": false,
        "user": {"id": 1234567890, "screen_name": "tester"},
        "pics": [{"pid": "p1", "large": {"size": "large", "url": "https://wx1.sinaimg.cn/large/abc.jpg"}}]
      }},
      {"card_type": 9, "mblog": {
        "id": "4851725594528289",
        "bid": "AbCdEf01X",
        "created_at": "Tue Sep 20 11:00:00 +0800 2022",
        "text": "转发微博",
        "isLongText": false,
        "user": {"id": 1234567890, "screen_name": "tester"},
        "retweeted_status": {
          "id": "4851725594528200",
          "bid": "RtWeibo01",
          "created_at": "Tue Sep 20 09:00:00 +0800 2022",
          "text": "被转发的原文",
          "isLongText": false,
          "user": {"id": 9876543210, "screen_name": "origin"}
        }
      }}
    ]
  }
}`

func TestParseIndexList(t *testing.T) {
	var r indexListResp
	if err := json.Unmarshal([]byte(sampleListJSON), &r); err != nil {
		t.Fatal(err)
	}
	if r.OK != 1 {
		t.Fatalf("expected ok=1, got %d", r.OK)
	}
	var statuses []*Status
	for _, card := range r.Data.Cards {
		if card.Mblog != nil {
			statuses = append(statuses, card.Mblog)
		}
	}
	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses (noise card filtered), got %d", len(statuses))
	}

	first := statuses[0]
	if first.ID != "4851725594528288" || first.Bid != "MlHCnj00U" {
		t.Fatalf("unexpected first status: %+v", first)
	}
	if first.User == nil || first.User.ScreenName != "tester" {
		t.Fatal("expected user parsed")
	}
	if len(first.Pics) != 1 || first.Pics[0].Large.URL != "https://wx1.sinaimg.cn/large/abc.jpg" {
		t.Fatal("expected pic parsed")
	}

	rt := statuses[1].Retweeted
	if rt == nil || rt.User == nil || rt.User.ScreenName != "origin" {
		t.Fatal("expected nested retweet parsed")
	}
}

func TestStatusToHTML(t *testing.T) {
	var r indexListResp
	_ = json.Unmarshal([]byte(sampleListJSON), &r)
	cfg := config.Config{ImageCache: "https://img.example/?url="}

	// 表情图应转为 alt 文本，配图应走反代前缀
	first := r.Data.Cards[1].Mblog
	html := StatusToHTML(cfg, first)
	if !contains(html, "[赞]") || contains(html, "url-icon") {
		t.Fatalf("emoji not converted: %s", html)
	}
	if !contains(html, "https://img.example/?url=https%3A%2F%2Fwx1.sinaimg.cn%2Flarge%2Fabc.jpg") {
		t.Fatalf("pic not wrapped with image cache: %s", html)
	}

	// 转发块应带引用样式与作者链接
	rt := r.Data.Cards[2].Mblog
	rtHTML := StatusToHTML(cfg, rt)
	if !contains(rtHTML, `border-left: 3px solid gray`) || !contains(rtHTML, "@origin") {
		t.Fatalf("retweet block missing: %s", rtHTML)
	}
}

func TestFeedTitle(t *testing.T) {
	st := &Status{StatusTitle: "官方标题"}
	if FeedTitle(st) != "官方标题" {
		t.Fatal("expected status_title priority")
	}
	long := &Status{Text: "<b>" + strings.Repeat("汉", 30) + "</b>"}
	title := FeedTitle(long)
	if got := len([]rune(title)); got != 25 {
		t.Fatalf("expected 25 runes, got %d", got)
	}
	if contains(title, "<b>") {
		t.Fatalf("tags should be stripped: %s", title)
	}
}

func TestParseWeiboTime(t *testing.T) {
	got := ParseWeiboTime("Tue Sep 20 10:00:00 +0800 2022")
	if got.IsZero() {
		t.Fatal("expected valid time")
	}
	if got.UTC().Hour() != 2 {
		t.Fatalf("expected 02:00 UTC, got %v", got.UTC())
	}
	if !ParseWeiboTime("garbage").IsZero() {
		t.Fatal("expected zero time on invalid input")
	}
}

func TestExtractUID(t *testing.T) {
	cases := map[string]struct {
		want string
		ok   bool
	}{
		"/u/1234567890":     {"1234567890", true},
		"/u/1234567890?x=1": {"1234567890", true},
		"/1234567890":       {"", false},
		"/u/":               {"", false},
	}
	for path, want := range cases {
		got, ok := extractUID(path)
		if got != want.want || ok != want.ok {
			t.Fatalf("extractUID(%q) = %q, %v; want %q, %v", path, got, ok, want.want, want.ok)
		}
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }
