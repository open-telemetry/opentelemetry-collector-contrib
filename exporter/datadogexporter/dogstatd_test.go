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

package datadogexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestDogStatsDExporter(t *testing.T) {
	var (
		testNamespace = "test."
		testTags      = []string{"key1:val1", "key2:val2"}
	)

	logger := zap.NewNop()
	cfg := &Config{
		TagsConfig: TagsConfig{
			Tags: testTags,
		},

		Metrics: MetricsConfig{
			Namespace: testNamespace,
			DogStatsD: DogStatsDConfig{
				Endpoint: "localhost:5000",
			},
		},
	}

	exp, err := newDogStatsDExporter(logger, cfg)
	require.NoError(t, err)

	assert.Equal(t, cfg, exp.GetConfig())
	assert.Equal(t, logger, exp.GetLogger())
	assert.Equal(t, testNamespace, exp.client.Namespace)
	assert.Equal(t, testTags, exp.client.Tags)
}

func TestInvalidDogStatsDExporter(t *testing.T) {
	logger := zap.NewNop()

	// The configuration is invalid if no
	// endpoint is set
	cfg := &Config{
		Metrics: MetricsConfig{
			DogStatsD: DogStatsDConfig{
				Endpoint: "",
			},
		},
	}

	_, err := newDogStatsDExporter(logger, cfg)
	require.Error(t, err)
}
