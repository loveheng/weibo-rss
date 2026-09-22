// Package anticrawl 实现公共防风控机制，设计移植自 TS 版 antiCrawl.ts：
//   - 请求重试（指数退避 + 随机抖动）；
//   - 风控响应识别（可按源覆盖，如微博 403/418、Instagram 401/403/429）；
//   - 命中风控后的回调链：先执行源特有钩子（如刷新访客 Cookie），再熔断。
package anticrawl

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"time"
)

// DefaultRiskyStatuses 为默认识别为风控/限流的状态码。
var DefaultRiskyStatuses = []int{401, 403, 418, 429}

// StatusError 表示上游返回了特定 HTTP 状态码。
type StatusError struct {
	Code int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("upstream status: %d", e.Code)
}

// Hooks 为源特有风控钩子。
type Hooks struct {
	// RiskyStatuses 覆盖该源识别为风控的状态码；为空时使用 DefaultRiskyStatuses
	RiskyStatuses []int
	// OnRiskDetected 命中风控时触发（如刷新访客 Cookie），在熔断回调之前执行
	OnRiskDetected func(ctx context.Context) error
}

func (h *Hooks) isRisky(code int) bool {
	statuses := DefaultRiskyStatuses
	if h != nil && len(h.RiskyStatuses) > 0 {
		statuses = h.RiskyStatuses
	}
	for _, s := range statuses {
		if s == code {
			return true
		}
	}
	return false
}

// Doer 执行一次上游请求，返回未读取的响应体。
type Doer func(ctx context.Context) (*http.Response, error)

// DoWithRetry 带重试执行请求：指数退避 + 随机抖动；
// 命中风控状态码时立即停止重试并返回 *StatusError。
func DoWithRetry(ctx context.Context, h *Hooks, do Doer, retries int, baseDelay time.Duration) (*http.Response, error) {
	if baseDelay <= 0 {
		baseDelay = time.Second
	}
	attempt := 0
	for {
		resp, err := do(ctx)
		if err == nil {
			if h.isRisky(resp.StatusCode) {
				drainClose(resp)
				return nil, &StatusError{Code: resp.StatusCode}
			}
			return resp, nil
		}
		var se *StatusError
		if errors.As(err, &se) {
			return nil, se
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if attempt >= retries {
			return nil, err
		}
		delay := baseDelay<<attempt + time.Duration(rand.IntN(300))*time.Millisecond
		if sleepErr := sleep(ctx, delay); sleepErr != nil {
			return nil, sleepErr
		}
		attempt++
	}
}

// HandleForbidden 统一处理风控错误，等价于 TS 版 handleForbiddenErr：
// 命中风控时先执行 OnRiskDetected 钩子，再执行熔断回调 trip；
// 非风控错误原样返回。
func HandleForbidden(ctx context.Context, h *Hooks, err error, trip func()) error {
	var se *StatusError
	if errors.As(err, &se) && h.isRisky(se.Code) {
		if h.OnRiskDetected != nil {
			// 钩子失败不阻塞熔断（与原版静默失败语义一致）
			_ = h.OnRiskDetected(ctx)
		}
		trip()
		return ErrRisky
	}
	return err
}

// ErrRisky 在钩子与熔断均执行完毕后返回，调用方可据此映射为 503。
var ErrRisky = errors.New("anticrawl: risky status detected")

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// drainClose 读取并关闭响应体，保证连接可复用。
func drainClose(resp *http.Response) {
	if resp.Body != nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
		_ = resp.Body.Close()
	}
}
