// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileexporter

import (
	"io"
	"testing"
	"time"

	"github.com/DeRuina/timberjack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateMetricsError(t *testing.T) {
	cfg := &Config{
		FormatType: formatTypeJSON,
	}
	e, err := createMetricsExporter(
		t.Context(),
		exportertest.NewNopSettings(metadata.Type),
		cfg)
	require.NoError(t, err)
	err = e.Start(t.Context(), componenttest.NewNopHost())
	assert.Error(t, err)
}

func TestCreateMetrics(t *testing.T) {
	cfg := &Config{
		FormatType: formatTypeJSON,
		Path:       tempFileName(t),
	}
	exp, err := createMetricsExporter(
		t.Context(),
		exportertest.NewNopSettings(metadata.Type),
		cfg)
	assert.NoError(t, err)
	require.NotNil(t, exp)
	assert.NoError(t, exp.Shutdown(t.Context()))
}

func TestCreateTraces(t *testing.T) {
	cfg := &Config{
		FormatType: formatTypeJSON,
		Path:       tempFileName(t),
	}
	exp, err := createTracesExporter(
		t.Context(),
		exportertest.NewNopSettings(metadata.Type),
		cfg)
	assert.NoError(t, err)
	require.NotNil(t, exp)
	assert.NoError(t, exp.Shutdown(t.Context()))
}

func TestCreateTracesError(t *testing.T) {
	cfg := &Config{
		FormatType: formatTypeJSON,
	}
	e, err := createTracesExporter(
		t.Context(),
		exportertest.NewNopSettings(metadata.Type),
		cfg)
	require.NoError(t, err)
	err = e.Start(t.Context(), componenttest.NewNopHost())
	assert.Error(t, err)
}

func TestCreateLogs(t *testing.T) {
	cfg := &Config{
		FormatType: formatTypeJSON,
		Path:       tempFileName(t),
	}
	exp, err := createLogsExporter(
		t.Context(),
		exportertest.NewNopSettings(metadata.Type),
		cfg)
	assert.NoError(t, err)
	require.NotNil(t, exp)
	assert.NoError(t, exp.Shutdown(t.Context()))
}

func TestCreateLogsError(t *testing.T) {
	cfg := &Config{
		FormatType: formatTypeJSON,
	}
	e, err := createLogsExporter(
		t.Context(),
		exportertest.NewNopSettings(metadata.Type),
		cfg)
	require.NoError(t, err)
	err = e.Start(t.Context(), componenttest.NewNopHost())
	assert.Error(t, err)
}

func TestCreateProfiles(t *testing.T) {
	cfg := &Config{
		FormatType: formatTypeJSON,
		Path:       tempFileName(t),
	}
	exp, err := createProfilesExporter(
		t.Context(),
		exportertest.NewNopSettings(metadata.Type),
		cfg)
	assert.NoError(t, err)
	require.NotNil(t, exp)
	assert.NoError(t, exp.Shutdown(t.Context()))
}

func TestCreateProfilesError(t *testing.T) {
	cfg := &Config{
		FormatType: formatTypeJSON,
	}
	e, err := createProfilesExporter(
		t.Context(),
		exportertest.NewNopSettings(metadata.Type),
		cfg)
	require.NoError(t, err)
	err = e.Start(t.Context(), componenttest.NewNopHost())
	assert.Error(t, err)
}

func TestNewFileWriter(t *testing.T) {
	type args struct {
		cfg *Config
	}
	tests := []struct {
		name     string
		args     args
		want     io.WriteCloser
		validate func(*testing.T, *fileWriter)
	}{
		{
			name: "single file",
			args: args{
				cfg: &Config{
					Path:          tempFileName(t),
					FlushInterval: 5 * time.Second,
				},
			},
			validate: func(t *testing.T, writer *fileWriter) {
				assert.Equal(t, 5*time.Second, writer.flushInterval)
				_, ok := writer.file.(*bufferedWriteCloser)
				assert.True(t, ok)
			},
		},
		{
			name: "rotation file",
			args: args{
				cfg: &Config{
					Path: tempFileName(t),
					Rotation: &Rotation{
						MaxBackups: defaultMaxBackups,
					},
				},
			},
			validate: func(t *testing.T, writer *fileWriter) {
				logger, ok := writer.file.(*timberjack.Logger)
				assert.True(t, ok)
				assert.Equal(t, defaultMaxBackups, logger.MaxBackups)
			},
		},
		{
			name: "rotation file with user's configuration",
			args: args{
				cfg: &Config{
					Path: tempFileName(t),
					Rotation: &Rotation{
						MaxMegabytes: 30,
						MaxDays:      100,
						MaxBackups:   3,
						LocalTime:    true,
					},
				},
			},
			validate: func(t *testing.T, writer *fileWriter) {
				logger, ok := writer.file.(*timberjack.Logger)
				assert.True(t, ok)
				assert.Equal(t, 3, logger.MaxBackups)
				assert.Equal(t, 30, logger.MaxSize)
				assert.Equal(t, 100, logger.MaxAge)
				assert.True(t, logger.LocalTime)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newFileWriter(tt.args.cfg.Path, tt.args.cfg.Append, tt.args.cfg.Rotation, tt.args.cfg.FlushInterval, nil)
			defer func() {
				assert.NoError(t, got.file.Close())
			}()
			assert.NoError(t, err)
			tt.validate(t, got)
		})
	}
}
