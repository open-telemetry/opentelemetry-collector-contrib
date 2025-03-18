// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8slogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8slogreceiver"
import (
	"fmt"

	"go.opentelemetry.io/collector/confmap"
	"go.uber.org/zap"
)

var (
	runtimeAPIBuilderFactories = map[string]func() runtimeAPIBuilder{
		"docker": func() runtimeAPIBuilder {
			return &DockerConfig{}
		},
		"cri": func() runtimeAPIBuilder {
			return &CRIConfig{}
		},
	}
)

type RuntimeAPIConfig struct {
	runtimeAPIBuilder
}

// Unmarshal is the trick to unmarshal the config with different types.
func (c *RuntimeAPIConfig) Unmarshal(component *confmap.Conf) error {
	if !component.IsSet("type") {
		return fmt.Errorf("missing required field 'type'")
	}
	typeInterface := component.Get("type")

	typeString, ok := typeInterface.(string)
	if !ok {
		return fmt.Errorf("non-string type %T for field 'type'", typeInterface)
	}

	builderFunc, ok := runtimeAPIBuilderFactories[typeString]
	if !ok {
		return fmt.Errorf("unsupported runtime api '%s'", typeString)
	}

	builder := builderFunc()
	if err := component.Unmarshal(builder); err != nil {
		return fmt.Errorf("failed to unmarshal to %s: %w", typeString, err)
	}

	c.runtimeAPIBuilder = builder
	return nil
}

type runtimeAPIBuilder interface {
	Validate() error
	Type() string
	NewClient(logger *zap.Logger, hostRoot string) (any, error)
}

type baseRuntimeAPIConfig struct {
	Type string `mapstructure:"type"`
}

// CRIConfig allows specifying how to connect to the CRI server.
type CRIConfig struct {
	baseRuntimeAPIConfig `mapstructure:",squash"`

	// Addr represents the address of the CRI endpoint.
	// By default, it is set to <HOST_ROOT>/run/containerd/containerd.sock.
	Addr string `mapstructure:"addr"`

	// ContainerdState represents the path to the containerd state directory.
	// By default, the directories below are tried in order:
	// - <HOST_ROOT>/run/containerd
	// - <HOST_ROOT>/var/run/containerd
	ContainerdState string `mapstructure:"containerd_state"`
}

func (c *CRIConfig) Validate() error {
	return nil
}

func (c *CRIConfig) Type() string {
	return "cri"
}
func (c *CRIConfig) NewClient(logger *zap.Logger, hostRoot string) (any, error) {
	_ = logger
	_ = hostRoot
	// TODO implement me
	panic("implement me")
}

// DockerConfig allows specifying how to connect to the Docker daemon.
type DockerConfig struct {
	baseRuntimeAPIConfig `mapstructure:",squash"`

	// Addr represents the address of the Docker daemon.
	// By default, it is set to <HOST_ROOT>/var/run/docker.sock.
	Addr string `mapstructure:"addr"`

	// ContainerdAddr represents the address of the containerd daemon.
	// By default, directories below are tried in order:
	// - <HOST_ROOT>/run/docker/containerd/containerd.sock
	// - <HOST_ROOT>/run/containerd/containerd.sock
	// - <HOST_ROOT>/var/run/containerd/containerd.sock
	ContainerdAddr string `mapstructure:"containerd_addr"`
}

func (c *DockerConfig) Validate() error {
	return nil
}

func (c *DockerConfig) Type() string {
	return "docker"
}

func (c *DockerConfig) NewClient(logger *zap.Logger, hostRoot string) (any, error) {
	_ = logger
	_ = hostRoot
	// TODO implement me
	panic("implement me")
}
