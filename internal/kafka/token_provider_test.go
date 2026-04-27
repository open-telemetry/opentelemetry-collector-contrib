// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafka

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/oauth2"
)

func TestFileTokenProvider(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "token")
	if err := os.WriteFile(path, []byte("  bearer-token\n"), 0o600); err != nil {
		t.Fatalf("write token file: %v", err)
	}

	provider, err := NewFileTokenProvider(path)
	if err != nil {
		t.Fatalf("NewFileTokenProvider failed: %v", err)
	}

	token, err := provider.Token(t.Context())
	if err != nil {
		t.Fatalf("Token returned error: %v", err)
	}
	if token != "bearer-token" {
		t.Fatalf("unexpected token %q", token)
	}

	// Update token and ensure we read the new value on subsequent calls.
	if writeErr := os.WriteFile(path, []byte("next-token"), 0o600); writeErr != nil {
		t.Fatalf("rewrite token file: %v", writeErr)
	}

	token, err = provider.Token(t.Context())
	if err != nil {
		t.Fatalf("Token returned error after update: %v", err)
	}
	if token != "next-token" {
		t.Fatalf("expected updated token, got %q", token)
	}
}

func TestFileTokenProviderErrors(t *testing.T) {
	provider, err := NewFileTokenProvider("/non-existent/file")
	if err != nil {
		t.Fatalf("NewFileTokenProvider unexpected error: %v", err)
	}
	if _, tokenErr := provider.Token(t.Context()); tokenErr == nil {
		t.Fatal("expected error for missing file")
	}

	_, err = NewFileTokenProvider("")
	if err == nil {
		t.Fatal("expected error for empty path")
	}

	dir := t.TempDir()
	emptyPath := filepath.Join(dir, "empty")
	if writeErr := os.WriteFile(emptyPath, []byte(" \n\t"), 0o600); writeErr != nil {
		t.Fatalf("write empty token file: %v", writeErr)
	}

	provider, err = NewFileTokenProvider(emptyPath)
	if err != nil {
		t.Fatalf("NewFileTokenProvider unexpected error for empty token file: %v", err)
	}
	if _, err := provider.Token(t.Context()); !errors.Is(err, ErrEmptyToken) {
		t.Fatalf("expected ErrEmptyToken, got %v", err)
	}
}

func TestTokenSourceProvider(t *testing.T) {
	provider, err := NewTokenSourceProvider(oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "from-source"}))
	if err != nil {
		t.Fatalf("NewTokenSourceProvider failed: %v", err)
	}
	token, err := provider.Token(t.Context())
	if err != nil {
		t.Fatalf("Token returned error: %v", err)
	}
	if token != "from-source" {
		t.Fatalf("unexpected token %q", token)
	}

	if _, nilErr := NewTokenSourceProvider(nil); nilErr == nil {
		t.Fatal("expected error for nil token source")
	}

	emptyProvider, err := NewTokenSourceProvider(oauth2.StaticTokenSource(&oauth2.Token{}))
	if err != nil {
		t.Fatalf("NewTokenSourceProvider unexpected error: %v", err)
	}
	if _, err := emptyProvider.Token(t.Context()); !errors.Is(err, ErrEmptyToken) {
		t.Fatalf("expected ErrEmptyToken, got %v", err)
	}
}
