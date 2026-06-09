package crawler

import (
	"context"
	"testing"
	"time"
)

type fakeClock struct {
	now time.Time
}

func (f *fakeClock) Now() time.Time { return f.now }

func (f *fakeClock) Advance(d time.Duration) { f.now = f.now.Add(d) }

func TestRateLimiterDelayWaitsBetweenRequests(t *testing.T) {
	clk := &fakeClock{now: time.Unix(0, 0)}
	var waits []time.Duration

	rl := newRateLimiter(Options{Delay: 100 * time.Millisecond}, clk, func(ctx context.Context, d time.Duration) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		waits = append(waits, d)
		clk.Advance(d)
		return nil
	})

	ctx := context.Background()
	if err := rl.wait(ctx); err != nil {
		t.Fatalf("first wait: %v", err)
	}
	if err := rl.wait(ctx); err != nil {
		t.Fatalf("second wait: %v", err)
	}
	if err := rl.wait(ctx); err != nil {
		t.Fatalf("third wait: %v", err)
	}

	if len(waits) != 2 {
		t.Fatalf("wait count = %d, want 2", len(waits))
	}
	for i, got := range waits {
		if got < 100*time.Millisecond {
			t.Errorf("wait[%d] = %v, want >= 100ms", i, got)
		}
	}
}

func TestRateLimiterRPSOverridesDelay(t *testing.T) {
	clk := &fakeClock{now: time.Unix(0, 0)}
	var waits []time.Duration

	rl := newRateLimiter(Options{
		Delay: 500 * time.Millisecond,
		RPS:   5,
	}, clk, func(ctx context.Context, d time.Duration) error {
		waits = append(waits, d)
		clk.Advance(d)
		return nil
	})

	ctx := context.Background()
	_ = rl.wait(ctx)
	_ = rl.wait(ctx)

	if len(waits) != 1 {
		t.Fatalf("wait count = %d, want 1", len(waits))
	}
	want := 200 * time.Millisecond
	if waits[0] != want {
		t.Errorf("wait = %v, want %v", waits[0], want)
	}
}

func TestRateLimiterNoLimit(t *testing.T) {
	rl := newRateLimiter(Options{}, nil, nil)
	if rl != nil {
		t.Fatal("expected nil limiter without delay and rps")
	}
}

func TestRateLimiterContextCancelStopsWaiting(t *testing.T) {
	clk := &fakeClock{now: time.Unix(0, 0)}
	rl := newRateLimiter(Options{Delay: time.Second}, clk, realSleeper)

	ctx, cancel := context.WithCancel(context.Background())
	_ = rl.wait(ctx)
	cancel()

	start := time.Now()
	err := rl.wait(ctx)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected context error")
	}
	if elapsed > 100*time.Millisecond {
		t.Errorf("wait after cancel took %v, want immediate return", elapsed)
	}
}

func TestRateLimiterMaxRequestsInPeriod(t *testing.T) {
	clk := &fakeClock{now: time.Unix(0, 0)}
	rl := newRateLimiter(Options{RPS: 2}, clk, func(ctx context.Context, d time.Duration) error {
		clk.Advance(d)
		return nil
	})

	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if err := rl.wait(ctx); err != nil {
			t.Fatalf("wait %d: %v", i, err)
		}
	}

	elapsed := clk.now.Sub(time.Unix(0, 0))
	minElapsed := 2 * 500*time.Millisecond
	if elapsed < minElapsed {
		t.Errorf("elapsed = %v, want >= %v", elapsed, minElapsed)
	}
}
