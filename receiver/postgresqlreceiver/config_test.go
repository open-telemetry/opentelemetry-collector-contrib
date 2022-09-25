// Copyright The OpenTelemetry Authors
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

package postgresqlreceiver

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/multierr"
)

func TestValidate(t *testing.T) {
	testCases := []struct {
		desc                  string
		defaultConfigModifier func(cfg *Config)
		expected              error
	}{
		{
			desc:                  "missing username and password",
			defaultConfigModifier: func(cfg *Config) {},
			expected: multierr.Combine(
				errors.New(ErrNoUsername),
				errors.New(ErrNoPassword),
			),
		},
		{
			desc: "missing password",
			defaultConfigModifier: func(cfg *Config) {
				cfg.Username = "otel"
			},
			expected: multierr.Combine(
				errors.New(ErrNoPassword),
			),
		},
		{
			desc: "missing username",
			defaultConfigModifier: func(cfg *Config) {
				cfg.Password = "otel"
			},
			expected: multierr.Combine(
				errors.New(ErrNoUsername),
			),
		},
		{
			desc: "bad endpoint",
			defaultConfigModifier: func(cfg *Config) {
				cfg.Username = "otel"
				cfg.Password = "otel"
				cfg.Endpoint = "open-telemetry"
			},
			expected: multierr.Combine(
				errors.New(ErrHostPort),
			),
		},
		{
			desc: "bad transport",
			defaultConfigModifier: func(cfg *Config) {
				cfg.Username = "otel"
				cfg.Password = "otel"
				cfg.Transport = "teacup"
			},
			expected: multierr.Combine(
				errors.New(ErrTransportsSupported),
			),
		},
		{
			desc: "unsupported SSL params",
			defaultConfigModifier: func(cfg *Config) {
				cfg.Username = "otel"
				cfg.Password = "otel"
				cfg.ServerName = "notlocalhost"
				cfg.MinVersion = "1.0"
				cfg.MaxVersion = "1.0"
			},
			expected: multierr.Combine(
				fmt.Errorf(ErrNotSupported, "ServerName"),
				fmt.Errorf(ErrNotSupported, "MaxVersion"),
				fmt.Errorf(ErrNotSupported, "MinVersion"),
			),
		},
		{
			desc: "no error",
			defaultConfigModifier: func(cfg *Config) {
				cfg.Username = "otel"
				cfg.Password = "otel"
			},
			expected: nil,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*Config)
			tC.defaultConfigModifier(cfg)
			actual := cfg.Validate()
			require.Equal(t, tC.expected, actual)
		})
	}
}
