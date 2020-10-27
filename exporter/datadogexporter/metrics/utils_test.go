// Copyright The OpenTelemetry Authors
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

package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"gopkg.in/zorkian/go-datadog-api.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/utils/cache"
)

func TestNewGauge(t *testing.T) {
	name := "test.metric"
	ts := uint64(1e9)
	value := 2.0
	tags := []string{"tag:value"}

	metric := NewGauge(name, ts, value, tags)

	assert.Equal(t, "test.metric", *metric.Metric)
	// Assert timestamp conversion from uint64 ns to float64 s
	assert.Equal(t, 1.0, *metric.Points[0][0])
	// Assert value
	assert.Equal(t, 2.0, *metric.Points[0][1])
	// Assert tags
	assert.Equal(t, []string{"tag:value"}, metric.Tags)
}

func TestRunningMetric(t *testing.T) {
	logger := zap.NewNop()
	cfg := &config.Config{}

	ms := RunningMetric("metrics", uint64(2e9), logger, cfg)

	assert.Equal(t, "otel.datadog_exporter.metrics.running", *ms[0].Metric)
	// Assert metrics list length (should be 1)
	assert.Equal(t, 1, len(ms))
	// Assert timestamp
	assert.Equal(t, 2.0, *ms[0].Points[0][0])
	// Assert value (should always be 1.0)
	assert.Equal(t, 1.0, *ms[0].Points[0][1])

	// Test that the namespace set in config does not get used: the running
	// metric needs to follow a specific format
	cfg = &config.Config{
		Metrics: config.MetricsConfig{
			Namespace: "test.",
		},
	}

	ms = RunningMetric("metrics", uint64(2e9), logger, cfg)

	// Check that the namespace isn't used
	assert.Equal(t, "otel.datadog_exporter.metrics.running", *ms[0].Metric)
	// Assert metrics list length (should be 1)
	assert.Equal(t, 1, len(ms))
	// Assert timestamp
	assert.Equal(t, 2.0, *ms[0].Points[0][0])
	// Assert value (should always be 1.0)
	assert.Equal(t, 1.0, *ms[0].Points[0][1])
}

func TestAddHostname(t *testing.T) {
	logger := zap.NewNop()

	// Reset hostname cache
	cache.Cache.Flush()

	// With hostname in config
	cfg := &config.Config{
		TagsConfig: config.TagsConfig{
			Hostname: "thishost",
		},
	}

	ms := []datadog.Metric{
		NewGauge("test.metric", 0, 1.0, []string{}),
		NewGauge("test.metric2", 0, 2.0, []string{}),
	}

	hostname := "thathost"

	ms[0].Host = &hostname

	AddHostname(ms, logger, cfg)

	// Check that all hostnames are set to the config's hostname
	assert.Equal(t, "thishost", *ms[0].Host)
	assert.Equal(t, "thishost", *ms[1].Host)

	// Reset hostname cache
	cache.Cache.Flush()

	// Without hostname in config
	cfg = &config.Config{}

	ms = []datadog.Metric{
		NewGauge("test.metric", 0, 1.0, []string{}),
	}

	ms[0].Host = &hostname

	AddHostname(ms, logger, cfg)

	// Check that the already set host remains set
	assert.Equal(t, "thathost", *ms[0].Host)
}

func TestAddNamespace(t *testing.T) {
	ms := []datadog.Metric{
		NewGauge("test.metric", 0, 1.0, []string{}),
		NewGauge("test.metric2", 0, 2.0, []string{}),
	}

	AddNamespace(ms, "namespace.")

	assert.Equal(t, "namespace.test.metric", *ms[0].Metric)
	assert.Equal(t, "namespace.test.metric2", *ms[1].Metric)
}
