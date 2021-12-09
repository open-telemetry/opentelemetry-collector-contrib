// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package couchdbreceiver

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.uber.org/multierr"
)

func TestValidate(t *testing.T) {
	testCases := []struct {
		desc        string
		cfg         *Config
		expectedErr error
	}{
		{
			desc: "missing required fields",
			cfg:  &Config{},
			expectedErr: multierr.Combine(
				errMissingUsername,
				errMissingPassword,
			),
		},
		{
			desc: "missing schema",
			cfg: &Config{
				Username: "otel",
				Password: "otel",
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "localhost:5984",
				},
			},
			expectedErr: errInvalidScheme,
		},
		{
			desc: "missing hostname",
			cfg: &Config{
				Username: "otel",
				Password: "otel",
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "http://:5984",
				},
			},
			expectedErr: fmt.Errorf(errInvalidHost.Error(), "http://:5984"),
		},
		{
			desc: "no error",
			cfg: &Config{
				Username: "otel",
				Password: "otel",
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "http://localhost:5984",
				},
			},
			expectedErr: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			actualErr := tc.cfg.Validate()
			require.Equal(t, tc.expectedErr, actualErr)
		})
	}
}

func TestValidateNotEmpty(t *testing.T) {
	testCases := []struct {
		desc        string
		cfg         *Config
		expectedErr error
	}{
		{
			desc: "missing username and password",
			cfg:  &Config{},
			expectedErr: multierr.Combine(
				errMissingUsername,
				errMissingPassword,
			),
		},
		{
			desc: "missing password",
			cfg: &Config{
				Username: "otelu",
			},
			expectedErr: multierr.Combine(
				errMissingPassword,
			),
		},
		{
			desc: "missing username",
			cfg: &Config{
				Password: "otelp",
			},
			expectedErr: multierr.Combine(
				errMissingUsername,
			),
		},
		{
			desc: "no error",
			cfg: &Config{
				Username: "otel",
				Password: "otel",
			},
			expectedErr: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			actualErr := validateNotEmpty(tc.cfg)
			require.Equal(t, tc.expectedErr, actualErr)
		})
	}
}

func TestValidateEndpointFormat(t *testing.T) {
	testCases := []struct {
		desc        string
		cfg         *Config
		expectedErr error
	}{
		{
			desc: "missing schema",
			cfg: &Config{
				Username: "otel",
				Password: "otel",
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "localhost:5984",
				},
			},
			expectedErr: errInvalidScheme,
		},
		{
			desc: "invalid endpoint",
			cfg: &Config{
				Username: "otel",
				Password: "otel",
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "http://localhost with a space:5984",
				},
			},
			expectedErr: fmt.Errorf(errInvalidEndpoint.Error(), "http://localhost with a space:5984"),
		},
		{
			desc: "missing hostname",
			cfg: &Config{
				Username: "otel",
				Password: "otel",
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "http://",
				},
			},
			expectedErr: fmt.Errorf(errInvalidHost.Error(), "http://"),
		},
		{
			desc: "no error",
			cfg: &Config{
				Username: "otel",
				Password: "otel",
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "http://localhost:5984",
				},
			},
			expectedErr: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			actualErr := validateEndpoint(tc.cfg)
			require.Equal(t, tc.expectedErr, actualErr)
		})
	}
}

func TestValidSchema(t *testing.T) {
	testcases := []struct {
		desc        string
		url         string
		expectedErr bool
	}{
		{
			desc:        "http url",
			url:         "http://localhost",
			expectedErr: true,
		},
		{
			desc:        "https url",
			url:         "https://localhost",
			expectedErr: true,
		},
		{
			desc:        "HTTP upper",
			url:         "HTTP://localhost",
			expectedErr: true,
		},
		{
			desc:        "everything else",
			url:         "ht",
			expectedErr: false,
		},
		{
			desc:        "everything else",
			url:         "localhost",
			expectedErr: false,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.desc, func(t *testing.T) {
			require.Equal(t, tc.expectedErr, validSchema(tc.url))
		})
	}
}
