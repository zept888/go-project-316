package crawler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"code/crawler"
)

type brokenLink struct {
	URL        string `json:"url"`
	StatusCode int    `json:"status_code"`
	Error      string `json:"error"`
}

type seoReport struct {
	HasTitle       bool   `json:"has_title"`
	Title          string `json:"title"`
	HasDescription bool   `json:"has_description"`
	Description    string `json:"description"`
	HasH1          bool   `json:"has_h1"`
}

type pageReport struct {
	URL          string       `json:"url"`
	Depth        int          `json:"depth"`
	HTTPStatus   int          `json:"http_status"`
	Status       string       `json:"status"`
	Error        string       `json:"error"`
	BrokenLinks  []brokenLink `json:"broken_links"`
	DiscoveredAt string       `json:"discovered_at"`
	SEO          seoReport    `json:"seo"`
}

type crawlReport struct {
	RootURL     string       `json:"root_url"`
	GeneratedAt string       `json:"generated_at"`
	Pages       []pageReport `json:"pages"`
}

func analyzeReport(t *testing.T, ctx context.Context, opts crawler.Options) crawlReport {
	t.Helper()

	if ctx == nil {
		ctx = context.Background()
	}

	data, err := crawler.Analyze(ctx, opts)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	var report crawlReport
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if report.RootURL != opts.URL {
		t.Errorf("root_url = %q, want %q", report.RootURL, opts.URL)
	}
	if report.GeneratedAt == "" {
		t.Error("generated_at is empty")
	}

	return report
}

func analyzeFirstPage(t *testing.T, opts crawler.Options) pageReport {
	t.Helper()

	report := analyzeReport(t, context.Background(), opts)
	if len(report.Pages) != 1 {
		t.Fatalf("pages count = %d, want 1", len(report.Pages))
	}

	return report.Pages[0]
}

func pageURLs(pages []pageReport) []string {
	urls := make([]string, len(pages))
	for i, page := range pages {
		urls[i] = page.URL
	}
	return urls
}

func setupDepthTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/":
			fmt.Fprintf(w, `<!DOCTYPE html><html><body>
				<a href="/a">a</a>
				<a href="/b">b</a>
				<a href="/dup">dup</a>
				<a href="/dup">dup again</a>
				<a href="https://external.test/out">external</a>
			</body></html>`)
		case "/a":
			fmt.Fprint(w, `<!DOCTYPE html><html><body><a href="/c">c</a></body></html>`)
		case "/b":
			fmt.Fprint(w, `<!DOCTYPE html><html><body><h1>b</h1></body></html>`)
		case "/c":
			fmt.Fprint(w, `<!DOCTYPE html><html><body><h1>c</h1></body></html>`)
		case "/dup":
			fmt.Fprint(w, `<!DOCTYPE html><html><body><h1>dup</h1></body></html>`)
		default:
			http.NotFound(w, r)
		}
	})

	return httptest.NewServer(mux)
}

func TestAnalyzeHTTPSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	page := analyzeFirstPage(t, crawler.Options{
		URL:        server.URL,
		Depth:      1,
		HTTPClient: server.Client(),
	})

	if page.URL != server.URL {
		t.Errorf("url = %q, want %q", page.URL, server.URL)
	}
	if page.HTTPStatus != http.StatusOK {
		t.Errorf("http_status = %d, want %d", page.HTTPStatus, http.StatusOK)
	}
	if page.Status != "ok" {
		t.Errorf("status = %q, want ok", page.Status)
	}
	if page.Error != "" {
		t.Errorf("error = %q, want empty", page.Error)
	}
}

