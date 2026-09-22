// Package feed 提供 RSS 输出的统一组装，替代 TS 版 routes.ts 中
// 直接调用 NodeRSS 的部分。各数据源只负责提供条目数据。
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

// BuildRSS 组装 RSS 2.0 XML。
func BuildRSS(siteURL, title, description string, items []Item) (string, error) {
	f := &feeds.Feed{
		Title:       title,
		Link:        &feeds.Link{Href: siteURL},
		Description: description,
	}
	for _, it := range items {
		f.Add(&feeds.Item{
			Title:       it.Title,
			Link:        &feeds.Link{Href: it.Link},
			Description: it.Description,
			Created:     it.Time,
		})
	}
	return f.ToRss()
}
