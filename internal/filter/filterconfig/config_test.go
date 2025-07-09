// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filterconfig

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset/regexp"
)

func Test_ValidateWithSpans(t *testing.T) {
	tests := []struct {
		name    string
		config  *MatchProperties
		wantErr bool
	}{
		{
			name: "valid",
			config: &MatchProperties{
				SpanKinds: []string{"kind"},
				Config: filterset.Config{
					MatchType: filterset.Regexp,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid",
			config: &MatchProperties{
				SpanKinds: []string{"kind"},
				Config: filterset.Config{
					MatchType: filterset.Regexp,
				},
				MetricNames: []string{"name"},
			},
			wantErr: true,
		},
		{
			name:    "invalid empty",
			config:  &MatchProperties{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr {
				require.Error(t, tt.config.ValidateForSpans())
			} else {
				require.NoError(t, tt.config.ValidateForSpans())
			}
		})
	}
}

func Test_ValidateWithMetrics(t *testing.T) {
	tests := []struct {
		name    string
		config  *MatchProperties
		wantErr bool
	}{
		{
			name: "valid",
			config: &MatchProperties{
				MetricNames: []string{"kind"},
				Config: filterset.Config{
					MatchType: filterset.Regexp,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid",
			config: &MatchProperties{
				SpanKinds: []string{"kind"},
				Config: filterset.Config{
					MatchType: filterset.Regexp,
				},
				MetricNames: []string{"name"},
			},
			wantErr: true,
		},
		{
			name:    "invalid empty",
			config:  &MatchProperties{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr {
				require.Error(t, tt.config.ValidateForMetrics())
			} else {
				require.NoError(t, tt.config.ValidateForMetrics())
			}
		})
	}
}

func Test_ValidateWithLogs(t *testing.T) {
	tests := []struct {
		name    string
		config  *MatchProperties
		wantErr bool
	}{
		{
			name: "valid",
			config: &MatchProperties{
				LogBodies: []string{"kind"},
				Config: filterset.Config{
					MatchType: filterset.Regexp,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid",
			config: &MatchProperties{
				SpanKinds: []string{"kind"},
				Config: filterset.Config{
					MatchType: filterset.Regexp,
				},
				LogBodies: []string{"name"},
			},
			wantErr: true,
		},
		{
			name:    "invalid empty",
			config:  &MatchProperties{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr {
				require.Error(t, tt.config.ValidateForLogs())
			} else {
				require.NoError(t, tt.config.ValidateForLogs())
			}
		})
	}
}

func Test_CreateMetricMatchPropertiesFromDefault(t *testing.T) {
	tests := []struct {
		name    string
		config  *MatchProperties
		want    *MetricMatchProperties
		wantErr bool
	}{
		{
			name:    "nil",
			config:  nil,
			wantErr: false,
			want:    nil,
		},
		{
			name: "invalid log bodies",
			config: &MatchProperties{
				MetricNames: []string{"kind"},
				Config: filterset.Config{
					MatchType: filterset.Regexp,
				},
				LogBodies: []string{"body"},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid log severity texts",
			config: &MatchProperties{
				MetricNames: []string{"kind"},
				Config: filterset.Config{
					MatchType: filterset.Regexp,
				},
				LogSeverityTexts: []string{"text"},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid log severity number",
			config: &MatchProperties{
				MetricNames: []string{"kind"},
				Config: filterset.Config{
					MatchType: filterset.Regexp,
				},
				LogSeverityNumber: &LogSeverityNumberMatchProperties{},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid span names",
			config: &MatchProperties{
				MetricNames: []string{"kind"},
				Config: filterset.Config{
					MatchType: filterset.Regexp,
				},
				SpanNames: []string{"text"},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid span kinds",
			config: &MatchProperties{
				MetricNames: []string{"kind"},
				Config: filterset.Config{
					MatchType: filterset.Regexp,
				},
				SpanKinds: []string{"text"},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid services",
			config: &MatchProperties{
				MetricNames: []string{"kind"},
				Config: filterset.Config{
					MatchType: filterset.Regexp,
				},
				Services: []string{"text"},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid attributes",
			config: &MatchProperties{
				MetricNames: []string{"kind"},
				Config: filterset.Config{
					MatchType: filterset.Regexp,
				},
				Attributes: []Attribute{
					{Key: "key", Value: "val"},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid span kinds",
			config: &MatchProperties{
				MetricNames: []string{"kind"},
				Config: filterset.Config{
					MatchType: filterset.Regexp,
				},
				Libraries: []InstrumentationLibrary{
					{Name: "name"},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "valid",
			config: &MatchProperties{
				MetricNames: []string{"kind"},
				Config: filterset.Config{
					MatchType: filterset.Regexp,
					RegexpConfig: &regexp.Config{
						CacheMaxNumEntries: 1,
					},
				},
			},
			want: &MetricMatchProperties{
				MetricNames: []string{"kind"},
				MatchType:   MetricRegexp,
				RegexpConfig: &regexp.Config{
					CacheMaxNumEntries: 1,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr {
				res, err := CreateMetricMatchPropertiesFromDefault(tt.config)
				require.Error(t, err)
				require.Nil(t, res)
			} else {
				res, err := CreateMetricMatchPropertiesFromDefault(tt.config)
				require.NoError(t, err)
				require.Equal(t, tt.want, res)
			}
		})
	}
}
