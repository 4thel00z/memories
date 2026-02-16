package internal

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

const (
	DefaultModelURL      = "https://huggingface.co/nomic-ai/nomic-embed-text-v1.5-GGUF/resolve/main/nomic-embed-text-v1.5.Q4_K_M.gguf"
	DefaultModelFilename = "nomic-embed-text-v1.5.Q4_K_M.gguf"
	DefaultModelSize     = 85 * 1024 * 1024
)

type ProgressWriter struct {
	Total      int64
	Written    int64
	OnProgress func(written, total int64)
}

func (pw *ProgressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.Written += int64(n)
	if pw.OnProgress != nil {
		pw.OnProgress(pw.Written, pw.Total)
	}
	return n, nil
}

type Downloader struct {
	cacheDir string
	token    string
	client   *http.Client
}

func NewDownloader(cacheDir, token string) *Downloader {
	return &Downloader{
		cacheDir: cacheDir,
		token:    token,
		client:   http.DefaultClient,
	}
}

func (d *Downloader) EnsureModel(ctx context.Context, url, filename string, onProgress func(written, total int64)) (string, error) {
	modelPath := filepath.Join(d.cacheDir, filename)

	if _, err := os.Stat(modelPath); err == nil {
		return modelPath, nil
	}

	if err := os.MkdirAll(d.cacheDir, 0755); err != nil {
		return "", fmt.Errorf("create cache dir: %w", err)
	}

	if err := d.download(ctx, url, modelPath, onProgress); err != nil {
		return "", err
	}

	return modelPath, nil
}

func (d *Downloader) download(ctx context.Context, url, dest string, onProgress func(written, total int64)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	if d.token != "" {
		req.Header.Set("Authorization", "Bearer "+d.token)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: status %d", resp.StatusCode)
	}

	tmpFile := dest + ".tmp"
	f, err := os.Create(tmpFile)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	pw := &ProgressWriter{
		Total:      resp.ContentLength,
		OnProgress: onProgress,
	}

	_, err = io.Copy(f, io.TeeReader(resp.Body, pw))
	closeErr := f.Close()

	if err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("write file: %w", err)
	}
	if closeErr != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("close file: %w", closeErr)
	}

	if err := os.Rename(tmpFile, dest); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("rename file: %w", err)
	}

	return nil
}

func DefaultCacheDir() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, "mem", "models"), nil
}
