// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deltatocumulativeprocessor_test

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
	"gopkg.in/yaml.v3"

	self "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor"
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

		ar := reader()

		type Stage struct {
			In  pmetric.Metrics `testar:"in,pmetric"`
			Out pmetric.Metrics `testar:"out,pmetric"`
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
				err := ar.ReadFile(file, &stage)
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

func config(t *testing.T, file string) *self.Config {
	cfg := self.NewFactory().CreateDefaultConfig().(*self.Config)
	cm, err := confmaptest.LoadConf(file)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg
	}
	require.NoError(t, err)

	err = cm.Unmarshal(cfg)
	require.NoError(t, err)
	return cfg
}

func reader() testar.Reader {
	return testar.Reader{
		Formats: testar.Formats{
			"pmetric": func(data []byte, into any) error {
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
			},
		},
	}
}

func setup(t *testing.T, cfg *self.Config) (processor.Metrics, *consumertest.MetricsSink) {
	t.Helper()

	next := &consumertest.MetricsSink{}
	if cfg == nil {
		cfg = &self.Config{MaxStale: 0, MaxStreams: math.MaxInt}
	}

	proc, err := self.NewFactory().CreateMetrics(
		context.Background(),
		processortest.NewNopSettings(),
		cfg,
		next,
	)
	require.NoError(t, err)

	return proc, next
}
