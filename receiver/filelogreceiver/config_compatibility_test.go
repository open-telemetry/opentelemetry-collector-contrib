// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filelogreceiver

import (
	"fmt"
	"maps"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/matcher"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver/internal/metadata"
)

func TestFactoryCompatibility(t *testing.T) {
	base := adapter.NewFactory(ReceiverType{}, metadata.LogsStability)
	factory := NewFactory()
	assert.Equal(t, base.Type(), factory.Type())
	assert.Equal(t, base.CreateDefaultConfig(), factory.CreateDefaultConfig())
	assert.Equal(t, base.LogsStability(), factory.LogsStability())
	assert.Equal(t, base.TracesStability(), factory.TracesStability())
	assert.Equal(t, base.MetricsStability(), factory.MetricsStability())
}

func TestOrderingConfigCompatibility(t *testing.T) {
	const gateID = "filelog.requireExplicitTopN"
	var originallyEnabled bool
	featuregate.GlobalRegistry().VisitAll(func(g *featuregate.Gate) {
		if g.ID() == gateID {
			originallyEnabled = g.IsEnabled()
		}
	})
	t.Cleanup(func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(gateID, originallyEnabled))
	})

	// Do not run in parallel: feature gates use a process-wide registry.
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("gate_enabled=%t", enabled), func(t *testing.T) {
			require.NoError(t, featuregate.GlobalRegistry().Set(gateID, enabled))
			for _, tc := range []struct {
				name string
				sort bool
				topN any
			}{
				{name: "no_ordering"},
				{name: "implicit_limit", sort: true},
				{name: "explicit_legacy_limit", sort: true, topN: 1},
				{name: "all_files", sort: true, topN: 0},
				{name: "custom_limit", sort: true, topN: 3},
				{name: "negative_limit", sort: true, topN: -1},
			} {
				t.Run(tc.name, func(t *testing.T) {
					values := map[string]any{"include": []any{"testdata/*.log"}}
					if tc.sort {
						ordering := map[string]any{
							"regex":   `(?P<name>.*)`,
							"sort_by": []any{map[string]any{"sort_type": "alphabetical", "regex_key": "name"}},
						}
						if tc.topN != nil {
							ordering["top_n"] = tc.topN
						}
						values["ordering_criteria"] = ordering
					}
					factory := NewFactory()
					cfg := factory.CreateDefaultConfig()
					require.NoError(t, confmap.NewFromStringMap(values).Unmarshal(cfg))
					core, logs := observer.New(zap.WarnLevel)
					set := receivertest.NewNopSettings(metadata.Type)
					set.Logger = zap.New(core)
					_, err := factory.CreateLogs(t.Context(), set, cfg, new(consumertest.LogsSink))
					switch {
					case tc.topN == -1:
						require.ErrorContains(t, err, "'top_n' must not be negative")
					case enabled && tc.sort && tc.topN == nil:
						require.ErrorContains(t, err, "'top_n' must be set explicitly")
					default:
						require.NoError(t, err)
					}
					if !enabled && tc.sort && tc.topN == nil {
						require.Equal(t, 1, logs.Len())
						warning := logs.All()[0]
						assert.Equal(t, zap.WarnLevel, warning.Level)
						assert.Contains(t, warning.Message, "top_n to 1")
						assert.Contains(t, warning.Message, "0 to match all files")
						assert.Equal(t, "https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/receiver/filelogreceiver/MIGRATION.md#explicit-ordering-limit", warning.ContextMap()["migration_guide"])
					} else {
						assert.Zero(t, logs.Len())
					}
				})
			}
		})
	}
}

func TestMinimalConfigCompatibility(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*FileLogConfig)
	require.NoError(t, confmap.NewFromStringMap(map[string]any{
		"include": []any{"testdata/*.log"},
	}).Unmarshal(cfg))
	baseline, err := confmaptest.LoadConf(filepath.Join("testdata", "config_compatibility_defaults.yaml"))
	require.NoError(t, err)
	// Do not call a default constructor for the expected configuration: doing so
	// would silently accept a changed default in any embedded Stanza config.
	expected := &FileLogConfig{}
	require.NoError(t, baseline.Unmarshal(expected))
	assert.Equal(t, expected, cfg)
	_, err = factory.CreateLogs(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, new(consumertest.LogsSink))
	require.NoError(t, err)
}

