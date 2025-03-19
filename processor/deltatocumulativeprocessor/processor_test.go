// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deltatocumulativeprocessor

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processortest"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"gopkg.in/yaml.v3"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/testing/compare"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/testing/sdktest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/testing/testar"
)

func TestProcessor(t *testing.T) {
	fis, err := os.ReadDir("testdata")
	require.NoError(t, err)

	for _, fi := range fis {
		if !fi.IsDir() {
			continue
		}

		type Stage struct {
			In  pmetric.Metrics `testar:"in,pmetric"`
			Out pmetric.Metrics `testar:"out,pmetric"`

			Sdk sdktest.Spec `testar:"telemetry,sdk"`
		}

		read := func(file string, into *Stage) error {
			return testar.ReadFile(file, into,
				testar.Parser("pmetric", unmarshalMetrics),
				testar.Parser("sdk", sdktest.Unmarshal),
			)
		}

		dir := fi.Name()
		t.Run(dir, func(t *testing.T) {
			file := func(f string) string {
				return filepath.Join("testdata", dir, f)
			}

			ctx := context.Background()
			cfg := config(t, file("config.yaml"))

			sink := new(consumertest.MetricsSink)
			proc, tel := setup(t, cfg, sink)

			stages, _ := filepath.Glob(file("*.test"))
			for _, file := range stages {
				var stage Stage
				err := read(file, &stage)
				require.NoError(t, err)

				sink.Reset()
				err = proc.ConsumeMetrics(ctx, stage.In)
				require.NoError(t, err)

				out := []pmetric.Metrics{stage.Out}
				if diff := compare.Diff(out, sink.AllMetrics()); diff != "" {
					t.Fatal(diff)
				}

				if err := sdktest.Test(stage.Sdk, tel.reader); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func config(t *testing.T, file string) *Config {
	cfg := NewFactory().CreateDefaultConfig().(*Config)
	cm, err := confmaptest.LoadConf(file)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg
	}
	require.NoError(t, err)

	err = cm.Unmarshal(cfg)
	require.NoError(t, err)
	return cfg
}

func setup(tb testing.TB, cfg *Config, next consumer.Metrics) (processor.Metrics, testTelemetry) {
	tb.Helper()
	if cfg == nil {
		cfg = &Config{MaxStale: 0, MaxStreams: math.MaxInt}
	}

	tt := setupTestTelemetry()
	tb.Cleanup(func() {
		assert.NoError(tb, tt.Shutdown(context.Background()))
	})
	proc, err := NewFactory().CreateMetrics(
		context.Background(),
		tt.newSettings(),
		cfg,
		next,
	)
	require.NoError(tb, err)

	return proc, tt
}

func unmarshalMetrics(data []byte, into *pmetric.Metrics) error {
	var tmp any
	if err := yaml.Unmarshal(data, &tmp); err != nil {
		return err
	}
	data, err := json.Marshal(tmp)
	if err != nil {
		return err
	}
	md, err := (&pmetric.JSONUnmarshaler{}).UnmarshalMetrics(data)
	if err != nil {
		return err
	}
	*into = md
	return nil
}

func TestTelemetry(t *testing.T) {
	tt := setupTestTelemetry()

	next := &consumertest.MetricsSink{}
	cfg := createDefaultConfig()

	_, err := NewFactory().CreateMetrics(
		context.Background(),
		tt.newSettings(),
		cfg,
		next,
	)
	require.NoError(t, err)

	var rm metricdata.ResourceMetrics
	require.NoError(t, tt.reader.Collect(context.Background(), &rm))
}

type testTelemetry struct {
	reader        *sdkmetric.ManualReader
	meterProvider *sdkmetric.MeterProvider
}

func setupTestTelemetry() testTelemetry {
	reader := sdkmetric.NewManualReader()
	return testTelemetry{
		reader:        reader,
		meterProvider: sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)),
	}
}

func (tt *testTelemetry) newSettings() processor.Settings {
	set := processortest.NewNopSettings(metadata.Type)
	set.TelemetrySettings.MeterProvider = tt.meterProvider
	return set
}

func (tt *testTelemetry) Shutdown(ctx context.Context) error {
	return tt.meterProvider.Shutdown(ctx)
}
