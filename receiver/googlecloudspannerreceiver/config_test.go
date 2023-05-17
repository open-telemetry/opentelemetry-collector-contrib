// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudspannerreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"
)

const (
	cardinalityLimit = 200_000
)

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	assert.Equal(t,
		&Config{
			ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
				CollectionInterval: 120 * time.Second,
			},
			TopMetricsQueryMaxRows:            10,
			BackfillEnabled:                   true,
			CardinalityTotalLimit:             200000,
			HideTopnLockstatsRowrangestartkey: true,
			TruncateText:                      true,
			Projects: []Project{
				{
					ID:                "spanner project 1",
					ServiceAccountKey: "path to spanner project 1 service account json key",
					Instances: []Instance{
						{
							ID:        "id1",
							Databases: []string{"db11", "db12"},
						},
						{
							ID:        "id2",
							Databases: []string{"db21", "db22"},
						},
					},
				},
				{
					ID:                "spanner project 2",
					ServiceAccountKey: "path to spanner project 2 service account json key",
					Instances: []Instance{
						{
							ID:        "id3",
							Databases: []string{"db31", "db32"},
						},
						{
							ID:        "id4",
							Databases: []string{"db41", "db42"},
						},
					},
				},
			},
		},
		cfg,
	)
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
		"No service account key":            {"id", "", []Instance{instance}, false},
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
		collectionInterval      time.Duration
		topMetricsQueryMaxRows  int
		cardinalityOverallLimit int
		projects                []Project
		requireError            bool
	}{
		"All required fields are populated":                   {defaultCollectionInterval, defaultTopMetricsQueryMaxRows, cardinalityLimit, []Project{project}, false},
		"Invalid collection interval":                         {-1, defaultTopMetricsQueryMaxRows, cardinalityLimit, []Project{project}, true},
		"Invalid top metrics query max rows":                  {defaultCollectionInterval, -1, cardinalityLimit, []Project{project}, true},
		"Top metrics query max rows greater than max allowed": {defaultCollectionInterval, defaultTopMetricsQueryMaxRows + 1, cardinalityLimit, []Project{project}, true},
		"No projects":                           {defaultCollectionInterval, defaultTopMetricsQueryMaxRows, cardinalityLimit, nil, true},
		"Invalid project in projects":           {defaultCollectionInterval, defaultTopMetricsQueryMaxRows, cardinalityLimit, []Project{{}}, true},
		"Cardinality overall limit is zero":     {defaultCollectionInterval, defaultTopMetricsQueryMaxRows, 0, []Project{project}, false},
		"Cardinality overall limit is negative": {defaultCollectionInterval, defaultTopMetricsQueryMaxRows, -cardinalityLimit, []Project{project}, true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			cfg := &Config{
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
					CollectionInterval: testCase.collectionInterval,
				},
				TopMetricsQueryMaxRows: testCase.topMetricsQueryMaxRows,
				CardinalityTotalLimit:  testCase.cardinalityOverallLimit,
				Projects:               testCase.projects,
			}

			err := component.ValidateConfig(cfg)

			if testCase.requireError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
