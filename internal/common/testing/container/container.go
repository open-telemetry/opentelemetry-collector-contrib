// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package container

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/require"
)

// ID identifies a container instance.
type ID string

// Containers holds the set of running containers during the test.
type Containers struct {
	cli               client.CommonAPIClient
	runningContainers map[ID]Container
	t                 *testing.T
}

// Container is a running container instance.
type Container struct {
	ID           ID
	waitForPorts []uint16
	ports        nat.PortMap
	t            *testing.T
}

// AddrForPort returns the endpoint with host:port for the container port.
func (c *Container) AddrForPort(port uint16) string {
	for dockerPort, bindings := range c.ports {
		if dockerPort.Int() == int(port) && len(bindings) > 0 {
			return net.JoinHostPort(bindings[0].HostIP, bindings[0].HostPort)
		}
	}

	c.t.Fatalf("unable to find container port %d", port)
	return ""
}

// Option is passed when starting a container to control the start options.
type Option func(c *Container)

// WithPortReady declares that the provided container port should be able to be connected
// to before returning from StartImage.
func WithPortReady(port uint16) Option {
	return func(c *Container) {
		c.waitForPorts = append(c.waitForPorts, port)
	}
}

// New wraps the provided testing.T instance so that any containers started will be cleaned
// up when the test completes.
func New(t *testing.T) *Containers {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("failed creating docker client from environment: %v", err)
	}

	ctx := &Containers{cli: cli, runningContainers: make(map[ID]Container), t: t}

	t.Cleanup(func() {
		ctx.Cleanup()
	})

	return ctx
}

// Cleanup stops and removes all containers. It is called automatically when the test completes
// if called from New() but can also be called manually if needed. It is idempotent.
func (c *Containers) Cleanup() {
	for _, con := range c.runningContainers {
		c.RemoveContainer(con)
	}
	if err := c.cli.Close(); err != nil {
		c.t.Logf("failed closing Docker connection: %v", err)
	}
}

func (c *Containers) pullImage(image string) {
	// Check if image is already present to avoid having to hit external registry each time.
	if _, _, err := c.cli.ImageInspectWithRaw(context.Background(), image); err != nil {
		if client.IsErrNotFound(err) {
			reader, pullErr := c.cli.ImagePull(context.Background(), image, types.ImagePullOptions{})
			if pullErr != nil {
				c.t.Fatalf("failed pulling image %v: %v", image, pullErr)
			}
			for scan := bufio.NewScanner(reader); scan.Scan(); {
				c.t.Logf("image pull: %s", scan.Text())
			}
		} else {
			c.t.Fatalf("failed checking image status: %v", err)
		}
	}
}

func (c *Containers) createContainer(image string, env []string) container.ContainerCreateCreatedBody {
	created, err := c.cli.ContainerCreate(context.Background(), &container.Config{
		Image:  image,
		Labels: map[string]string{"started-by": "opentelemetry-testing"},
		Env:    env,
	}, &container.HostConfig{
		PublishAllPorts: true,
	}, &network.NetworkingConfig{}, nil, "")
	if err != nil {
		c.t.Fatalf("failed creating container with image %v: %v", image, err)
	}
	return created
}

func (c *Containers) startContainer(created container.ContainerCreateCreatedBody) Container {
	if err := c.cli.ContainerStart(context.Background(), created.ID, types.ContainerStartOptions{}); err != nil {
		c.t.Fatalf("failed starting container %v: %v", created.ID, err)
	}

	con, err := c.cli.ContainerInspect(context.Background(), created.ID)
	if err != nil {
		c.t.Fatalf("failed inspecting container %v: %v", created.ID, err)
	}

	if con.NetworkSettings.IPAddress == "" {
		c.t.Fatalf("failed to acquire IP address")
	}

	ports := con.NetworkSettings.Ports

	return Container{
		ID:    ID(created.ID),
		ports: ports,
		t:     c.t,
	}
}

func (c *Containers) waitForPorts(con Container) {
	for _, port := range con.waitForPorts {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

		func() {
			defer cancel()
			addr := con.AddrForPort(port)
			if addr == "" {
				c.t.Fatalf("unable to find port binding for port %d", port)
			}

			err := backoff.Retry(func() error {
				conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
				if conn != nil {
					_ = conn.Close()
				}
				return err
			}, backoff.WithContext(backoff.NewConstantBackOff(100*time.Millisecond), ctx))
			require.NoError(c.t, err, "failed waiting for %v", addr)
		}()
	}

}

// StartImage starts a container with the given image and zero or more ContainerOptions.
func (c *Containers) StartImage(image string, opts ...Option) Container {
	return c.StartImageWithEnv(image, nil, opts...)
}

func (c *Containers) StartImageWithEnv(image string, env []string, opts ...Option) Container {
	c.pullImage(image)
	created := c.createContainer(image, env)
	con := c.startContainer(created)
	c.runningContainers[con.ID] = con

	for _, opt := range opts {
		opt(&con)
	}

	c.waitForPorts(con)

	return con
}

func (c *Containers) RemoveContainer(container Container) error {
	err := c.cli.ContainerRemove(context.Background(), string(container.ID), types.ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	})
	if err != nil {
		err = fmt.Errorf("failed removing container %v: %w", container.ID, err)
		c.t.Log(err.Error())
		return err
	}
	c.t.Logf("Removed container %v", container.ID)
	delete(c.runningContainers, container.ID)
	return nil
}
