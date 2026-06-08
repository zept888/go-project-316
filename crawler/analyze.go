package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type PageReport struct {
	URL        string `json:"url"`
	Depth      int    `json:"depth"`
	HTTPStatus int    `json:"http_status"`
	Status     string `json:"status"`
	Error      string `json:"error"`
}

type Report struct {
	RootURL     string       `json:"root_url"`
	Depth       int          `json:"depth"`
	GeneratedAt string       `json:"generated_at"`
	Pages       []PageReport `json:"pages"`
}

func Analyze(ctx context.Context, opts Options) ([]byte, error) {
	page := fetchPage(ctx, opts, opts.URL, 0)

	report := Report{
		RootURL:     opts.URL,
		Depth:       opts.Depth,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Pages:       []PageReport{page},
	}

	var data []byte
	var err error
	if opts.IndentJSON {
		data, err = json.MarshalIndent(report, "", "  ")
	} else {
		data, err = json.Marshal(report)
	}
	if err != nil {
		return nil, fmt.Errorf("marshal report: %w", err)
	}

	return data, nil
}

func fetchPage(ctx context.Context, opts Options, pageURL string, depth int) PageReport {
	page := PageReport{
		URL:   pageURL,
		Depth: depth,
	}

	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		page.Status = "error"
		page.Error = err.Error()
		return page
	}

	if opts.UserAgent != "" {
		req.Header.Set("User-Agent", opts.UserAgent)
	}

	resp, err := opts.HTTPClient.Do(req)
	if err != nil {
		page.Status = "error"
		page.Error = err.Error()
		return page
	}
	defer resp.Body.Close()

	page.HTTPStatus = resp.StatusCode
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		page.Status = "ok"
	} else {
		page.Status = "error"
		page.Error = fmt.Sprintf("http status %d", resp.StatusCode)
	}
	return page
}