func TestAnalyzeHTTPInvalidStatus(t *testing.T) {
	tests := []struct {
		name string
		code int
	}{
		{name: "404 Not Found", code: http.StatusNotFound},
		{name: "500 Internal Server Error", code: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.code)
			}))
			defer server.Close()

			page := analyzeFirstPage(t, crawler.Options{
				URL:        server.URL,
				Depth:      1,
				HTTPClient: server.Client(),
			})

			if page.HTTPStatus != tt.code {
				t.Errorf("http_status = %d, want %d", page.HTTPStatus, tt.code)
			}
			if page.Status != "error" {
				t.Errorf("status = %q, want error", page.Status)
			}
			if page.Error == "" {
				t.Error("error is empty")
			}
		})
	}
}

func TestAnalyzeNetworkFailure(t *testing.T) {
	page := analyzeFirstPage(t, crawler.Options{
		URL:        "http://127.0.0.1:1",
		Depth:      1,
		HTTPClient: &http.Client{},
	})

	if page.HTTPStatus != 0 {
		t.Errorf("http_status = %d, want 0", page.HTTPStatus)
	}
	if page.Status != "error" {
		t.Errorf("status = %q, want error", page.Status)
	}
	if page.Error == "" {
		t.Error("error is empty")
	}
}

func TestAnalyzeTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	page := analyzeFirstPage(t, crawler.Options{
		URL:        server.URL,
		Depth:      1,
		Timeout:    50 * time.Millisecond,
		HTTPClient: server.Client(),
	})

	if page.HTTPStatus != 0 {
		t.Errorf("http_status = %d, want 0", page.HTTPStatus)
	}
	if page.Status != "error" {
		t.Errorf("status = %q, want error", page.Status)
	}
	if page.Error == "" {
		t.Error("error is empty")
	}
}

func TestAnalyzeBrokenLinks(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!DOCTYPE html><html><body>
			<a href="/ok">working</a>
			<a href="/broken">broken</a>
			<a href="mailto:noreply@example.com">email</a>
			<a href="#">anchor</a>
			<a href="">empty</a>
		</body></html>`)
	})
	mux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/broken", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	page := analyzeFirstPage(t, crawler.Options{
		URL:        server.URL,
		Depth:      1,
		HTTPClient: server.Client(),
	})

	if page.Status != "ok" {
		t.Fatalf("status = %q, want ok", page.Status)
	}
	if page.DiscoveredAt == "" {
		t.Error("discovered_at is empty")
	}
	if len(page.BrokenLinks) != 1 {
		t.Fatalf("broken_links count = %d, want 1", len(page.BrokenLinks))
	}

	wantBroken := server.URL + "/broken"
	if page.BrokenLinks[0].URL != wantBroken {
		t.Errorf("broken url = %q, want %q", page.BrokenLinks[0].URL, wantBroken)
	}
	if page.BrokenLinks[0].StatusCode != http.StatusNotFound {
		t.Errorf("broken status_code = %d, want %d", page.BrokenLinks[0].StatusCode, http.StatusNotFound)
	}
	if page.BrokenLinks[0].Error != http.StatusText(http.StatusNotFound) {
		t.Errorf("broken error = %q, want %q", page.BrokenLinks[0].Error, http.StatusText(http.StatusNotFound))
	}
}

func TestAnalyzeBrokenLinksNetworkError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!DOCTYPE html><html><body>
			<a href="/ok">working</a>
			<a href="https://cdn.simple.test/app.js">broken external</a>
		</body></html>`)
	})
	mux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	page := analyzeFirstPage(t, crawler.Options{
		URL:        server.URL,
		Depth:      1,
		HTTPClient: server.Client(),
	})

	if len(page.BrokenLinks) != 1 {
		t.Fatalf("broken_links count = %d, want 1", len(page.BrokenLinks))
	}
	if page.BrokenLinks[0].URL != "https://cdn.simple.test/app.js" {
		t.Errorf("broken url = %q, want https://cdn.simple.test/app.js", page.BrokenLinks[0].URL)
	}
	if page.BrokenLinks[0].StatusCode != 0 {
		t.Errorf("broken status_code = %d, want 0", page.BrokenLinks[0].StatusCode)
	}
	if page.BrokenLinks[0].Error == "" {
		t.Error("broken error is empty")
	}
}

