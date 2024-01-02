// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubelet // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/kubelet"

import (
	"go.opentelemetry.io/collector/config/configtls"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

// ClientConfig for a kubelet client for talking to a kubelet HTTP endpoint.
type ClientConfig struct {
	k8sconfig.APIConfig  `mapstructure:",squash"`
	configtls.TLSSetting `mapstructure:",squash"`
	// InsecureSkipVerify controls whether the client verifies the server's
	// certificate chain and host name.
	InsecureSkipVerify bool `mapstructure:"insecure_skip_verify"`
}
