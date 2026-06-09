package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type BrokenLink struct {
	URL        string `json:"url"`
	StatusCode int    `json:"status_code,omitempty"`
	Error      string `json:"error,omitempty"`
}

type PageReport struct {
	URL          string       `json:"url"`
	Depth        int          `json:"depth"`
	HTTPStatus   int          `json:"http_status"`
	Status       string       `json:"status"`
	Error        string       `json:"error"`
	BrokenLinks  []BrokenLink `json:"broken_links,omitempty"`
	DiscoveredAt string       `json:"discovered_at,omitempty"`
	SEO          SEOReport    `json:"seo"`
}

type Report struct {
	RootURL     string       `json:"root_url"`
	Depth       int          `json:"depth"`
	GeneratedAt string       `json:"generated_at"`
	Pages       []PageReport `json:"pages"`
}

type crawlItem struct {
	url   string
	depth int
}

func Analyze(ctx context.Context, opts Options) ([]byte, error) {
	pages := crawl(ctx, opts)
	return marshalReport(opts, pages)
}

func crawl(ctx context.Context, opts Options) []PageReport {
	rl := newRateLimiter(opts, nil, nil)
	visited := map[string]struct{}{opts.URL: {}}
	queue := []crawlItem{{url: opts.URL, depth: 0}}
	var pages []PageReport

	for len(queue) > 0 {
		if ctx.Err() != nil {
			break
		}

		item := queue[0]
		queue = queue[1:]

		page, links := fetchPage(ctx, opts, rl, item.url, item.depth)
		pages = append(pages, page)

		childDepth := item.depth + 1
		if childDepth >= opts.Depth || page.Status != "ok" {
			continue
		}

		for _, link := range links {
			if !sameDomain(opts.URL, link) {
				continue
			}
			if _, seen := visited[link]; seen {
				continue
			}
			visited[link] = struct{}{}
			queue = append(queue, crawlItem{url: link, depth: childDepth})
		}
	}

	return pages
}

func marshalReport(opts Options, pages []PageReport) ([]byte, error) {
	report := Report{
		RootURL:     opts.URL,
		Depth:       opts.Depth,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Pages:       pages,
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

func fetchPage(ctx context.Context, opts Options, rl *rateLimiter, pageURL string, depth int) (PageReport, []string) {
	page := PageReport{
		URL:   pageURL,
		Depth: depth,
	}

	reqCtx := ctx
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		reqCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, pageURL, nil)
	if err != nil {
		page.Status = "error"
		page.Error = err.Error()
		return page, nil
	}

	if opts.UserAgent != "" {
		req.Header.Set("User-Agent", opts.UserAgent)
	}

	resp, err := doHTTP(reqCtx, opts, rl, req)
	if err != nil {
		page.Status = "error"
		page.Error = err.Error()
		return page, nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		page.Status = "error"
		page.Error = err.Error()
		return page, nil
	}

	links, _ := extractLinks(pageURL, body)

	page.HTTPStatus = resp.StatusCode
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		page.Status = "ok"
		page.DiscoveredAt = time.Now().UTC().Format(time.RFC3339)
		page.SEO = extractSEO(body)
		page.BrokenLinks = checkBrokenLinks(reqCtx, opts, rl, links)
	} else {
		page.Status = "error"
		page.Error = fmt.Sprintf("http status %d", resp.StatusCode)
	}

	return page, links
}
