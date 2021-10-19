// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package googlecloudspannerreceiver

import (
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Receivers), 1)

	receiver := cfg.Receivers[config.NewComponentID(typeStr)].(*Config)

	assert.Equal(t, 120*time.Second, receiver.CollectionInterval)
	assert.Equal(t, 10, receiver.TopMetricsQueryMaxRows)
	assert.True(t, receiver.BackfillEnabled)

	assert.Equal(t, 2, len(receiver.Projects))

	assert.Equal(t, "spanner project 1", receiver.Projects[0].ID)
	assert.Equal(t, "path to spanner project 1 service account json key", receiver.Projects[0].ServiceAccountKey)
	assert.Equal(t, 2, len(receiver.Projects[0].Instances))

	assert.Equal(t, "id1", receiver.Projects[0].Instances[0].ID)
	assert.Equal(t, 2, len(receiver.Projects[0].Instances[0].Databases))
	assert.Equal(t, "db11", receiver.Projects[0].Instances[0].Databases[0])
	assert.Equal(t, "db12", receiver.Projects[0].Instances[0].Databases[1])
	assert.Equal(t, "id2", receiver.Projects[0].Instances[1].ID)
	assert.Equal(t, 2, len(receiver.Projects[0].Instances[1].Databases))
	assert.Equal(t, "db21", receiver.Projects[0].Instances[1].Databases[0])
	assert.Equal(t, "db22", receiver.Projects[0].Instances[1].Databases[1])

	assert.Equal(t, "spanner project 2", receiver.Projects[1].ID)
	assert.Equal(t, "path to spanner project 2 service account json key", receiver.Projects[1].ServiceAccountKey)
	assert.Equal(t, len(receiver.Projects[1].Instances), 2)

	assert.Equal(t, "id3", receiver.Projects[1].Instances[0].ID)
	assert.Equal(t, 2, len(receiver.Projects[1].Instances[0].Databases))
	assert.Equal(t, "db31", receiver.Projects[1].Instances[0].Databases[0])
	assert.Equal(t, "db32", receiver.Projects[1].Instances[0].Databases[1])
	assert.Equal(t, "id4", receiver.Projects[1].Instances[1].ID)
	assert.Equal(t, 2, len(receiver.Projects[1].Instances[1].Databases))
	assert.Equal(t, "db41", receiver.Projects[1].Instances[1].Databases[0])
	assert.Equal(t, "db42", receiver.Projects[1].Instances[1].Databases[1])
}

func TestValidateInstance(t *testing.T) {
	testCases := map[string]struct {
		id           string
		databases    []string
		requireError bool
	}{
		"All required fields are populated": {"id", []string{"name"}, false},
		"No id":                             {"", []string{"name"}, true},
		"No databases":                      {"id", nil, true},
		"Databases have empty names":        {"id", []string{""}, true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			instance := Instance{
				ID:        testCase.id,
				Databases: testCase.databases,
			}

			err := instance.Validate()

			if testCase.requireError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateProject(t *testing.T) {
	instance := Instance{
		ID:        "id",
		Databases: []string{"name"},
	}

	testCases := map[string]struct {
		id                string
		serviceAccountKey string
		instances         []Instance
		requireError      bool
	}{
		"All required fields are populated": {"id", "key", []Instance{instance}, false},
		"No id":                             {"", "key", []Instance{instance}, true},
		"No service account key":            {"id", "", []Instance{instance}, true},
		"No instances":                      {"id", "key", nil, true},
		"Invalid instance in instances":     {"id", "key", []Instance{{}}, true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			project := Project{
				ID:                testCase.id,
				ServiceAccountKey: testCase.serviceAccountKey,
				Instances:         testCase.instances,
			}

			err := project.Validate()

			if testCase.requireError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateConfig(t *testing.T) {
	instance := Instance{
		ID:        "id",
		Databases: []string{"name"},
	}

	project := Project{
		ID:                "id",
		ServiceAccountKey: "key",
		Instances:         []Instance{instance},
	}

	testCases := map[string]struct {
		collectionInterval     time.Duration
		topMetricsQueryMaxRows int
		projects               []Project
		requireError           bool
	}{
		"All required fields are populated":                   {defaultCollectionInterval, defaultTopMetricsQueryMaxRows, []Project{project}, false},
		"Invalid collection interval":                         {-1, defaultTopMetricsQueryMaxRows, []Project{project}, true},
		"Invalid top metrics query max rows":                  {defaultCollectionInterval, -1, []Project{project}, true},
		"Top metrics query max rows greater than max allowed": {defaultCollectionInterval, defaultTopMetricsQueryMaxRows + 1, []Project{project}, true},
		"No projects":                 {defaultCollectionInterval, defaultTopMetricsQueryMaxRows, nil, true},
		"Invalid project in projects": {defaultCollectionInterval, defaultTopMetricsQueryMaxRows, []Project{{}}, true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			cfg := &Config{
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
					ReceiverSettings:   config.NewReceiverSettings(config.NewComponentID(typeStr)),
					CollectionInterval: testCase.collectionInterval,
				},
				TopMetricsQueryMaxRows: testCase.topMetricsQueryMaxRows,
				Projects:               testCase.projects,
			}

			err := cfg.Validate()

			if testCase.requireError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
