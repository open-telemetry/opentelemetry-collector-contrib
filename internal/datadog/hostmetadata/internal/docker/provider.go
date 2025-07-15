// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package docker contains the Docker hostname provider
package docker // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/hostmetadata/internal/docker"

import (
	"context"
	"errors"
	"fmt"

	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes/source"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/docker"
)

var _ source.Provider = (*Provider)(nil)

type Provider struct {
	logger   *zap.Logger
	detector docker.Provider
}

// Source returns the hostname from Docker daemon if available.
func (p *Provider) Source(ctx context.Context) (source.Source, error) {
	hostname, err := p.detector.Hostname(ctx)
	if err != nil {
		return source.Source{}, fmt.Errorf("failed to get Docker hostname: %w", err)
	}

	if hostname == "" {
		//lint:ignore ST1005, "Docker" is a proper noun
		return source.Source{}, errors.New("Docker hostname is empty")
	}

	return source.Source{Kind: source.HostnameKind, Identifier: hostname}, nil
}

// NewProvider creates a new Docker hostname provider.
func NewProvider(logger *zap.Logger) (*Provider, error) {
	detector, err := docker.NewProvider()
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker detector: %w", err)
	}

	return &Provider{
		logger:   logger,
		detector: detector,
	}, nil
}
