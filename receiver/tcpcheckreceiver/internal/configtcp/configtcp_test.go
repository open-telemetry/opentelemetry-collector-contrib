// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package configtcp // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver/internal/configtcp"

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensionauth/extensionauthtest"
)

type mockHost struct {
	component.Host
	ext map[component.ID]extension.Extension
}

func TestAllTCPClientSettings(t *testing.T) {
	host := &mockHost{
		ext: map[component.ID]extension.Extension{
			component.MustNewID("testauth"): extensionauthtest.NewNopClient(),
		},
	}

	endpoint := "localhost:8080"
	timeout := time.Second * 5

	tests := []struct {
		name        string
		settings    TCPClientSettings
		shouldError bool
	}{
		{
			name: "valid_settings_endpoint",
			settings: TCPClientSettings{
				Endpoint: endpoint,
				Timeout:  timeout,
			},
			shouldError: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tt := componenttest.NewNopTelemetrySettings()
			tt.TracerProvider = nil

			client, err := test.settings.ToClient(host, tt)
			if test.shouldError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			assert.EqualValues(t, client.TCPAddrConfig.Endpoint, test.settings.Endpoint)
			assert.EqualValues(t, client.TCPAddrConfig.DialerConfig.Timeout, test.settings.Timeout)
		})
	}
}
