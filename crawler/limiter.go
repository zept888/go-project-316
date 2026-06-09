package crawler

import (
	"context"
	"net/http"
	"time"
)

type clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

type sleeper func(ctx context.Context, d time.Duration) error

func realSleeper(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type rateLimiter struct {
	interval time.Duration
	last     time.Time
	clock    clock
	sleeper  sleeper
}

func newRateLimiter(opts Options, clk clock, sleep sleeper) *rateLimiter {
	var interval time.Duration
	switch {
	case opts.RPS > 0:
		interval = time.Duration(float64(time.Second) / opts.RPS)
	case opts.Delay > 0:
		interval = opts.Delay
	default:
		return nil
	}

	if clk == nil {
		clk = realClock{}
	}
	if sleep == nil {
		sleep = realSleeper
	}

	return &rateLimiter{
		interval: interval,
		last:     clk.Now().Add(-interval),
		clock:    clk,
		sleeper:  sleep,
	}
}

func (l *rateLimiter) wait(ctx context.Context) error {
	if l == nil {
		return nil
	}

	next := l.last.Add(l.interval)
	now := l.clock.Now()
	if now.Before(next) {
		if err := l.sleeper(ctx, next.Sub(now)); err != nil {
			return err
		}
	}

	l.last = l.clock.Now()
	return nil
}

func doHTTP(ctx context.Context, opts Options, rl *rateLimiter, req *http.Request) (*http.Response, error) {
	if rl != nil {
		if err := rl.wait(ctx); err != nil {
			return nil, err
		}
	}
	return opts.HTTPClient.Do(req)
}
