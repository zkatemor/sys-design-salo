package cache

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestGetMissThenHit(t *testing.T) {
	c := New()
	if _, ok := c.Get("k"); ok {
		t.Fatal("want miss")
	}
	c.Set("k", []byte("v"), time.Second)
	v, ok := c.Get("k")
	if !ok || string(v) != "v" {
		t.Fatalf("got v=%q ok=%v", v, ok)
	}
}

func TestTTLExpires(t *testing.T) {
	c := New()
	c.Set("k", []byte("v"), 30*time.Millisecond)
	time.Sleep(60 * time.Millisecond)
	if _, ok := c.Get("k"); ok {
		t.Fatal("want expired miss")
	}
}

func TestGetOrLoadCallsLoaderOnMiss(t *testing.T) {
	c := New()
	var calls atomic.Int64
	v, err := c.GetOrLoad(context.Background(), "k", time.Second, func(_ context.Context) ([]byte, error) {
		calls.Add(1)
		return []byte("v"), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(v) != "v" || calls.Load() != 1 {
		t.Fatalf("v=%q calls=%d", v, calls.Load())
	}
}

func TestGetOrLoadReturnsCachedValue(t *testing.T) {
	c := New()
	c.Set("k", []byte("v"), time.Second)

	var calls atomic.Int64
	v, err := c.GetOrLoad(context.Background(), "k", time.Second, func(_ context.Context) ([]byte, error) {
		calls.Add(1)
		return []byte("nope"), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(v) != "v" || calls.Load() != 0 {
		t.Fatalf("v=%q calls=%d (loader must not be called on hit)", v, calls.Load())
	}
}

// Главный кейс: 50 параллельных промахов по одному ключу должны
// привести к ОДНОМУ вызову loader.
func TestGetOrLoadSingleflight(t *testing.T) {
	c := New()
	var calls atomic.Int64
	block := make(chan struct{})

	loader := func(_ context.Context) ([]byte, error) {
		calls.Add(1)
		<-block
		return []byte("v"), nil
	}

	const N = 50
	var wg sync.WaitGroup
	vals := make([]string, N)
	errs := make([]error, N)
	started := make(chan struct{}, N)

	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			started <- struct{}{}
			v, err := c.GetOrLoad(context.Background(), "k", time.Second, loader)
			vals[i] = string(v)
			errs[i] = err
		}(i)
	}
	for i := 0; i < N; i++ {
		<-started
	}
	// Дать всем шанс зайти в GetOrLoad и упереться в singleflight.
	time.Sleep(20 * time.Millisecond)
	close(block)
	wg.Wait()

	for i := 0; i < N; i++ {
		if errs[i] != nil {
			t.Fatalf("g%d err: %v", i, errs[i])
		}
		if vals[i] != "v" {
			t.Fatalf("g%d val: %q", i, vals[i])
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("loader called %d times, want 1 (thundering herd!)", got)
	}
}
