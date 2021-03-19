// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package system

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/Showmax/go-fqdn"
	"github.com/docker/docker/client"
)

type systemMetadata interface {
	// Hostname returns the hostname or fully qualified domain name
	Hostname(context.Context) (string, error)

	// OSType returns the host operating system
	OSType(context.Context) (string, error)
}

func newSystemMetadata() systemMetadata {
	return &systemMetadataImpl{}
}

type systemMetadataImpl struct{}

func (*systemMetadataImpl) Hostname(context.Context) (string, error) {
	return fqdn.FqdnHostname()
}

func (*systemMetadataImpl) OSType(context.Context) (string, error) {
	return strings.ToUpper(runtime.GOOS), nil
}

type dockerMetadataImpl struct {
	dockerClient *client.Client
}

func newDockerMetadata() (systemMetadata, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("could not initialize Docker client: %w", err)
	}
	return &dockerMetadataImpl{dockerClient: cli}, nil
}

func (d *dockerMetadataImpl) Hostname(ctx context.Context) (string, error) {
	info, err := d.dockerClient.Info(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to fetch Docker information: %w", err)
	}
	return info.Name, nil
}

func (d *dockerMetadataImpl) OSType(ctx context.Context) (string, error) {
	info, err := d.dockerClient.Info(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to fetch Docker OS type: %w", err)
	}
	return info.OSType, nil
}
