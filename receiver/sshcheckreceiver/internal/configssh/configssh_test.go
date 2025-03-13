// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package configssh // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sshcheckreceiver/internal/configssh"

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensionauth/extensionauthtest"
	"golang.org/x/crypto/ssh"
)

type mockHost struct {
	component.Host
	ext map[component.ID]extension.Extension
}

func TestAllSSHClientSettings(t *testing.T) {
	host := &mockHost{
		ext: map[component.ID]extension.Extension{
			component.MustNewID("testauth"): extensionauthtest.NewNopClient(),
		},
	}

	endpoint := "localhost:2222"
	timeout := time.Second * 5
	username := "otelu"

	tests := []struct {
		name        string
		settings    SSHClientSettings
		shouldError bool
	}{
		{
			name: "valid_settings_keyfile",
			settings: SSHClientSettings{
				Endpoint: endpoint,
				Timeout:  timeout,
				Username: username,
				KeyFile:  "testdata/keys/id_rsa",
			},
			shouldError: false,
		},
		{
			name: "valid_settings_password",
			settings: SSHClientSettings{
				Endpoint: endpoint,
				Timeout:  timeout,
				Username: username,
				Password: "otelp",
			},
			shouldError: false,
		},
		{
			name: "valid_settings_keyfile_and_password",
			settings: SSHClientSettings{
				Endpoint: endpoint,
				Timeout:  timeout,
				Username: username,
				KeyFile:  "testdata/keys/id_rsa_enc",
				Password: "r54-G0pher_t3st$",
			},
			shouldError: false,
		},
		{
			name: "valid_settings_keyfile_and_password",
			settings: SSHClientSettings{
				Endpoint: endpoint,
				Timeout:  timeout,
				Username: username,
				KeyFile:  "testdata/keys/id_rsa_enc",
				Password: "r54-G0pher_t3st$",
			},
			shouldError: false,
		},
		{
			name: "valid_settings_insecure_ignore_host_key",
			settings: SSHClientSettings{
				Endpoint:      endpoint,
				Timeout:       timeout,
				Username:      username,
				KeyFile:       "testdata/keys/id_rsa_enc",
				Password:      "r54-G0pher_t3st$",
				IgnoreHostKey: true,
			},
			shouldError: false,
		},
		{
			name: "invalid_settings_nonexistent_keyfile_path",
			settings: SSHClientSettings{
				Endpoint: endpoint,
				Timeout:  timeout,
				Username: username,
				KeyFile:  "path/to/keyfile",
			},
			shouldError: true,
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

			assert.EqualValues(t, client.ClientConfig.User, test.settings.Username)

			if len(test.settings.KeyFile) > 0 || len(test.settings.Password) > 0 {
				assert.Len(t, client.ClientConfig.Auth, 1)
			}
		})
	}
}

func Test_Client_Dial(t *testing.T) {
	host := &mockHost{
		ext: map[component.ID]extension.Extension{
			component.MustNewID("testauth"): extensionauthtest.NewNopClient(),
		},
	}

	endpoint := "localhost:2222"
	timeout := time.Second * 5
	username := "otelu"
	keyfile := "testdata/keys/id_rsa"

	tests := []struct {
		name        string
		settings    SSHClientSettings
		dial        func(string, string, *ssh.ClientConfig) (*ssh.Client, error)
		shouldError bool
	}{
		{
			name: "dial_sets_client_with_DialFunc",
			settings: SSHClientSettings{
				Endpoint: endpoint,
				Timeout:  timeout,
				Username: username,
				KeyFile:  keyfile,
			},
			dial: func(_, _ string, _ *ssh.ClientConfig) (*ssh.Client, error) {
				return &ssh.Client{}, nil
			},
			shouldError: false,
		},
		{
			name: "dial_returns_error_of_DialFunc",
			settings: SSHClientSettings{
				Endpoint: endpoint,
				Timeout:  timeout,
				Username: username,
				KeyFile:  keyfile,
			},
			dial: func(_, _ string, _ *ssh.ClientConfig) (*ssh.Client, error) {
				return nil, errors.New("dial")
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

			err = client.Dial("localhost:2222")
			if test.shouldError {
				assert.Error(t, err)
				assert.EqualValues(t, (*ssh.Client)(nil), client.Client)
			} else {
				assert.NoError(t, err)
				assert.EqualValues(t, &ssh.Client{}, client.Client)
			}

			if test.settings.IgnoreHostKey {
				assert.EqualValues(t, client.HostKeyCallback, ssh.InsecureIgnoreHostKey()) //#nosec G106
			}
			if len(test.settings.KeyFile) > 0 || len(test.settings.Password) > 0 {
				assert.Len(t, client.ClientConfig.Auth, 1)
			}
		})
	}
}

func Test_Client_ToSFTPClient(t *testing.T) {
	host := &mockHost{
		ext: map[component.ID]extension.Extension{
			component.MustNewID("testauth"): extensionauthtest.NewNopClient(),
		},
	}

	endpoint := "localhost:2222"
	timeout := time.Second * 5
	username := "otelu"
	keyfile := "testdata/keys/id_rsa"
	tests := []struct {
		name        string
		settings    SSHClientSettings
		dial        func(string, string, *ssh.ClientConfig) (*ssh.Client, error)
		shouldError bool
	}{
		{
			name: "dial_sets_client_with_DialFunc",
			settings: SSHClientSettings{
				Endpoint: endpoint,
				Timeout:  timeout,
				Username: username,
				KeyFile:  keyfile,
			},
			dial: func(_, _ string, _ *ssh.ClientConfig) (*ssh.Client, error) {
				return &ssh.Client{}, nil
			},
			shouldError: false,
		},
		{
			name: "dial_sets_client_with_DialFunc",
			settings: SSHClientSettings{
				Endpoint: endpoint,
				Timeout:  timeout,
				Username: username,
				KeyFile:  keyfile,
			},
			dial: func(_, _ string, _ *ssh.ClientConfig) (*ssh.Client, error) {
				return nil, errors.New("dial")
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

			if test.shouldError {
				err := client.Dial("localhost:2222")
				assert.Error(t, err)
				assert.EqualValues(t, (*ssh.Client)(nil), client.Client)
			} else {
				err := client.Dial("localhost:2222")
				assert.NoError(t, err)
				_, err = client.SFTPClient()
				assert.Error(t, err)
			}
		})
	}
}
