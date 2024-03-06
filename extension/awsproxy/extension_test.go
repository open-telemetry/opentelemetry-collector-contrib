// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsproxy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confignet"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy"
)

func TestInvalidEndpoint(t *testing.T) {
	_, err := newXrayProxy(
		&Config{
			ProxyConfig: proxy.Config{
				TCPAddrConfig: confignet.TCPAddrConfig{
					Endpoint: "invalidEndpoint",
				},
			},
		},
		componenttest.NewNopTelemetrySettings(),
	)
	assert.Error(t, err)
}
