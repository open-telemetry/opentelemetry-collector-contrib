// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileexporter

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"gopkg.in/natefinch/lumberjack.v2"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateMetricsExporterError(t *testing.T) {
	cfg := &Config{
		FormatType: formatTypeJSON,
	}
	e, err := createMetricsExporter(
		context.Background(),
		exportertest.NewNopSettings(),
		cfg)
	require.NoError(t, err)
	err = e.Start(context.Background(), componenttest.NewNopHost())
	assert.Error(t, err)
}

func TestCreateMetricsExporter(t *testing.T) {
	cfg := &Config{
		FormatType: formatTypeJSON,
		Path:       tempFileName(t),
	}
	exp, err := createMetricsExporter(
		context.Background(),
		exportertest.NewNopSettings(),
		cfg)
	assert.NoError(t, err)
	require.NotNil(t, exp)
	assert.NoError(t, exp.Shutdown(context.Background()))
}

func TestCreateTracesExporter(t *testing.T) {
	cfg := &Config{
		FormatType: formatTypeJSON,
		Path:       tempFileName(t),
	}
	exp, err := createTracesExporter(
		context.Background(),
		exportertest.NewNopSettings(),
		cfg)
	assert.NoError(t, err)
	require.NotNil(t, exp)
	assert.NoError(t, exp.Shutdown(context.Background()))
}

func TestCreateTracesExporterError(t *testing.T) {
	cfg := &Config{
		FormatType: formatTypeJSON,
	}
	e, err := createTracesExporter(
		context.Background(),
		exportertest.NewNopSettings(),
		cfg)
	require.NoError(t, err)
	err = e.Start(context.Background(), componenttest.NewNopHost())
	assert.Error(t, err)
}

func TestCreateLogsExporter(t *testing.T) {
	cfg := &Config{
		FormatType: formatTypeJSON,
		Path:       tempFileName(t),
	}
	exp, err := createLogsExporter(
		context.Background(),
		exportertest.NewNopSettings(),
		cfg)
	assert.NoError(t, err)
	require.NotNil(t, exp)
	assert.NoError(t, exp.Shutdown(context.Background()))
}

func TestCreateLogsExporterError(t *testing.T) {
	cfg := &Config{
		FormatType: formatTypeJSON,
	}
	e, err := createLogsExporter(
		context.Background(),
		exportertest.NewNopSettings(),
		cfg)
	require.NoError(t, err)
	err = e.Start(context.Background(), componenttest.NewNopHost())
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
				assert.Equal(t, true, ok)
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
				logger, ok := writer.file.(*lumberjack.Logger)
				assert.Equal(t, true, ok)
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
				logger, ok := writer.file.(*lumberjack.Logger)
				assert.Equal(t, true, ok)
				assert.Equal(t, 3, logger.MaxBackups)
				assert.Equal(t, 30, logger.MaxSize)
				assert.Equal(t, 100, logger.MaxAge)
				assert.Equal(t, true, logger.LocalTime)
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
