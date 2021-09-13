// Copyright 2020 OpenTelemetry Authors
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

// +build !windows

package podmanreceiver

import (
	"context"

	"github.com/containers/podman/v3/libpod/define"
	"github.com/containers/podman/v3/pkg/bindings"
	"github.com/containers/podman/v3/pkg/bindings/containers"
)

type client interface {
	stats() ([]define.ContainerStats, error)
}

type podmanClient struct {
	conn context.Context
}

func newPodmanClient(endpoint string) (client, error) {
	conn, err := bindings.NewConnection(context.Background(), endpoint)
	if err != nil {
		return nil, err
	}
	return &podmanClient{conn: conn}, nil
}

func (c *podmanClient) stats() ([]define.ContainerStats, error) {
	stream := false
	ch, err := containers.Stats(c.conn, []string{}, &containers.StatsOptions{Stream: &stream})
	if err != nil {
		return nil, err
	}
	report := <-ch
	return report.Stats, report.Error
}
