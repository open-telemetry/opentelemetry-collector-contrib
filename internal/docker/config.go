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
)

type Config struct {
	// The URL of the docker server. Default is "unix:///var/run/docker.sock"
	// on non-Windows and "npipe:////./pipe/docker_engine" on Windows
	Endpoint string `mapstructure:"endpoint"`

	// The maximum amount of time to wait for docker API responses. Default is 5s
	Timeout time.Duration `mapstructure:"timeout"`

	// A list of filters whose matching images are to be excluded. Supports literals, globs, and regex.
	ExcludedImages []string `mapstructure:"excluded_images"`

	// Docker client API version.
	DockerAPIVersion string `mapstructure:"api_version"`
}

// NewConfig creates a new config to be used when creating
// a docker client
func NewConfig(endpoint string, timeout time.Duration, excludedImages []string, apiVersion string) (*Config, error) {
	cfg := &Config{
		Endpoint:         endpoint,
		Timeout:          timeout,
		ExcludedImages:   excludedImages,
		DockerAPIVersion: apiVersion,
	}
	return cfg, cfg.validate()
}

// NewDefaultConfig creates a new config with default values
// to be used when creating a docker client
func NewDefaultConfig() *Config {
	cfg := &Config{
		Endpoint:         client.DefaultDockerHost,
		Timeout:          5 * time.Second,
		DockerAPIVersion: minimumRequiredDockerAPIVersion,
	}

	return cfg
}

// validate asserts that an endpoint field is set
// on the config struct
func (config Config) validate() error {
	if config.Endpoint == "" {
		return errors.New("config.Endpoint must be specified")
	}
	if err := VersionIsValidAndGTE(config.DockerAPIVersion, minimumRequiredDockerAPIVersion); err != nil {
		return err
	}
	return nil
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