func TestAnalyzeSEOAllPresent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!DOCTYPE html><html><head>
			<title>Tom &amp; Jerry</title>
			<meta name="description" content="A &amp; B">
		</head><body><h1>Hello &amp; World</h1></body></html>`)
	}))
	defer server.Close()

	page := analyzeFirstPage(t, crawler.Options{
		URL:        server.URL,
		Depth:      1,
		HTTPClient: server.Client(),
	})

	if !page.SEO.HasTitle {
		t.Error("has_title = false, want true")
	}
	if page.SEO.Title != "Tom & Jerry" {
		t.Errorf("title = %q, want %q", page.SEO.Title, "Tom & Jerry")
	}
	if !page.SEO.HasDescription {
		t.Error("has_description = false, want true")
	}
	if page.SEO.Description != "A & B" {
		t.Errorf("description = %q, want %q", page.SEO.Description, "A & B")
	}
	if !page.SEO.HasH1 {
		t.Error("has_h1 = false, want true")
	}
}

func TestAnalyzeSEOMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!DOCTYPE html><html><body><p>no seo</p></body></html>`)
	}))
	defer server.Close()

	page := analyzeFirstPage(t, crawler.Options{
		URL:        server.URL,
		Depth:      1,
		HTTPClient: server.Client(),
	})

	if page.SEO.HasTitle {
		t.Error("has_title = true, want false")
	}
	if page.SEO.Title != "" {
		t.Errorf("title = %q, want empty", page.SEO.Title)
	}
	if page.SEO.HasDescription {
		t.Error("has_description = true, want false")
	}
	if page.SEO.Description != "" {
		t.Errorf("description = %q, want empty", page.SEO.Description)
	}
	if page.SEO.HasH1 {
		t.Error("has_h1 = true, want false")
	}
}

func TestAnalyzeDepthOneOnlyRoot(t *testing.T) {
	server := setupDepthTestServer(t)
	defer server.Close()

	report := analyzeReport(t, context.Background(), crawler.Options{
		URL:        server.URL,
		Depth:      1,
		HTTPClient: server.Client(),
	})

	if len(report.Pages) != 1 {
		t.Fatalf("pages count = %d, want 1", len(report.Pages))
	}
	if report.Pages[0].Depth != 0 {
		t.Errorf("depth = %d, want 0", report.Pages[0].Depth)
	}
}

func TestAnalyzeDepthTwoIncludesChildren(t *testing.T) {
	server := setupDepthTestServer(t)
	defer server.Close()

	report := analyzeReport(t, context.Background(), crawler.Options{
		URL:        server.URL,
		Depth:      2,
		HTTPClient: server.Client(),
	})

	if len(report.Pages) != 4 {
		t.Fatalf("pages count = %d, want 4", len(report.Pages))
	}

	want := map[string]int{
		server.URL:     0,
		server.URL + "/a":    1,
		server.URL + "/b":    1,
		server.URL + "/dup":  1,
	}
	if len(want) != len(report.Pages) {
		t.Fatalf("unexpected pages: %v", pageURLs(report.Pages))
	}
	for _, page := range report.Pages {
		depth, ok := want[page.URL]
		if !ok {
			t.Errorf("unexpected page url %q", page.URL)
			continue
		}
		if page.Depth != depth {
			t.Errorf("page %q depth = %d, want %d", page.URL, page.Depth, depth)
		}
	}
}

func TestAnalyzeExternalNotInPages(t *testing.T) {
	server := setupDepthTestServer(t)
	defer server.Close()

	report := analyzeReport(t, context.Background(), crawler.Options{
		URL:        server.URL,
		Depth:      2,
		HTTPClient: server.Client(),
	})

	for _, page := range report.Pages {
		if page.URL == "https://external.test/out" {
			t.Fatalf("external url must not be in pages: %v", pageURLs(report.Pages))
		}
	}

	root := report.Pages[0]
	foundExternal := false
	for _, link := range root.BrokenLinks {
		if link.URL == "https://external.test/out" {
			foundExternal = true
			break
		}
	}
	if !foundExternal {
		t.Fatal("external link must be checked as broken link on root page")
	}
}

