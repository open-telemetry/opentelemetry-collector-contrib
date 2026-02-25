// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package docker // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker"

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/client"
	"go.opentelemetry.io/collector/confmap"
)

type Config struct {
	// The URL of the docker server. Default is "unix:///var/run/docker.sock"
	// on non-Windows and "npipe:////./pipe/docker_engine" on Windows
	Endpoint string `mapstructure:"endpoint"`

	// The maximum amount of time to wait for docker API responses. Default is 5s
	Timeout time.Duration `mapstructure:"timeout"`

	// A list of filters whose matching images are to be excluded. Supports literals, globs, and regex.
	ExcludedImages []string `mapstructure:"excluded_images"`

	// Docker client API version. If empty, the client will auto-negotiate
	// the API version with the Docker daemon using version negotiation.
	DockerAPIVersion string `mapstructure:"api_version"`
}

func (config *Config) Unmarshal(conf *confmap.Conf) error {
	// WithIgonreUnused needed because this configuration is embedded inside other configurations
	err := conf.Unmarshal(config, confmap.WithIgnoreUnused())
	if err != nil {
		if floatAPIVersion, ok := conf.Get("api_version").(float64); ok {
			return fmt.Errorf(
				"%w.\n\nHint: You may want to wrap the 'api_version' value in quotes (api_version: \"%1.2f\")",
				err,
				floatAPIVersion,
			)
		}
		return err
	}
	return nil
}

func (config Config) Validate() error {
	if config.Endpoint == "" {
		return errors.New("endpoint must be specified")
	}
	return nil
}

// NewConfig creates a new config to be used when creating
// a docker client
func NewConfig(endpoint string, timeout time.Duration, excludedImages []string, apiVersion string) *Config {
	cfg := &Config{
		Endpoint:         endpoint,
		Timeout:          timeout,
		ExcludedImages:   excludedImages,
		DockerAPIVersion: apiVersion,
	}
	return cfg
}

// NewDefaultConfig creates a new config with default values
// to be used when creating a docker client
// DockerAPIVersion is intentionally left empty for auto-negotiation.
func NewDefaultConfig() *Config {
	cfg := &Config{
		Endpoint: client.DefaultDockerHost,
		Timeout:  5 * time.Second,
	}

	return cfg
}

type apiVersion struct {
	major int
	minor int
}

func NewAPIVersion(version string) (string, error) {
	s := strings.TrimSpace(version)
	split := strings.Split(s, ".")

	invalidVersion := "invalid version %q"

	nParts := len(split)
	if s == "" || nParts < 1 || nParts > 2 {
		return "", fmt.Errorf(invalidVersion, s)
	}

	apiVer := new(apiVersion)
	var err error
	target := map[int]*int{0: &apiVer.major, 1: &apiVer.minor}
	for i, part := range split {
		part = strings.TrimSpace(part)
		if part != "" {
			if *target[i], err = strconv.Atoi(part); err != nil {
				return "", fmt.Errorf(invalidVersion+": %w", s, err)
			}
		}
	}

	return fmt.Sprintf("%d.%d", apiVer.major, apiVer.minor), nil
}

// MustNewAPIVersion evaluates version as a client api version and panics if invalid.
func MustNewAPIVersion(version string) string {
	v, err := NewAPIVersion(version)
	if err != nil {
		panic(err)
	}
	return v
}

// VersionIsValidAndGTE evalutes version as a client api version and returns an error if invalid or less than gte.
// gte is assumed to be valid (easiest if result of MustNewAPIVersion on initialization)
func VersionIsValidAndGTE(version, gte string) error {
	v, err := NewAPIVersion(version)
	if err != nil {
		return err
	}
	if versions.LessThan(v, gte) {
		return fmt.Errorf(`"api_version" %s must be at least %s`, version, gte)
	}
	return nil
}
