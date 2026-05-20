// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package docker // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/docker"

import (
	"context"
	"fmt"
	"os"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/internal"
)

type Provider interface {
	// Hostname returns the OS hostname
	Hostname(context.Context) (string, error)

	// OSType returns the host operating system
	OSType(context.Context) (string, error)

	// ContainerInfo returns the current container information
	ContainerInfo(context.Context) (container.InspectResponse, error)
}

type dockerProviderImpl struct {
	dockerClient *client.Client
}

func NewProvider(opts ...client.Opt) (Provider, error) {
	opts = append(opts, client.FromEnv)
	cli, err := client.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("could not initialize Docker client: %w", err)
	}
	return &dockerProviderImpl{dockerClient: cli}, nil
}

func (d *dockerProviderImpl) Hostname(ctx context.Context) (string, error) {
	result, err := d.dockerClient.Info(ctx, client.InfoOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to fetch Docker information: %w", err)
	}
	return result.Info.Name, nil
}

func (d *dockerProviderImpl) OSType(ctx context.Context) (string, error) {
	result, err := d.dockerClient.Info(ctx, client.InfoOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to fetch Docker OS type: %w", err)
	}
	return internal.GOOSToOSType(result.Info.OSType), nil
}

func (d *dockerProviderImpl) ContainerInfo(ctx context.Context) (container.InspectResponse, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return container.InspectResponse{}, err
	}
	result, err := d.dockerClient.ContainerInspect(ctx, hostname, client.ContainerInspectOptions{})
	if err != nil {
		return container.InspectResponse{}, fmt.Errorf("failed to fetch container information: %w", err)
	}
	return result.Container, nil
}
