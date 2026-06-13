package crawler

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/net/html"
)

type AssetReport struct {
	URL        string `json:"url"`
	Type       string `json:"type"`
	StatusCode int    `json:"status_code"`
	SizeBytes  int64  `json:"size_bytes"`
	Error      string `json:"error"`
}

type pageAssetRef struct {
	url  string
	typ  string
}

type assetCache struct {
	mu    sync.Mutex
	opts  Options
	rl    *rateLimiter
	items map[string]AssetReport
}

func newAssetCache(opts Options, rl *rateLimiter) *assetCache {
	return &assetCache{
		opts:  opts,
		rl:    rl,
		items: make(map[string]AssetReport),
	}
}

func (c *assetCache) get(ctx context.Context, assetURL, assetType string) AssetReport {
	c.mu.Lock()
	if item, ok := c.items[assetURL]; ok {
		c.mu.Unlock()
		return item
	}
	c.mu.Unlock()

	item := fetchAsset(ctx, c.opts, c.rl, assetURL, assetType)

	c.mu.Lock()
	c.items[assetURL] = item
	c.mu.Unlock()

	return item
}

func collectPageAssets(ctx context.Context, cache *assetCache, pageURL string, body []byte) []AssetReport {
	refs, err := extractAssets(pageURL, body)
	if err != nil {
		return []AssetReport{}
	}

	assets := make([]AssetReport, 0, len(refs))
	for _, ref := range refs {
		assets = append(assets, cache.get(ctx, ref.url, ref.typ))
	}
	return assets
}

func extractAssets(pageURL string, body []byte) ([]pageAssetRef, error) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	var refs []pageAssetRef

	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "img":
				if ref, ok := assetRefFromAttr(pageURL, n, "src", "image"); ok {
					if _, exists := seen[ref.url]; !exists {
						seen[ref.url] = struct{}{}
						refs = append(refs, ref)
					}
				}
			case "script":
				if ref, ok := assetRefFromAttr(pageURL, n, "src", "script"); ok {
					if _, exists := seen[ref.url]; !exists {
						seen[ref.url] = struct{}{}
						refs = append(refs, ref)
					}
				}
			case "link":
				if isStylesheetLink(n) {
					if ref, ok := assetRefFromAttr(pageURL, n, "href", "style"); ok {
						if _, exists := seen[ref.url]; !exists {
							seen[ref.url] = struct{}{}
							refs = append(refs, ref)
						}
					}
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(doc)

	return refs, nil
}

func assetRefFromAttr(pageURL string, n *html.Node, attrName, assetType string) (pageAssetRef, bool) {
	for _, attr := range n.Attr {
		if attr.Key != attrName {
			continue
		}
		abs, ok := normalizeLink(pageURL, attr.Val)
		if !ok {
			return pageAssetRef{}, false
		}
		return pageAssetRef{url: abs, typ: assetType}, true
	}
	return pageAssetRef{}, false
}

func isStylesheetLink(n *html.Node) bool {
	for _, attr := range n.Attr {
		if attr.Key == "rel" && strings.EqualFold(strings.TrimSpace(attr.Val), "stylesheet") {
			return true
		}
	}
	return false
}

func fetchAsset(ctx context.Context, opts Options, rl *rateLimiter, assetURL, assetType string) AssetReport {
	report := AssetReport{
		URL:  assetURL,
		Type: assetType,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, assetURL, nil)
	if err != nil {
		report.Error = err.Error()
		return report
	}
	if opts.UserAgent != "" {
		req.Header.Set("User-Agent", opts.UserAgent)
	}

	resp, err := doHTTPWithRetry(ctx, opts, rl, req)
	if err != nil {
		report.Error = err.Error()
		return report
	}
	defer resp.Body.Close()

	report.StatusCode = resp.StatusCode

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		report.Error = err.Error()
		return report
	}

	if resp.StatusCode >= http.StatusBadRequest {
		report.Error = http.StatusText(resp.StatusCode)
		return report
	}

	size, sizeErr := assetSizeBytes(resp, body)
	report.SizeBytes = size
	if sizeErr != nil {
		report.Error = sizeErr.Error()
	}

	return report
}

func assetSizeBytes(resp *http.Response, body []byte) (int64, error) {
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		size, err := strconv.ParseInt(cl, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid Content-Length: %s", cl)
		}
		if size < 0 {
			return 0, fmt.Errorf("invalid Content-Length: %s", cl)
		}
		return size, nil
	}

	if body == nil {
		return 0, fmt.Errorf("response body is unavailable")
	}

	return int64(len(body)), nil
}
