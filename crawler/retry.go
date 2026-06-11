package crawler

import (
	"context"
	"io"
	"net/http"
	"time"
)

const retryBackoff = 100 * time.Millisecond

func isRetryableStatus(code int) bool {
	return code == http.StatusTooManyRequests || code >= http.StatusInternalServerError
}

func doHTTPWithRetry(ctx context.Context, opts Options, rl *rateLimiter, req *http.Request) (*http.Response, error) {
	return doHTTPWithRetryAndSleep(ctx, opts, rl, req, realSleeper)
}

func doHTTPWithRetryAndSleep(ctx context.Context, opts Options, rl *rateLimiter, req *http.Request, sleep sleeper) (*http.Response, error) {
	attempts := opts.Retries + 1
	if attempts < 1 {
		attempts = 1
	}

	var lastResp *http.Response

	for attempt := 0; attempt < attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if attempt > 0 {
			if err := sleep(ctx, retryBackoff); err != nil {
				return nil, err
			}
		}

		resp, err := doHTTP(ctx, opts, rl, req)
		if err != nil {
			if attempt == attempts-1 {
				return nil, err
			}
			continue
		}

		if !isRetryableStatus(resp.StatusCode) {
			return resp, nil
		}

		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		lastResp = resp

		if attempt == attempts-1 {
			return resp, nil
		}
	}

	return lastResp, nil
}
