//// Copyright The OpenTelemetry Authors
//// SPDX-License-Identifier: Apache-2.0
//
//package configtcp // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver/internal/configtcp"
//
//import (
//	"github.com/stretchr/testify/require"
//	"testing"
//	"time"
//
//	"github.com/stretchr/testify/assert"
//	"go.opentelemetry.io/collector/component"
//	"go.opentelemetry.io/collector/component/componenttest"
//	"go.opentelemetry.io/collector/extension"
//)
//
//type mockHost struct {
//	component.Host
//	ext map[component.ID]extension.Extension
//}
//
//func TestAllTCPClientSettings(t *testing.T) {
//	//host := &mockHost{
//	//	ext: map[component.ID]extension.Extension{
//	//		component.MustNewID("testauth"): &authtest.MockClient{},
//	//	},
//	//}
//
//	endpoint := "localhost:8080"
//	timeout := time.Second * 5
//
//	tests := []struct {
//		name        string
//		settings    TCPClientSettings
//		shouldError bool
//	}{
//		{
//			name: "valid_settings_endpoint",
//			settings: TCPClientSettings{
//				Endpoint: endpoint,
//				Timeout:  timeout,
//			},
//			shouldError: false,
//		},
//	}
//	for _, test := range tests {
//		t.Run(test.name, func(t *testing.T) {
//			tt := componenttest.NewNopTelemetrySettings()
//			tt.TracerProvider = nil
//
//			client, err := test.settings.ToClient()
//			if test.shouldError {
//				assert.Error(t, err)
//				return
//			}
//			assert.NoError(t, err)
//
//			assert.EqualValues(t, client.TCPAddrConfig.Endpoint, test.settings.Endpoint)
//			assert.EqualValues(t, client.TCPAddrConfig.DialerConfig.Timeout, test.settings.Timeout)
//		})
//	}
//}
//
//func TestMockTCPServer(t *testing.T) {
//
//	settings := TCPClientSettings{
//		Endpoint: "localhost:8080",
//		Timeout:  time.Second * 5,
//	}
//	client, err := settings.ToClient()
//
//	client.Listen()
//	require.NoError(t, err, "Failed to start listener")
//	defer client.Listener.Close()
//
//	go func() {
//		conn, err := client.Listener.Accept()
//		require.NoError(t, err, "Failed to accept connection")
//		defer conn.Close()
//
//		// Read client message
//		buf := make([]byte, 256)
//		n, err := conn.Read(buf)
//		require.NoError(t, err, "Failed to read from client")
//
//		// Respond to the client
//		conn.Write([]byte("Echo: " + string(buf[:n])))
//	}()
//
//	err = client.Dial()
//	require.NoError(t, err, "Client failed to connect")
//	defer client.Connection.Close()
//
//	// Sends a message
//	message := "Hi, Server!"
//	_, err = client.Connection.Write([]byte(message))
//	require.NoError(t, err, "Client failed to send message")
//
//	buf := make([]byte, 256)
//	n, err := client.Connection.Read(buf)
//	require.NoError(t, err, "Client failed to read response")
//	require.Equal(t, "Echo: "+message, string(buf[:n]), "Unexpected response from server")
//}
