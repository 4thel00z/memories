package internal

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// TestDownloadRejectsTruncated verifies that a response whose body is shorter
// than the advertised Content-Length is treated as a failed download and the
// partial file is not left behind.
func TestDownloadRejectsTruncated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Claim 100 bytes but send only 10.
		w.Header().Set("Content-Length", strconv.Itoa(100))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("0123456789"))
	}))
	defer srv.Close()

	cacheDir := t.TempDir()
	d := NewDownloader(cacheDir, "")

	_, err := d.EnsureModel(context.Background(), srv.URL, "model.gguf", nil)
	if err == nil {
		t.Fatal("expected truncated download to error, got nil")
	}

	dest := filepath.Join(cacheDir, "model.gguf")
	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Errorf("partial download should not be left in place: %v", statErr)
	}
	if _, statErr := os.Stat(dest + ".tmp"); !os.IsNotExist(statErr) {
		t.Errorf("temp file should be cleaned up: %v", statErr)
	}
}

// TestDownloadCompleteSucceeds verifies a fully-delivered body is accepted.
func TestDownloadCompleteSucceeds(t *testing.T) {
	body := []byte("a complete model file")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	cacheDir := t.TempDir()
	d := NewDownloader(cacheDir, "")

	path, err := d.EnsureModel(context.Background(), srv.URL, "model.gguf", nil)
	if err != nil {
		t.Fatalf("complete download: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}
	if string(got) != string(body) {
		t.Errorf("content mismatch: got %q", got)
	}
}
