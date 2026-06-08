package crawler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"code/crawler"
)

type pageReport struct {
	URL        string `json:"url"`
	Depth      int    `json:"depth"`
	HTTPStatus int    `json:"http_status"`
	Status     string `json:"status"`
	Error      string `json:"error"`
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
