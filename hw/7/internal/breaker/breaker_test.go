package breaker

import (
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newClock() *fakeClock {
	return &fakeClock{now: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

var errBoom = errors.New("boom")

func newTestBreaker(t *testing.T, clock *fakeClock) *Breaker {
	t.Helper()
	b, err := New(Config{
		FailureThreshold:  3,
		OpenTimeout:       time.Second,
		HalfOpenMaxProbes: 2,
		SuccessThreshold:  2,
		Now:               clock.Now,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return b
}

func TestClosedToOpenAfterFailureThreshold(t *testing.T) {
	clock := newClock()
	b := newTestBreaker(t, clock)

	for i := 0; i < 3; i++ {
		_ = b.Execute(func() error { return errBoom })
	}
	if got := b.State(); got != StateOpen {
		t.Fatalf("state = %v, want Open", got)
	}
}

func TestOpenRejectsCalls(t *testing.T) {
	clock := newClock()
	b := newTestBreaker(t, clock)

	for i := 0; i < 3; i++ {
		_ = b.Execute(func() error { return errBoom })
	}

	called := false
	err := b.Execute(func() error {
		called = true
		return nil
	})
	if !errors.Is(err, ErrOpen) {
		t.Fatalf("err = %v, want ErrOpen", err)
	}
	if called {
		t.Fatal("fn must NOT be called while breaker is open")
	}
}

func TestOpenToHalfOpenAfterTimeout(t *testing.T) {
	clock := newClock()
	b := newTestBreaker(t, clock)

	for i := 0; i < 3; i++ {
		_ = b.Execute(func() error { return errBoom })
	}
	clock.Advance(2 * time.Second)

	if got := b.State(); got != StateHalfOpen {
		t.Fatalf("state = %v, want HalfOpen", got)
	}
}

func TestHalfOpenSuccessClosesBreaker(t *testing.T) {
	clock := newClock()
	b := newTestBreaker(t, clock)

	for i := 0; i < 3; i++ {
		_ = b.Execute(func() error { return errBoom })
	}
	clock.Advance(2 * time.Second)

	for i := 0; i < 2; i++ {
		if err := b.Execute(func() error { return nil }); err != nil {
			t.Fatalf("probe %d: unexpected err %v", i, err)
		}
	}
	if got := b.State(); got != StateClosed {
		t.Fatalf("state = %v, want Closed", got)
	}
}

func TestHalfOpenFailureReopens(t *testing.T) {
	clock := newClock()
	b := newTestBreaker(t, clock)

	for i := 0; i < 3; i++ {
		_ = b.Execute(func() error { return errBoom })
	}
	clock.Advance(2 * time.Second)

	_ = b.Execute(func() error { return errBoom })

	if got := b.State(); got != StateOpen {
		t.Fatalf("state = %v, want Open", got)
	}
}

func TestHalfOpenLimitsConcurrentProbes(t *testing.T) {
	clock := newClock()
	b := newTestBreaker(t, clock)

	for i := 0; i < 3; i++ {
		_ = b.Execute(func() error { return errBoom })
	}
	clock.Advance(2 * time.Second)

	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = b.Execute(func() error {
				entered <- struct{}{}
				<-release
				return nil
			})
		}()
	}
	for i := 0; i < 2; i++ {
		<-entered
	}

	err := b.Execute(func() error { return nil })
	if !errors.Is(err, ErrTooManyProbes) {
		t.Fatalf("err = %v, want ErrTooManyProbes", err)
	}

	close(release)
	wg.Wait()
}

func TestClosedSuccessResetsFailureCounter(t *testing.T) {
	clock := newClock()
	b := newTestBreaker(t, clock)

	_ = b.Execute(func() error { return errBoom })
	_ = b.Execute(func() error { return errBoom })
	_ = b.Execute(func() error { return nil })
	_ = b.Execute(func() error { return errBoom })
	_ = b.Execute(func() error { return errBoom })

	if got := b.State(); got != StateClosed {
		t.Fatalf("state = %v, want Closed", got)
	}
}