func TestAnalyzeDuplicateLinksVisitedOnce(t *testing.T) {
	server := setupDepthTestServer(t)
	defer server.Close()

	report := analyzeReport(t, context.Background(), crawler.Options{
		URL:        server.URL,
		Depth:      2,
		HTTPClient: server.Client(),
	})

	dupCount := 0
	for _, page := range report.Pages {
		if page.URL == server.URL+"/dup" {
			dupCount++
		}
	}
	if dupCount != 1 {
		t.Fatalf("/dup page count = %d, want 1", dupCount)
	}
}

func TestAnalyzeContextCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/":
			time.Sleep(50 * time.Millisecond)
			fmt.Fprintf(w, `<!DOCTYPE html><html><body><a href="/slow">slow</a></body></html>`)
		case "/slow":
			time.Sleep(200 * time.Millisecond)
			fmt.Fprint(w, `<!DOCTYPE html><html><body><h1>slow</h1></body></html>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		time.Sleep(80 * time.Millisecond)
		cancel()
		close(done)
	}()

	report := analyzeReport(t, ctx, crawler.Options{
		URL:        server.URL,
		Depth:      2,
		HTTPClient: server.Client(),
	})

	<-done

	if len(report.Pages) == 0 {
		t.Fatal("expected partial report with at least one page")
	}
	if len(report.Pages) > 2 {
		t.Fatalf("pages count = %d, want at most 2", len(report.Pages))
	}
}

func TestAnalyzeWithRateLimitSamePages(t *testing.T) {
	server := setupDepthTestServer(t)
	defer server.Close()

	base := crawler.Options{
		URL:        server.URL,
		Depth:      2,
		HTTPClient: server.Client(),
	}

	unlimited := analyzeReport(t, context.Background(), base)
	delayed := analyzeReport(t, context.Background(), crawler.Options{
		URL:         base.URL,
		Depth:       base.Depth,
		Delay:       time.Millisecond,
		HTTPClient:  base.HTTPClient,
	})
	rpsLimited := analyzeReport(t, context.Background(), crawler.Options{
		URL:         base.URL,
		Depth:       base.Depth,
		RPS:         1000,
		Delay:       time.Second,
		HTTPClient:  base.HTTPClient,
	})

	if len(unlimited.Pages) != 4 {
		t.Fatalf("unlimited pages = %d, want 4", len(unlimited.Pages))
	}
	if len(delayed.Pages) != len(unlimited.Pages) {
		t.Fatalf("delayed pages = %d, want %d", len(delayed.Pages), len(unlimited.Pages))
	}
	if len(rpsLimited.Pages) != len(unlimited.Pages) {
		t.Fatalf("rps pages = %d, want %d", len(rpsLimited.Pages), len(unlimited.Pages))
	}

	for _, page := range delayed.Pages {
		if page.Status != "ok" {
			t.Errorf("delayed page %q status = %q, want ok", page.URL, page.Status)
		}
	}
}

func TestAnalyzeBrokenLinksRetryLastResult(t *testing.T) {
	var calls atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!DOCTYPE html><html><body><a href="/flaky">flaky</a></body></html>`)
	})
	mux.HandleFunc("/flaky", func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	page := analyzeFirstPage(t, crawler.Options{
		URL:        server.URL,
		Depth:      1,
		Retries:    1,
		HTTPClient: server.Client(),
	})

	if len(page.BrokenLinks) != 0 {
		t.Fatalf("broken_links = %v, want empty after successful retry", page.BrokenLinks)
	}
	if got := int(calls.Load()); got != 2 {
		t.Fatalf("probe requests = %d, want 2", got)
	}
}
