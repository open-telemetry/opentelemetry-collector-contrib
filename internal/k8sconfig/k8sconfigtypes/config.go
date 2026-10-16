// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package k8sconfigtypes holds dependency-light configuration types for
// connecting to the Kubernetes API. Client constructors that consume these
// types live in the parent k8sconfig package.
package k8sconfigtypes // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig/k8sconfigtypes"

import (
	"errors"
	"fmt"
)

// AuthType describes the type of authentication to use for the K8s API.
type AuthType string

// TODO: Add option for TLS once
// https://go.opentelemetry.io/collector/issues/933
// is addressed.
const (
	// AuthTypeNone means no auth is required.
	AuthTypeNone AuthType = "none"
	// AuthTypeServiceAccount means to use the built-in service account that
	// K8s automatically provisions for each pod.
	AuthTypeServiceAccount AuthType = "serviceAccount"
	// AuthTypeKubeConfig uses local credentials like those used by kubectl.
	AuthTypeKubeConfig AuthType = "kubeConfig"
	// AuthTypeTLS indicates that client TLS auth is desired.
	AuthTypeTLS AuthType = "tls"
)

var authTypes = map[AuthType]bool{
	AuthTypeNone:           true,
	AuthTypeServiceAccount: true,
	AuthTypeKubeConfig:     true,
	AuthTypeTLS:            true,
}

// APIConfig contains options relevant to connecting to the K8s API.
type APIConfig struct {
	// How to authenticate to the K8s API server. This can be one of `none`
	// (for no auth), `serviceAccount` (to use the standard service account
	// token provided to the agent pod), or `kubeConfig` to use credentials
	// from `~/.kube/config`.
	AuthType AuthType `mapstructure:"auth_type"`

	// When using auth_type `kubeConfig`, override the current context.
	Context string `mapstructure:"context"`

	// KubeAPIQPS is the maximum number of queries per second to the Kubernetes API.
	// Uses client-go's default (5) if unset. Increase if you see client-side throttling warnings.
	KubeAPIQPS float32 `mapstructure:"kube_api_qps"`

	// KubeAPIBurst is the maximum burst of requests to the Kubernetes API.
	// Uses client-go's default (10) if unset. Increase if you see client-side throttling warnings.
	KubeAPIBurst int `mapstructure:"kube_api_burst"`
}

// Validate validates the Kubernetes API config.
func (c APIConfig) Validate() error {
	if !authTypes[c.AuthType] {
		return fmt.Errorf("invalid authType for kubernetes: %v", c.AuthType)
	}

	if c.KubeAPIQPS < 0 {
		return errors.New("kube_api_qps must be greater than 0")
	}

	if c.KubeAPIBurst < 0 {
		return errors.New("kube_api_burst must be greater than 0")
	}

	return nil
}
