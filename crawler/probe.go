package crawler

import (
	"context"
	"io"
	"net/http"
)

func checkBrokenLinks(ctx context.Context, opts Options, links []string) []BrokenLink {
	var broken []BrokenLink

	for _, linkURL := range links {
		statusCode, err := probeURL(ctx, opts, linkURL)
		if err != nil {
			broken = append(broken, BrokenLink{
				URL:   linkURL,
				Error: err.Error(),
			})
			continue
		}
		if statusCode >= http.StatusBadRequest {
			broken = append(broken, BrokenLink{
				URL:        linkURL,
				StatusCode: statusCode,
			})
		}
	}

	return broken
}

func probeURL(ctx context.Context, opts Options, target string) (int, error) {
	statusCode, err := doProbe(ctx, opts, target, http.MethodHead)
	if err != nil {
		return 0, err
	}
	if statusCode == http.StatusMethodNotAllowed || statusCode == http.StatusNotImplemented {
		return doProbe(ctx, opts, target, http.MethodGet)
	}
	return statusCode, nil
}

func doProbe(ctx context.Context, opts Options, target, method string) (int, error) {
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(ctx, method, target, nil)
	if err != nil {
		return 0, err
	}
	if opts.UserAgent != "" {
		req.Header.Set("User-Agent", opts.UserAgent)
	}

	resp, err := opts.HTTPClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	return resp.StatusCode, nil
}
