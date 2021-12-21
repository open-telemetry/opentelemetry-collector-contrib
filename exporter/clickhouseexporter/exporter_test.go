// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package clickhouseexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestExporter_New(t *testing.T) {
	type validate func(*testing.T, *clickhouseExporter, error)

	success := func(t *testing.T, exporter *clickhouseExporter, err error) {
		require.Nil(t, err)
		require.NotNil(t, exporter)
	}

	failWith := func(want error) validate {
		return func(t *testing.T, exporter *clickhouseExporter, err error) {
			require.Nil(t, exporter)
			require.NotNil(t, err)
			if err.Error() != want.Error() {
				t.Fatalf("Expected error '%v', but got '%v'", want, err)
			}
		}
	}

	tests := map[string]struct {
		config *Config
		want   validate
		env    map[string]string
	}{
		"no address": {
			config: withDefaultConfig(func(cfg *Config) {
				cfg.DSN = ""
			}),
			want: failWith(errConfigNoDSN),
		},
		"valid": {
			config: withDefaultConfig(func(cfg *Config) {
				cfg.DSN = "tcp://127.0.0.1:9000"
			}),
			want: success,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			exporter, err := newExporter(zap.NewNop(), test.config)
			if exporter != nil {
				defer func() {
					require.NoError(t, exporter.Shutdown(context.TODO()))
				}()
			}

			test.want(t, exporter, err)
		})
	}
}
