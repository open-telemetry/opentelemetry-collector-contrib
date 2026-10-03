// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package openshift // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/openshift"

import (
	"go.opentelemetry.io/collector/config/configtls"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/openshift/internal/metadata"
)

const defaultCAPath = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt" //#nosec

// Config can contain user-specified inputs to overwrite default values.
// Unset fields are resolved by the SDK detector from the pod environment.
type Config struct {
	// Address is the address of the openshift api server
	Address string `mapstructure:"address"`

	// Token is used to identify against the openshift api server
	Token string `mapstructure:"token"`

	// TLSs contains TLS configurations that are specific to client
	// connection used to communicate with the OpenShift API.
	TLSs configtls.ClientConfig `mapstructure:"tls"`

	ResourceAttributes metadata.ResourceAttributesConfig `mapstructure:"resource_attributes"`
}

func CreateDefaultConfig() Config {
	return Config{
		ResourceAttributes: metadata.DefaultResourceAttributesConfig(),
	}
}
