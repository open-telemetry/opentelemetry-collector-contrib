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

package elasticsearchexporter

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestExporter_New(t *testing.T) {
	type validate func(*testing.T, *elasticsearchExporter, error)

	success := func(t *testing.T, exporter *elasticsearchExporter, err error) {
		require.Nil(t, err)
		require.NotNil(t, exporter)
	}

	failWith := func(want error) validate {
		return func(t *testing.T, exporter *elasticsearchExporter, err error) {
			require.Nil(t, exporter)
			require.NotNil(t, err)
			if !errors.Is(err, want) {
				t.Fatalf("Expected error '%v', but got '%v'", want, err)
			}
		}
	}

	failWithMessage := func(msg string) validate {
		return func(t *testing.T, exporter *elasticsearchExporter, err error) {
			require.Nil(t, exporter)
			require.NotNil(t, err)
			require.Contains(t, err.Error(), msg)
		}
	}

	tests := map[string]struct {
		config *Config
		want   validate
		env    map[string]string
	}{
		"no endpoint": {
			config: withDefaultConfig(nil),
			want:   failWith(errConfigNoEndpoint),
		},
		"create from default config with ELASTICSEARCH_URL environment variable": {
			config: withDefaultConfig(nil),
			want:   success,
			env:    map[string]string{defaultElasticsearchEnvName: "localhost:9200"},
		},
		"create from default with endpoints": {
			config: withDefaultConfig(func(cfg *Config) {
				cfg.Endpoints = []string{"test:9200"}
			}),
			want: success,
		},
		"create with cloudid": {
			config: withDefaultConfig(func(cfg *Config) {
				cfg.CloudID = "foo:YmFyLmNsb3VkLmVzLmlvJGFiYzEyMyRkZWY0NTY="
			}),
			want: success,
		},
		"create with invalid cloudid": {
			config: withDefaultConfig(func(cfg *Config) {
				cfg.CloudID = "invalid"
			}),
			want: failWithMessage("cannot parse CloudID"),
		},
		"fail if endpoint and cloudid are set": {
			config: withDefaultConfig(func(cfg *Config) {
				cfg.Endpoints = []string{"test:9200"}
				cfg.CloudID = "foo:YmFyLmNsb3VkLmVzLmlvJGFiYzEyMyRkZWY0NTY="
			}),
			want: failWithMessage("Addresses and CloudID are set"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			env := test.env
			if len(env) == 0 {
				env = map[string]string{defaultElasticsearchEnvName: ""}
			}

			oldEnv := make(map[string]string, len(env))
			defer func() {
				for k, v := range oldEnv {
					os.Setenv(k, v)
				}
			}()

			for k := range env {
				oldEnv[k] = os.Getenv(k)
			}
			for k, v := range env {
				os.Setenv(k, v)
			}

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