func TestExplicitConfigCompatibility(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_compatibility.yaml"))
	require.NoError(t, err)
	for _, receiverType := range []component.Type{metadata.Type, metadata.DeprecatedType} {
		t.Run(receiverType.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*FileLogConfig)
			require.NoError(t, cm.Unmarshal(cfg))
			require.NoError(t, confmap.Validate(cfg))
			input := cfg.InputConfig
			assert.Equal(t, []string{"testdata/*.log"}, input.Include)
			assert.Equal(t, []string{"testdata/excluded*.log"}, input.Exclude)
			assert.Equal(t, 48*time.Hour, input.ExcludeOlderThan)
			assert.Equal(t, "beginning", input.StartAt)
			assert.Equal(t, 400*time.Millisecond, input.PollInterval)
			assert.Equal(t, 32, input.MaxConcurrentFiles)
			assert.Equal(t, 2, input.MaxBatches)
			assert.EqualValues(t, 2000, input.FingerprintSize)
			assert.EqualValues(t, 8192, input.InitialBufferSize)
			assert.EqualValues(t, 2*1024*1024, input.MaxLogSize)
			assert.Equal(t, "truncate", input.MaxLogSizeBehavior)
			assert.Equal(t, "utf-16le", input.Encoding)
			assert.Equal(t, 2*time.Second, input.FlushPeriod)
			assert.Equal(t, "^START", input.SplitConfig.LineStartPattern)
			assert.True(t, input.SplitConfig.OmitPattern)
			assert.True(t, input.TrimConfig.PreserveLeading)
			assert.True(t, input.TrimConfig.PreserveTrailing)
			assert.False(t, input.IncludeFileName)
			assert.True(t, input.IncludeFilePath)
			assert.True(t, input.IncludeFileNameResolved)
			assert.True(t, input.IncludeFilePathResolved)
			assert.True(t, input.IncludeFileRecordNumber)
			assert.True(t, input.IncludeFileRecordOffset)
			assert.True(t, input.DeleteAfterRead)
			assert.True(t, input.AcquireFSLock)
			assert.True(t, input.FileCacheAdvise)
			assert.True(t, input.SkipUnmodifiedFiles)
			require.NotNil(t, input.Header)
			assert.Equal(t, "^#", input.Header.Pattern)
			require.Len(t, input.Header.MetadataOperators, 1)
			assert.Equal(t, "regex_parser", input.Header.MetadataOperators[0].Type())
			assert.Equal(t, "auto", input.Compression)
			assert.Equal(t, 8, input.PollsToArchive)
			assert.Equal(t, "read_new", input.OnTruncate)
			assert.Equal(t, `(?P<date>\d{4}-\d{2}-\d{2})`, input.OrderingCriteria.Regex)
			assert.Equal(t, `(?P<group>.*)`, input.OrderingCriteria.GroupBy)
			require.NotNil(t, input.OrderingCriteria.TopN)
			assert.Equal(t, 3, *input.OrderingCriteria.TopN)
			assert.Equal(t, []matcher.Sort{{SortType: "timestamp", RegexKey: "date", Layout: "%Y-%m-%d", Location: "UTC", Ascending: true}}, input.OrderingCriteria.SortBy)
			assert.EqualValues(t, "application", input.Attributes["source"])
			assert.EqualValues(t, "example", input.Resource["service.name"])
			require.NotNil(t, cfg.StorageID)
			assert.Equal(t, "file_storage/compatibility", cfg.StorageID.String())
			assert.True(t, cfg.RetryOnFailure.Enabled)
			assert.Equal(t, 2*time.Second, cfg.RetryOnFailure.InitialInterval)
			assert.Equal(t, time.Minute, cfg.RetryOnFailure.MaxInterval)
			assert.Zero(t, cfg.RetryOnFailure.MaxElapsedTime)
			require.Len(t, cfg.Operators, 2)
			assert.Equal(t, "parse", cfg.Operators[0].ID())
			assert.Equal(t, "move_message", cfg.Operators[1].ID())

			core, logs := observer.New(zap.WarnLevel)
			set := receivertest.NewNopSettings(receiverType)
			set.Logger = zap.New(core)
			_, err = factory.CreateLogs(t.Context(), set, cfg, new(consumertest.LogsSink))
			require.NoError(t, err)
			if receiverType == metadata.DeprecatedType {
				require.Equal(t, 1, logs.Len())
				warning := logs.All()[0]
				assert.Equal(t, zap.WarnLevel, warning.Level)
				assert.Contains(t, warning.Message, "file_log")
				assert.Equal(t, "https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/receiver/filelogreceiver/MIGRATION.md#receiver-type-name", warning.ContextMap()["migration_guide"])
			} else {
				assert.Zero(t, logs.Len())
			}
		})
	}
}

func TestGatedConfigCompatibility(t *testing.T) {
	// Exercise the configuration gates at the receiver boundary, not just in
	// the shared matcher. Restore each process-wide gate before the next case.
	for _, tc := range []struct {
		gate   string
		values map[string]any
	}{
		{
			gate:   "filelog.allowFileDeletion",
			values: map[string]any{"delete_after_read": true},
		},
		{
			gate: "filelog.allowHeaderMetadataParsing",
			values: map[string]any{"header": map[string]any{
				"pattern": "^#",
				"metadata_operators": []any{map[string]any{
					"type": "regex_parser", "regex": `^# (?P<source>.*)`,
				}},
			}},
		},
		{
			gate: "filelog.mtimeSortType",
			values: map[string]any{"ordering_criteria": map[string]any{
				"top_n":   0,
				"sort_by": []any{map[string]any{"sort_type": "mtime"}},
			}},
		},
	} {
		t.Run(tc.gate, func(t *testing.T) {
			var gate *featuregate.Gate
			featuregate.GlobalRegistry().VisitAll(func(g *featuregate.Gate) {
				if g.ID() == tc.gate {
					gate = g
				}
			})
			require.NotNil(t, gate)
			originallyEnabled := gate.IsEnabled()
			t.Cleanup(func() {
				require.NoError(t, featuregate.GlobalRegistry().Set(tc.gate, originallyEnabled))
			})
			for _, enabled := range []bool{false, true} {
				t.Run(fmt.Sprintf("enabled=%t", enabled), func(t *testing.T) {
					require.NoError(t, featuregate.GlobalRegistry().Set(tc.gate, enabled))
					for _, useOption := range []bool{false, true} {
						values := map[string]any{"include": []any{"testdata/*.log"}, "start_at": "beginning"}
						if useOption {
							maps.Copy(values, tc.values)
						}
						factory := NewFactory()
						cfg := factory.CreateDefaultConfig()
						require.NoError(t, confmap.NewFromStringMap(values).Unmarshal(cfg))
						_, err := factory.CreateLogs(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, new(consumertest.LogsSink))
						if useOption && !enabled {
							require.ErrorContains(t, err, tc.gate)
						} else {
							require.NoError(t, err)
						}
					}
				})
			}
		})
	}
}
