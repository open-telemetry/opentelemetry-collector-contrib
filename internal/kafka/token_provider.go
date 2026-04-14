// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafka // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka"

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/oauth2"
)

// TokenProvider exposes a mechanism to retrieve the OAuth bearer token used when
// authenticating with Kafka.
type TokenProvider interface {
	Token(context.Context) (string, error)
}

// ErrEmptyToken indicates that the configured token source returned an empty value.
var ErrEmptyToken = errors.New("oauth bearer token is empty")

// NewFileTokenProvider returns a TokenProvider that reads the bearer token from
// the supplied file path. The file is read on every Token call, allowing clients
// to pick up refreshed credentials without restarting the collector.
func NewFileTokenProvider(path string) (TokenProvider, error) {
	if path == "" {
		return nil, errors.New("token file path must be provided")
	}
	return &fileTokenProvider{path: path}, nil
}

// NewTokenSourceProvider returns a TokenProvider that uses the supplied oauth2.TokenSource.
func NewTokenSourceProvider(tokenSource oauth2.TokenSource) (TokenProvider, error) {
	if tokenSource == nil {
		return nil, errors.New("token source must be provided")
	}
	return &tokenSourceWrapper{tokenSource: tokenSource}, nil
}

type fileTokenProvider struct {
	path string
}

func (p *fileTokenProvider) Token(ctx context.Context) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	raw, err := os.ReadFile(p.path)
	if err != nil {
		return "", fmt.Errorf("read token file: %w", err)
	}
	token := strings.TrimSpace(string(raw))
	if token == "" {
		return "", ErrEmptyToken
	}
	return token, nil
}

type tokenSourceWrapper struct {
	tokenSource oauth2.TokenSource
}

func (p *tokenSourceWrapper) Token(ctx context.Context) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}
	tok, err := p.tokenSource.Token()
	if err != nil {
		return "", err
	}
	if tok == nil || tok.AccessToken == "" {
		return "", ErrEmptyToken
	}
	return tok.AccessToken, nil
}
