package crawler

import (
	"net/http"
	"testing"
)

func TestAssetSizeBytesUsesContentLength(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{"Content-Length": []string{"42"}},
	}
	size, err := assetSizeBytes(resp, []byte("ignored"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if size != 42 {
		t.Fatalf("size = %d, want 42", size)
	}
}

func TestAssetSizeBytesUsesBodyWithoutHeader(t *testing.T) {
	body := []byte("hello-asset")
	resp := &http.Response{Header: http.Header{}}

	size, err := assetSizeBytes(resp, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if size != int64(len(body)) {
		t.Fatalf("size = %d, want %d", size, len(body))
	}
}
