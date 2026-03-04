// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package docker // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/docker"

import (
	"context"
	"fmt"

	"github.com/docker/docker/client"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/internal"
)

type Provider interface {
	// Hostname returns the OS hostname
	Hostname(context.Context) (string, error)

	// OSType returns the host operating system
	OSType(context.Context) (string, error)
}

type dockerProviderImpl struct {
	dockerClient *client.Client
}

func NewProvider(opts ...client.Opt) (Provider, error) {
	opts = append(opts, client.FromEnv, client.WithAPIVersionNegotiation())
	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("could not initialize Docker client: %w", err)
	}
	return &dockerProviderImpl{dockerClient: cli}, nil
}

func (d *dockerProviderImpl) Hostname(ctx context.Context) (string, error) {
	info, err := d.dockerClient.Info(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to fetch Docker information: %w", err)
	}
	return info.Name, nil
}

func (d *dockerProviderImpl) OSType(ctx context.Context) (string, error) {
	info, err := d.dockerClient.Info(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to fetch Docker OS type: %w", err)
	}
	return internal.GOOSToOSType(info.OSType), nil
}
