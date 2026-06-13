package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCLIOutputsOnlyJSONWithTrailingNewline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><head><title>t</title></head><body></body></html>`))
	}))
	defer server.Close()

	binName := "hexlet-go-crawler"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	bin := filepath.Join(t.TempDir(), binName)
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	run := exec.Command(bin, server.URL)
	output, err := run.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("run: %v\n%s", err, exitErr.Stderr)
		}
		t.Fatalf("run: %v", err)
	}

	if len(output) == 0 || output[len(output)-1] != '\n' {
		t.Fatalf("output must end with newline, got %q", output)
	}

	trimmed := bytes.TrimSuffix(output, []byte("\n"))
	if len(trimmed) == 0 || trimmed[0] != '{' || trimmed[len(trimmed)-1] != '}' {
		t.Fatalf("output must be JSON object, got %q", trimmed)
	}

	var report map[string]any
	if err := json.Unmarshal(trimmed, &report); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if strings.TrimSpace(string(output)) != strings.TrimSpace(string(trimmed)) {
		t.Fatalf("output contains extra text outside json")
	}
}
