// Package feed 提供 RSS 输出的统一组装。
// 各订阅源只需组装 Channel 结构，无需感知 XML 细节。
package feed

import (
	"time"

	"github.com/gorilla/feeds"
)

// Item 为单条 RSS 条目。
type Item struct {
	Title       string
	Description string
	Link        string
	Time        time.Time
}

// Channel 为一个订阅源的频道信息与条目列表。
type Channel struct {
	SiteURL     string
	Title       string
	Description string
	Items       []Item
}

// BuildRSS 组装 RSS 2.0 XML。
func BuildRSS(c Channel) (string, error) {
	f := &feeds.Feed{
		Title:       c.Title,
		Link:        &feeds.Link{Href: c.SiteURL},
		Description: c.Description,
	}
	for _, it := range c.Items {
		f.Add(&feeds.Item{
			Title:       it.Title,
			Link:        &feeds.Link{Href: it.Link},
			Description: it.Description,
			Created:     it.Time,
		})
	}
	return f.ToRss()
}
