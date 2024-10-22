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

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"gopkg.in/yaml.v3"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/datatest/compare"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/testar"
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
		}

		read := func(file string, into *Stage) error {
			return testar.ReadFile(file, into,
				testar.Parser("pmetric", unmarshalMetrics),
			)
		}

		dir := fi.Name()
		t.Run(dir, func(t *testing.T) {
			file := func(f string) string {
				return filepath.Join("testdata", dir, f)
			}

			ctx := context.Background()
			cfg := config(t, file("config.yaml"))
			proc, sink := setup(t, cfg)

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

func setup(t *testing.T, cfg *Config) (processor.Metrics, *consumertest.MetricsSink) {
	t.Helper()

	next := &consumertest.MetricsSink{}
	if cfg == nil {
		cfg = &Config{MaxStale: 0, MaxStreams: math.MaxInt}
	}

	proc, err := NewFactory().CreateMetrics(
		context.Background(),
		processortest.NewNopSettings(),
		cfg,
		next,
	)
	require.NoError(t, err)

	return proc, next
}

func unmarshalMetrics(data []byte, into any) error {
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
	*(into.(*pmetric.Metrics)) = md
	return nil
}

func TestTelemetry(t *testing.T) {
	tt := setupTestTelemetry()

	next := &consumertest.MetricsSink{}
	cfg := createDefaultConfig()

	_, err := NewFactory().CreateMetrics(
		context.Background(),
		tt.NewSettings(),
		cfg,
		next,
	)
	require.NoError(t, err)

	var rm metricdata.ResourceMetrics
	if err := tt.reader.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}
}
