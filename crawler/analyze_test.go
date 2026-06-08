package crawler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"code/crawler"
)

func TestAnalyzeSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	opts := crawler.Options{
		URL:        server.URL,
		Depth:      1,
		HTTPClient: server.Client(),
	}

	data, err := crawler.Analyze(context.Background(), opts)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	var report struct {
		RootURL     string `json:"root_url"`
		GeneratedAt string `json:"generated_at"`
		Pages       []struct {
			URL        string `json:"url"`
			HTTPStatus int    `json:"http_status"`
			Status     string `json:"status"`
		} `json:"pages"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if report.RootURL != server.URL {
		t.Errorf("root_url = %q, want %q", report.RootURL, server.URL)
	}
	if report.GeneratedAt == "" {
		t.Error("generated_at is empty")
	}
	if len(report.Pages) != 1 {
		t.Fatalf("pages count = %d, want 1", len(report.Pages))
	}
	if report.Pages[0].URL != server.URL {
		t.Errorf("pages[0].url = %q, want %q", report.Pages[0].URL, server.URL)
	}
	if report.Pages[0].HTTPStatus != http.StatusOK {
		t.Errorf("pages[0].http_status = %d, want %d", report.Pages[0].HTTPStatus, http.StatusOK)
	}
	if report.Pages[0].Status != "ok" {
		t.Errorf("pages[0].status = %q, want ok", report.Pages[0].Status)
	}
}

func TestAnalyzeNetworkError(t *testing.T) {
	opts := crawler.Options{
		URL:        "http://127.0.0.1:1",
		Depth:      1,
		HTTPClient: &http.Client{},
	}

	data, err := crawler.Analyze(context.Background(), opts)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	var report struct {
		Pages []struct {
			Status string `json:"status"`
			Error  string `json:"error"`
		} `json:"pages"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(report.Pages) != 1 {
		t.Fatalf("pages count = %d, want 1", len(report.Pages))
	}
	if report.Pages[0].Status != "error" {
		t.Errorf("pages[0].status = %q, want error", report.Pages[0].Status)
	}
	if report.Pages[0].Error == "" {
		t.Error("pages[0].error is empty")
	}
}
