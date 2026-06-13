package crawler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"code/crawler"
)

type assetReport struct {
	URL        string `json:"url"`
	Type       string `json:"type"`
	StatusCode int    `json:"status_code"`
	SizeBytes  int64  `json:"size_bytes"`
	Error      string `json:"error"`
}

func analyzeAssetsPages(t *testing.T, rootURL string, depth int, client *http.Client) []struct {
	Assets []assetReport `json:"assets"`
} {
	t.Helper()

	data, err := crawler.Analyze(context.Background(), crawler.Options{
		URL:        rootURL,
		Depth:      depth,
		HTTPClient: client,
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	var report struct {
		Pages []struct {
			Assets []assetReport `json:"assets"`
		} `json:"pages"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return report.Pages
}

func TestAssetCacheFetchesOnce(t *testing.T) {
	var assetCalls atomic.Int32
	logoBody := []byte("logo-bytes")

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/":
			fmt.Fprint(w, `<!DOCTYPE html><html><body>
				<img src="/logo.png">
				<a href="/page2">page2</a>
			</body></html>`)
		case "/page2":
			fmt.Fprint(w, `<!DOCTYPE html><html><body>
				<img src="/logo.png">
			</body></html>`)
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("/logo.png", func(w http.ResponseWriter, r *http.Request) {
		assetCalls.Add(1)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(logoBody)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(logoBody)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	pages := analyzeAssetsPages(t, server.URL+"/", 2, server.Client())
	if len(pages) != 2 {
		t.Fatalf("pages count = %d, want 2", len(pages))
	}
	if got := int(assetCalls.Load()); got != 1 {
		t.Fatalf("asset requests = %d, want 1", got)
	}

	wantURL := server.URL + "/logo.png"
	for i, page := range pages {
		if len(page.Assets) != 1 {
			t.Fatalf("page[%d] assets = %d, want 1", i, len(page.Assets))
		}
		asset := page.Assets[0]
		if asset.URL != wantURL {
			t.Errorf("page[%d] url = %q, want %q", i, asset.URL, wantURL)
		}
		if asset.Type != "image" {
			t.Errorf("page[%d] type = %q, want image", i, asset.Type)
		}
		if asset.StatusCode != http.StatusOK {
			t.Errorf("page[%d] status_code = %d, want 200", i, asset.StatusCode)
		}
		if asset.SizeBytes != int64(len(logoBody)) {
			t.Errorf("page[%d] size_bytes = %d, want %d", i, asset.SizeBytes, len(logoBody))
		}
		if asset.Error != "" {
			t.Errorf("page[%d] error = %q, want empty", i, asset.Error)
		}
	}
}

func TestAssetSizeWithoutContentLength(t *testing.T) {
	body := []byte("plain-css-content")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, `<!DOCTYPE html><html><head>
				<link rel="stylesheet" href="/app.css">
			</head><body></body></html>`)
		case "/app.css":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(body)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	pages := analyzeAssetsPages(t, server.URL+"/", 1, server.Client())
	if len(pages) != 1 || len(pages[0].Assets) != 1 {
		t.Fatalf("unexpected pages: %+v", pages)
	}

	asset := pages[0].Assets[0]
	if asset.Type != "style" {
		t.Errorf("type = %q, want style", asset.Type)
	}
	if asset.SizeBytes != int64(len(body)) {
		t.Errorf("size_bytes = %d, want %d", asset.SizeBytes, len(body))
	}
	if asset.Error != "" {
		t.Errorf("error = %q, want empty", asset.Error)
	}
}

func TestAssetErrorStatusIncluded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, `<!DOCTYPE html><html><body>
				<script src="/missing.js"></script>
			</body></html>`)
		case "/missing.js":
			w.WriteHeader(http.StatusNotFound)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	pages := analyzeAssetsPages(t, server.URL+"/", 1, server.Client())
	if len(pages[0].Assets) != 1 {
		t.Fatalf("assets count = %d, want 1", len(pages[0].Assets))
	}

	asset := pages[0].Assets[0]
	if asset.URL != server.URL+"/missing.js" {
		t.Errorf("url = %q", asset.URL)
	}
	if asset.Type != "script" {
		t.Errorf("type = %q, want script", asset.Type)
	}
	if asset.StatusCode != http.StatusNotFound {
		t.Errorf("status_code = %d, want 404", asset.StatusCode)
	}
	if asset.SizeBytes != 0 {
		t.Errorf("size_bytes = %d, want 0", asset.SizeBytes)
	}
	if asset.Error != http.StatusText(http.StatusNotFound) {
		t.Errorf("error = %q, want %q", asset.Error, http.StatusText(http.StatusNotFound))
	}
}
