package cache

import (
	"context"
	"errors"
	"testing"
	"time"
)

func newTestCache() *Cache { return New(100, nil) }

func TestCacheSetGet(t *testing.T) {
	c := newTestCache()
	c.Set("k", "v", time.Minute)
	v, ok := c.Get("k")
	if !ok || v != "v" {
		t.Fatalf("expected hit v, got %v ok=%v", v, ok)
	}
	if _, ok := c.Get("missing"); ok {
		t.Fatal("expected miss for missing key")
	}
}

func TestCacheTTLExpire(t *testing.T) {
	c := newTestCache()
	c.Set("k", "v", 30*time.Millisecond)
	if _, ok := c.Get("k"); !ok {
		t.Fatal("expected hit before expiry")
	}
	time.Sleep(60 * time.Millisecond)
	if _, ok := c.Get("k"); ok {
		t.Fatal("expected miss after expiry")
	}
	if c.Len() != 0 {
		t.Fatalf("expected expired entry removed, len=%d", c.Len())
	}
}

func TestCacheDefaultTTL(t *testing.T) {
	c := newTestCache()
	c.Set("k", "v", 0)
	if _, ok := c.Get("k"); !ok {
		t.Fatal("expected hit with default ttl")
	}
}

func TestCacheLRUEviction(t *testing.T) {
	c := New(2, nil)
	c.Set("a", 1, time.Minute)
	c.Set("b", 2, time.Minute)
	c.Get("a") // 刷新 a 的近期性
	c.Set("c", 3, time.Minute)
	if _, ok := c.Get("b"); ok {
		t.Fatal("expected b evicted by LRU")
	}
	if _, ok := c.Get("a"); !ok {
		t.Fatal("expected a retained")
	}
}

func TestMemo(t *testing.T) {
	c := newTestCache()
	ctx := context.Background()
	calls := 0
	fetch := func(ctx context.Context) (string, error) {
		calls++
		return "val", nil
	}

	got, err := Memo(ctx, c, "k", time.Minute, fetch)
	if err != nil || got != "val" {
		t.Fatalf("first memo: got=%v err=%v", got, err)
	}
	if _, err = Memo(ctx, c, "k", time.Minute, fetch); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("expected fetch called once, got %d", calls)
	}
}

func TestMemoDoesNotCacheError(t *testing.T) {
	c := newTestCache()
	ctx := context.Background()
	calls := 0
	fail := true
	fetch := func(ctx context.Context) (string, error) {
		calls++
		if fail {
			return "", errors.New("boom")
		}
		return "ok", nil
	}

	if _, err := Memo(ctx, c, "k", time.Minute, fetch); err == nil {
		t.Fatal("expected error from first call")
	}
	fail = false
	if _, err := Memo(ctx, c, "k", time.Minute, fetch); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("expected failed fetch not cached (2 calls), got %d", calls)
	}
}
