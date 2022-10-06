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

package k8sobjectsreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	sub, err := cm.Sub("k8sobjects")
	require.NoError(t, err)
	require.NoError(t, config.UnmarshalReceiver(sub, cfg))
	require.NotNil(t, cfg)

	err = cfg.Validate()
	require.Error(t, err)

	cfg.makeDiscoveryClient = getMockDiscoveryClient

	err = cfg.Validate()
	assert.NoError(t, err)

	expected := []*K8sObjectsConfig{
		{
			Name:          "pods",
			Mode:          PullMode,
			Interval:      time.Hour,
			FieldSelector: "status.phase=Running",
			LabelSelector: "environment in (production),tier in (frontend)",
			gvr: &schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
		},
		{
			Name:       "events",
			Mode:       WatchMode,
			Namespaces: []string{"default"},
			gvr: &schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "events",
			},
		},
	}
	assert.EqualValues(t, expected, cfg.Objects)

}

func TestValidConfigs(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "invalid_config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	sub, err := cm.Sub("k8sobjects/invalid_resource")
	require.NoError(t, err)
	require.NoError(t, config.UnmarshalReceiver(sub, cfg))

	cfg.makeDiscoveryClient = getMockDiscoveryClient

	err = cfg.Validate()
	assert.ErrorContains(t, err, "resource fake_resource not found")

}
