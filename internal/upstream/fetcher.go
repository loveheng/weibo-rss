package upstream

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// Fetcher 组合 Client 与风控钩子，提供「限流 + 抖动 + 重试 + 风控处理」
// 的通用请求骨架。
type Fetcher struct {
	Client    *Client
	Hooks     *Hooks
	Retries   int
	BaseDelay time.Duration
}

// NewFetcher 创建 Fetcher，填充重试参数默认值（2 次、1s 基准退避）。
func NewFetcher(client *Client, hooks *Hooks) *Fetcher {
	return &Fetcher{Client: client, Hooks: hooks, Retries: 2, BaseDelay: time.Second}
}

// Do 在限流器内执行带抖动、重试与风控处理的请求：
//   - 冷却期内或风控熔断时返回 ErrThrottled / ErrRisky；
//   - 成功时返回未读取的响应体（调用方负责关闭）。
func (f *Fetcher) Do(ctx context.Context, runner *Throttler, method, rawURL string, headers map[string]string) (*http.Response, error) {
	var resp *http.Response
	err := runner.Run(ctx, func() error {
		if err := Jitter(ctx, 100*time.Millisecond); err != nil {
			return err
		}
		r, err := DoWithRetry(ctx, f.Hooks, func(ctx context.Context) (*http.Response, error) {
			return f.Client.Do(ctx, method, rawURL, nil, headers)
		}, f.Retries, f.BaseDelay)
		if err != nil {
			return HandleForbidden(ctx, f.Hooks, err, runner.Trip)
		}
		resp = r
		return nil
	})
	return resp, err
}

// JSON 同 Do，并将响应体解码进 out（随后关闭响应体）。
func (f *Fetcher) JSON(ctx context.Context, runner *Throttler, method, rawURL string, headers map[string]string, out any) error {
	resp, err := f.Do(ctx, runner, method, rawURL, headers)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(out)
}
