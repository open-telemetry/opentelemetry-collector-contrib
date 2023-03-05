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
	"path/filepath"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const defaultEndpoint = "tcp://127.0.0.1:9000"

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	defaultCfg := createDefaultConfig()
	defaultCfg.(*Config).Endpoint = defaultEndpoint

	tests := []struct {
		id       component.ID
		expected component.Config
	}{

		{
			id:       component.NewIDWithName(typeStr, ""),
			expected: defaultCfg,
		},
		{
			id: component.NewIDWithName(typeStr, "full"),
			expected: &Config{
				Endpoint:         "tcp://127.0.0.1:9000",
				Database:         "otel",
				Username:         "foo",
				Password:         "bar",
				TTLDays:          3,
				LogsTableName:    "otel_logs",
				TracesTableName:  "otel_traces",
				MetricsTableName: "otel_metrics",
				TimeoutSettings: exporterhelper.TimeoutSettings{
					Timeout: 5 * time.Second,
				},
				RetrySettings: exporterhelper.RetrySettings{
					Enabled:             true,
					InitialInterval:     5 * time.Second,
					MaxInterval:         30 * time.Second,
					MaxElapsedTime:      300 * time.Second,
					RandomizationFactor: backoff.DefaultRandomizationFactor,
					Multiplier:          backoff.DefaultMultiplier,
				},
				QueueSettings: QueueSettings{
					QueueSize: 100,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func withDefaultConfig(fns ...func(*Config)) *Config {
	cfg := createDefaultConfig().(*Config)
	for _, fn := range fns {
		fn(cfg)
	}
	return cfg
}

func TestConfig_buildDBOptions(t *testing.T) {
	type fields struct {
		Endpoint string
		Username string
		Password string
		Database string
	}
	type args struct {
		database string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    struct{ address, username, password, database string }
		wantErr error
	}{
		{
			name: "valid config",
			fields: fields{
				Endpoint: defaultEndpoint,
				Username: "foo",
				Password: "bar",
				Database: "otel",
			},
			args: args{
				database: defaultDatabase,
			},
			want: struct{ address, username, password, database string }{
				address:  "127.0.0.1:9000",
				username: "foo",
				password: "bar",
				database: "default",
			},
		},
		{
			name: "invalid config",
			fields: fields{
				Endpoint: "127.0.0.1:9000",
			},
			wantErr: errConfigInvalidDSN,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Endpoint: tt.fields.Endpoint,
				Username: tt.fields.Username,
				Password: tt.fields.Password,
				Database: tt.fields.Database,
			}
			got, err := cfg.buildDBOptions(tt.args.database)

			if tt.wantErr != nil {
				assert.Equalf(t, tt.wantErr, err, "buildDSN(%v)", tt.args.database)
			} else {
				assert.Equalf(t, tt.want.address, got.Addr[0], "buildDSN(%v)", tt.args.database)
				assert.Equalf(t, tt.want.username, got.Auth.Username, "buildDSN(%v)", tt.args.database)
				assert.Equalf(t, tt.want.password, got.Auth.Password, "buildDSN(%v)", tt.args.database)
				assert.Equalf(t, tt.want.database, got.Auth.Database, "buildDSN(%v)", tt.args.database)
			}

		})
	}
}
