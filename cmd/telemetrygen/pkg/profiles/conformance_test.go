// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package profiles

import (
	"testing"
	"time"

	"github.com/open-telemetry/sig-profiling/profcheck"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pprofile"
	otlpprofiles "go.opentelemetry.io/proto/otlp/profiles/v1development"
	"go.uber.org/zap/zaptest"
	"google.golang.org/protobuf/proto"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/config"
)

// assertProfilesConform validates that every generated profile conforms to the
// OTLP profiles signal schema using profcheck, the OpenTelemetry SIG profiling
// conformance checker. The pdata profiles are marshaled to OTLP proto bytes and
// unmarshaled into the development proto type that profcheck operates on.
func assertProfilesConform(t *testing.T, exported []pprofile.Profiles) {
	t.Helper()
	require.NotEmpty(t, exported, "expected at least one exported profile to check")

	checker := profcheck.ConformanceChecker{
		CheckDictionaryDuplicates: true,
		CheckSampleTimestampShape: true,
	}
	marshaler := &pprofile.ProtoMarshaler{}

	for i, td := range exported {
		b, err := marshaler.MarshalProfiles(td)
		require.NoErrorf(t, err, "marshal exported profile %d", i)

		var data otlpprofiles.ProfilesData
		require.NoErrorf(t, proto.Unmarshal(b, &data), "unmarshal exported profile %d into OTLP proto", i)

		require.NoErrorf(t, checker.Check(&data), "exported profile %d is not OTLP-conformant", i)
	}
}

func TestGenerateProfileConformance(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
	}{
		{
			name: "default",
			cfg: &Config{
				NumProfiles:     1,
				SampleCount:     5,
				StackDepth:      3,
				UniqueFunctions: 10,
				ProfileDuration: 30 * time.Second,
			},
		},
		{
			name: "trace_correlation",
			cfg: &Config{
				NumProfiles:     1,
				SampleCount:     5,
				StackDepth:      3,
				UniqueFunctions: 10,
				ProfileDuration: 30 * time.Second,
				TraceID:         "ae87dadd90e9935a4bc9660628efd569",
				SpanID:          "5828fa4960140870",
			},
		},
		{
			name: "telemetry_attributes",
			cfg: &Config{
				Config: config.Config{
					TelemetryAttributes: config.KeyValue{"key1": "value1", "key2": "value2"},
				},
				NumProfiles:     1,
				SampleCount:     5,
				StackDepth:      3,
				UniqueFunctions: 10,
				ProfileDuration: 30 * time.Second,
			},
		},
		{
			name: "load_size",
			cfg: &Config{
				Config: config.Config{
					LoadSize: 1,
				},
				NumProfiles:     1,
				SampleCount:     5,
				StackDepth:      3,
				UniqueFunctions: 10,
				ProfileDuration: 30 * time.Second,
			},
		},
		{
			name: "multiple_profiles",
			cfg: &Config{
				NumProfiles:     5,
				SampleCount:     3,
				StackDepth:      2,
				UniqueFunctions: 4,
				ProfileDuration: 10 * time.Second,
			},
		},
		{
			name: "batched",
			cfg: &Config{
				Config: config.Config{
					Batch:     true,
					BatchSize: 2,
				},
				NumProfiles:     6,
				SampleCount:     3,
				StackDepth:      2,
				UniqueFunctions: 4,
				ProfileDuration: 10 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.cfg.WorkerCount = 1

			m := &mockProfileExporter{}
			exporterFactory := func() (profileExporter, error) {
				return m, nil
			}

			require.NoError(t, run(tt.cfg, exporterFactory, zaptest.NewLogger(t)))
			assertProfilesConform(t, m.profiles)
		})
	}
}
