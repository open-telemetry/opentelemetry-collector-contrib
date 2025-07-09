// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsproxy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confignet"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy"
)

func TestInvalidEndpoint(t *testing.T) {
	x, err := newXrayProxy(
		&Config{
			ProxyConfig: proxy.Config{
				TCPAddrConfig: confignet.TCPAddrConfig{
					Endpoint: "invalidEndpoint",
				},
			},
		},
		componenttest.NewNopTelemetrySettings(),
	)
	assert.NoError(t, err)
	err = x.Start(context.Background(), componenttest.NewNopHost())
	defer assert.NoError(t, x.Shutdown(context.Background()))
	assert.Error(t, err)
}
