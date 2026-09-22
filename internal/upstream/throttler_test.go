package upstream

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunSerializes(t *testing.T) {
	tr := New("test", nil, time.Minute)
	var concurrent, maxConcurrent int32
	var wg sync.WaitGroup

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = tr.Run(context.Background(), func() error {
				cur := atomic.AddInt32(&concurrent, 1)
				for {
					old := atomic.LoadInt32(&maxConcurrent)
					if cur <= old || atomic.CompareAndSwapInt32(&maxConcurrent, old, cur) {
						break
					}
				}
				time.Sleep(10 * time.Millisecond)
				atomic.AddInt32(&concurrent, -1)
				return nil
			})
		}()
	}
	wg.Wait()

	if maxConcurrent != 1 {
		t.Fatalf("expected max concurrency 1, got %d", maxConcurrent)
	}
}

func TestTripAndRecover(t *testing.T) {
	tr := New("test", nil, 50*time.Millisecond)

	tr.Trip()
	if err := tr.Run(context.Background(), func() error { return nil }); !errors.Is(err, ErrThrottled) {
		t.Fatalf("expected ErrThrottled, got %v", err)
	}

	time.Sleep(60 * time.Millisecond)
	if err := tr.Run(context.Background(), func() error { return nil }); err != nil {
		t.Fatalf("expected recovery after cooldown, got %v", err)
	}
}

func TestTripWhileQueued(t *testing.T) {
	tr := New("test", nil, time.Minute)
	release := make(chan struct{})

	go func() { _ = tr.Run(context.Background(), func() error { <-release; return nil }) }()
	time.Sleep(20 * time.Millisecond) // 确保第一个请求已持有资格
	tr.Trip()

	// 排队等待期间被熔断：获取资格后应二次校验并拒绝
	err := tr.Run(context.Background(), func() error { return nil })
	close(release)
	if !errors.Is(err, ErrThrottled) {
		t.Fatalf("expected ErrThrottled for queued request after trip, got %v", err)
	}
}

func TestContextCancelWhileQueued(t *testing.T) {
	tr := New("test", nil, time.Minute)
	release := make(chan struct{})
	go func() { _ = tr.Run(context.Background(), func() error { <-release; return nil }) }()
	time.Sleep(20 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := tr.Run(ctx, func() error { return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	close(release)
}
