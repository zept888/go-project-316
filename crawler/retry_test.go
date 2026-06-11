package crawler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetryFailsAfterExhaustedAttempts(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	page := fetchPageForTest(t, server, 2)

	if page.Status != "error" {
		t.Fatalf("status = %q, want error", page.Status)
	}
	if page.HTTPStatus != http.StatusServiceUnavailable {
		t.Fatalf("http_status = %d, want 503", page.HTTPStatus)
	}
	if got := int(calls.Load()); got != 3 {
		t.Fatalf("requests = %d, want 3", got)
	}
}

func TestRetrySucceedsOnSecondAttempt(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	page := fetchPageForTest(t, server, 2)

	if page.Status != "ok" {
		t.Fatalf("status = %q, want ok", page.Status)
	}
	if page.HTTPStatus != http.StatusOK {
		t.Fatalf("http_status = %d, want 200", page.HTTPStatus)
	}
	if got := int(calls.Load()); got != 2 {
		t.Fatalf("requests = %d, want 2", got)
	}
}

func TestRetryDoesNotRetryPermanentErrors(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	page := fetchPageForTest(t, server, 2)

	if page.HTTPStatus != http.StatusNotFound {
		t.Fatalf("http_status = %d, want 404", page.HTTPStatus)
	}
	if got := int(calls.Load()); got != 1 {
		t.Fatalf("requests = %d, want 1", got)
	}
}

func TestRetryContextCancelStopsBackoff(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	block := make(chan struct{})

	sleep := func(ctx context.Context, d time.Duration) error {
		cancel()
		close(block)
		return ctx.Err()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = doHTTPWithRetryAndSleep(ctx, Options{
		Retries:    2,
		HTTPClient: server.Client(),
	}, nil, req, sleep)

	<-block

	if err == nil {
		t.Fatal("expected context error")
	}
	if got := int(calls.Load()); got != 1 {
		t.Fatalf("requests = %d, want 1", got)
	}
}

func fetchPageForTest(t *testing.T, server *httptest.Server, retries int) PageReport {
	t.Helper()

	page, _ := fetchPage(context.Background(), Options{
		Retries:    retries,
		HTTPClient: server.Client(),
	}, nil, newAssetCache(Options{HTTPClient: server.Client()}, nil), server.URL, 0)
	return page
}
