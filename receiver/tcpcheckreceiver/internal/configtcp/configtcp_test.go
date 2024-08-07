// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package configtcp // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver/internal/configtcp"

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/auth/authtest"
)

type mockHost struct {
	component.Host
	ext map[component.ID]extension.Extension
}

func TestAllSSHClientSettings(t *testing.T) {
	host := &mockHost{
		ext: map[component.ID]extension.Extension{
			component.MustNewID("testauth"): &authtest.MockClient{},
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

			assert.EqualValues(t, client.Address, test.settings.Endpoint)
			assert.EqualValues(t, client.Timeout, test.settings.Timeout)
		})
	}
}

func Test_Client_Dial(t *testing.T) {
	host := &mockHost{
		ext: map[component.ID]extension.Extension{
			component.MustNewID("testauth"): &authtest.MockClient{},
		},
	}

	endpoint := "localhost:8080"
	timeout := time.Second * 5

	tests := []struct {
		name        string
		settings    TCPClientSettings
		dial        func(network, address string, timeout time.Duration) (net.Conn, error)
		shouldError bool
	}{
		{
			name: "dial_sets_client_with_DialFunc",
			settings: TCPClientSettings{
				Endpoint: endpoint,
				Timeout:  timeout,
			},
			dial: func(network, address string, timeout time.Duration) (net.Conn, error) {
				return nil, nil
			},
			shouldError: false,
		},
		{
			name: "dial_returns_error_of_DialFunc",
			settings: TCPClientSettings{
				Endpoint: endpoint,
				Timeout:  timeout,
			},
			dial: func(network, address string, timeout time.Duration) (net.Conn, error) {
				return nil, fmt.Errorf("dial")
			},
			shouldError: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tt := componenttest.NewNopTelemetrySettings()
			tt.TracerProvider = nil

			client, err := test.settings.ToClient(host, tt)
			assert.NoError(t, err)

			client.DialFunc = test.dial

			err = client.Dial(endpoint, timeout)
			if test.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
