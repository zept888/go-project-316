package crawler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"code/crawler"
)

type brokenLink struct {
	URL        string `json:"url"`
	StatusCode int    `json:"status_code"`
	Error      string `json:"error"`
}

type pageReport struct {
	URL          string       `json:"url"`
	Depth        int          `json:"depth"`
	HTTPStatus   int          `json:"http_status"`
	Status       string       `json:"status"`
	Error        string       `json:"error"`
	BrokenLinks  []brokenLink `json:"broken_links"`
	DiscoveredAt string       `json:"discovered_at"`
}

func analyzeFirstPage(t *testing.T, opts crawler.Options) pageReport {
	t.Helper()

	data, err := crawler.Analyze(context.Background(), opts)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	var report struct {
		RootURL     string       `json:"root_url"`
		GeneratedAt string       `json:"generated_at"`
		Pages       []pageReport `json:"pages"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if report.RootURL != opts.URL {
		t.Errorf("root_url = %q, want %q", report.RootURL, opts.URL)
	}
	if report.GeneratedAt == "" {
		t.Error("generated_at is empty")
	}
	if len(report.Pages) != 1 {
		t.Fatalf("pages count = %d, want 1", len(report.Pages))
	}

	return report.Pages[0]
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
		URL:        server.URL + "/",
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
	if page.BrokenLinks[0].Error != "" {
		t.Errorf("broken error = %q, want empty", page.BrokenLinks[0].Error)
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
		URL:        server.URL + "/",
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
