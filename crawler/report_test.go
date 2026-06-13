package crawler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestReportMatchesGoldenShape(t *testing.T) {
	fixedTime := time.Date(2024, 6, 1, 12, 34, 56, 0, time.UTC)
	originalReportTime := reportTime
	reportTime = func() time.Time { return fixedTime }
	t.Cleanup(func() { reportTime = originalReportTime })

	logoBody := bytes.Repeat([]byte("x"), 12345)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!DOCTYPE html><html><head>
			<title>Example title</title>
			<meta name="description" content="Example description">
		</head><body>
			<h1>Example h1</h1>
			<a href="/missing">missing</a>
			<img src="/static/logo.png">
		</body></html>`)
	})
	mux.HandleFunc("/missing", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/static/logo.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(logoBody)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(logoBody)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	data, err := Analyze(context.Background(), Options{
		URL:        server.URL + "/",
		Depth:      1,
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	want := Report{
		RootURL:     server.URL + "/",
		Depth:       1,
		GeneratedAt: fixedTime.Format(time.RFC3339),
		Pages: []PageReport{
			{
				URL:        server.URL + "/",
				Depth:      0,
				HTTPStatus: http.StatusOK,
				Status:     "ok",
				Error:      "",
				SEO: SEOReport{
					HasTitle:       true,
					Title:          "Example title",
					HasDescription: true,
					Description:    "Example description",
					HasH1:          true,
				},
				BrokenLinks: []BrokenLink{
					{
						URL:        server.URL + "/missing",
						StatusCode: http.StatusNotFound,
						Error:      http.StatusText(http.StatusNotFound),
					},
				},
				Assets: []AssetReport{
					{
						URL:        server.URL + "/static/logo.png",
						Type:       "image",
						StatusCode: http.StatusOK,
						SizeBytes:  int64(len(logoBody)),
						Error:      "",
					},
				},
				DiscoveredAt: fixedTime.Format(time.RFC3339),
			},
		},
	}

	var got Report
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	wantJSON, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal want: %v", err)
	}
	gotJSON, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal got: %v", err)
	}
	if !bytes.Equal(wantJSON, gotJSON) {
		t.Fatalf("report mismatch:\nwant %s\ngot  %s", wantJSON, gotJSON)
	}
}

func TestIndentJSONSameContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!DOCTYPE html><html><head><title>t</title></head><body></body></html>`)
	}))
	defer server.Close()

	base := Options{
		URL:        server.URL,
		Depth:      1,
		HTTPClient: server.Client(),
	}

	compact, err := Analyze(context.Background(), base)
	if err != nil {
		t.Fatalf("Analyze compact: %v", err)
	}

	indented, err := Analyze(context.Background(), Options{
		URL:         base.URL,
		Depth:       base.Depth,
		HTTPClient:  base.HTTPClient,
		IndentJSON:  true,
	})
	if err != nil {
		t.Fatalf("Analyze indent: %v", err)
	}

	if bytes.Equal(compact, indented) {
		t.Fatal("expected different formatting for indent-json")
	}

	var gotCompact Report
	var gotIndent Report
	if err := json.Unmarshal(compact, &gotCompact); err != nil {
		t.Fatalf("unmarshal compact: %v", err)
	}
	if err := json.Unmarshal(indented, &gotIndent); err != nil {
		t.Fatalf("unmarshal indent: %v", err)
	}

	compactAgain, err := json.Marshal(gotCompact)
	if err != nil {
		t.Fatal(err)
	}
	indentAgain, err := json.Marshal(gotIndent)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(compactAgain, indentAgain) {
		t.Fatalf("indent-json changed report content")
	}
}
