// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package windowsperfcountersreceiver

import (
	"fmt"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/scraperhelper"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Receivers), 2)

	r0 := cfg.Receivers[config.NewID(typeStr)]
	defaultConfigSingleObject := factory.CreateDefaultConfig()
	defaultConfigSingleObject.(*Config).PerfCounters = []PerfCounterConfig{{Object: "object", Counters: []string{"counter"}}}

	assert.Equal(t, defaultConfigSingleObject, r0)

	r1 := cfg.Receivers[config.NewIDWithName(typeStr, "customname")].(*Config)
	expectedConfig := &Config{
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			ReceiverSettings:   config.NewReceiverSettings(config.NewIDWithName(typeStr, "customname")),
			CollectionInterval: 30 * time.Second,
		},
		PerfCounters: []PerfCounterConfig{
			{
				Object:   "object1",
				Counters: []string{"counter1"},
			},
			{
				Object:   "object2",
				Counters: []string{"counter1", "counter2"},
			},
		},
	}

	assert.Equal(t, expectedConfig, r1)
}

func TestLoadConfig_Error(t *testing.T) {
	type testCase struct {
		name        string
		cfgFile     string
		expectedErr string
	}

	const (
		errorPrefix                   = "receiver \"windowsperfcounters\" has invalid configuration"
		negativeCollectionIntervalErr = "collection_interval must be a positive duration"
		noPerfCountersErr             = "must specify at least one perf counter"
		noObjectNameErr               = "must specify object name for all perf counters"
		noCountersErr                 = `perf counter for object "%s" does not specify any counters`
		emptyInstanceErr              = `perf counter for object "%s" includes an empty instance`
	)

	testCases := []testCase{
		{
			name:        "NegativeCollectionInterval",
			cfgFile:     "config-negative-collection-interval.yaml",
			expectedErr: fmt.Sprintf("%s: %s", errorPrefix, negativeCollectionIntervalErr),
		},
		{
			name:        "NoPerfCounters",
			cfgFile:     "config-noperfcounters.yaml",
			expectedErr: fmt.Sprintf("%s: %s", errorPrefix, noPerfCountersErr),
		},
		{
			name:        "NoObjectName",
			cfgFile:     "config-noobjectname.yaml",
			expectedErr: fmt.Sprintf("%s: %s", errorPrefix, noObjectNameErr),
		},
		{
			name:        "NoCounters",
			cfgFile:     "config-nocounters.yaml",
			expectedErr: fmt.Sprintf("%s: %s", errorPrefix, fmt.Sprintf(noCountersErr, "object")),
		},
		{
			name:        "EmptyInstance",
			cfgFile:     "config-emptyinstance.yaml",
			expectedErr: fmt.Sprintf("%s: %s", errorPrefix, fmt.Sprintf(emptyInstanceErr, "object")),
		},
		{
			name:    "AllErrors",
			cfgFile: "config-allerrors.yaml",
			expectedErr: fmt.Sprintf(
				"%s: [%s; %s; %s; %s]",
				errorPrefix,
				negativeCollectionIntervalErr,
				fmt.Sprintf(emptyInstanceErr, "object"),
				fmt.Sprintf(noCountersErr, "object"),
				noObjectNameErr,
			),
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			factories, err := componenttest.NopFactories()
			require.NoError(t, err)

			factory := NewFactory()
			factories.Receivers[typeStr] = factory
			_, err = configtest.LoadConfigAndValidate(path.Join(".", "testdata", test.cfgFile), factories)

			require.EqualError(t, err, test.expectedErr)
		})
	}
}
