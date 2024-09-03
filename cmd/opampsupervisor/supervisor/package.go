package supervisor

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

func downloadFile(ctx context.Context, url, storageDir string) (*os.File, error) {
	tmpDir := filepath.Join(storageDir, "tmp")
	if err := os.MkdirAll(tmpDir, 0750); err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode > 299 || resp.StatusCode < 200 {
		return nil, fmt.Errorf("got non-200 status: %d", resp.StatusCode)
	}

	f, err := os.CreateTemp(storageDir, "package-download-*")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}

	_, err = io.Copy(f, resp.Body)
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("copy package to temp file: %w", err)
	}

	return f, nil
}

func verifyFileSignature(f *os.File, keys [][]byte) error {
	return nil
}

func verifyFilePath(f *os.File) error {
	return nil
}
