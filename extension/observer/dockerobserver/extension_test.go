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

package dockerobserver

import (
	"encoding/json"
	"io/ioutil"
	"path"
	"testing"

	dtypes "github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

func containerJSON(t *testing.T) dtypes.ContainerJSON {
	containerRaw, err := ioutil.ReadFile(path.Join(".", "testdata", "container.json"))
	require.NoError(t, err)

	var container dtypes.ContainerJSON
	err = json.Unmarshal(containerRaw, &container)
	if err != nil {
		t.Fatal(err)
	}
	return container
}

func TestPortTypeToProtocol(t *testing.T) {
	tests := []struct {
		name string
		want observer.Transport
	}{
		{
			name: "tcp",
			want: observer.ProtocolTCP,
		},
		{
			name: "udp",
			want: observer.ProtocolUDP,
		},
		{
			name: "unsupported",
			want: observer.ProtocolUnknown,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, portProtoToTransport(tt.name))
		})
	}
}

func TestCollectEndpointsDefaultConfig(t *testing.T) {
	factory := NewFactory()
	ext, err := newObserver(zap.NewNop(), factory.CreateDefaultConfig().(*Config))
	require.NoError(t, err)
	require.NotNil(t, ext)

	obvs, ok := ext.(*dockerObserver)
	require.True(t, ok)

	c := containerJSON(t)
	cEndpoints := obvs.endpointsForContainer(&c)

	want := []observer.Endpoint{
		{
			ID:     "babc5a6d7af2a48e7f52e1da26047024dcf98b737e754c9c3459bb84d1e4f80c:8080",
			Target: "172.17.0.2",
			Details: &observer.Container{
				Name:        "/agitated_wu",
				Image:       "nginx",
				Command:     "nginx -g daemon off;",
				ContainerID: "babc5a6d7af2a48e7f52e1da26047024dcf98b737e754c9c3459bb84d1e4f80c",
				Transport:   observer.ProtocolTCP,
				Labels: map[string]string{
					"hello":      "world",
					"maintainer": "NGINX Docker Maintainers",
					"mstumpf":    "",
				},
				Port:          80,
				AlternatePort: 8080,
				Host:          "172.17.0.2",
			},
		},
	}

	require.Equal(t, cEndpoints, want)
}

func TestCollectEndpointsAllConfigSettings(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Extensions[typeStr] = factory
	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)

	require.Nil(t, err)
	require.NotNil(t, cfg)

	extAllSettings := cfg.Extensions[config.NewComponentIDWithName(typeStr, "all_settings")]

	ext, err := newObserver(zap.NewNop(), extAllSettings.(*Config))
	require.NoError(t, err)
	require.NotNil(t, ext)

	obvs := ext.(*dockerObserver)

	c := containerJSON(t)
	cEndpoints := obvs.endpointsForContainer(&c)

	want := []observer.Endpoint{
		{
			ID:     "babc5a6d7af2a48e7f52e1da26047024dcf98b737e754c9c3459bb84d1e4f80c:8080",
			Target: "127.0.0.1",
			Details: &observer.Container{
				Name:        "/agitated_wu",
				Image:       "nginx",
				Command:     "nginx -g daemon off;",
				ContainerID: "babc5a6d7af2a48e7f52e1da26047024dcf98b737e754c9c3459bb84d1e4f80c",
				Transport:   observer.ProtocolTCP,
				Labels: map[string]string{
					"hello":      "world",
					"maintainer": "NGINX Docker Maintainers",
					"mstumpf":    "",
				},
				Port:          8080,
				AlternatePort: 80,
				Host:          "127.0.0.1",
			},
		},
	}

	require.Equal(t, cEndpoints, want)
}

func TestCollectEndpointsUseHostnameIfPresent(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Extensions[typeStr] = factory
	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)

	require.Nil(t, err)
	require.NotNil(t, cfg)

	extUseHostname := cfg.Extensions[config.NewComponentIDWithName(typeStr, "use_hostname_if_present")]

	ext, err := newObserver(zap.NewNop(), extUseHostname.(*Config))
	require.NoError(t, err)
	require.NotNil(t, ext)

	obvs := ext.(*dockerObserver)

	c := containerJSON(t)
	cEndpoints := obvs.endpointsForContainer(&c)

	want := []observer.Endpoint{
		{
			ID:     "babc5a6d7af2a48e7f52e1da26047024dcf98b737e754c9c3459bb84d1e4f80c:8080",
			Target: "babc5a6d7af2",
			Details: &observer.Container{
				Name:        "/agitated_wu",
				Image:       "nginx",
				Command:     "nginx -g daemon off;",
				ContainerID: "babc5a6d7af2a48e7f52e1da26047024dcf98b737e754c9c3459bb84d1e4f80c",
				Transport:   observer.ProtocolTCP,
				Labels: map[string]string{
					"hello":      "world",
					"maintainer": "NGINX Docker Maintainers",
					"mstumpf":    "",
				},
				Port:          80,
				AlternatePort: 8080,
				Host:          "babc5a6d7af2",
			},
		},
	}

	require.Equal(t, cEndpoints, want)
}

func TestCollectEndpointsUseHostBindings(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Extensions[typeStr] = factory
	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)

	require.Nil(t, err)
	require.NotNil(t, cfg)

	extHostBindings := cfg.Extensions[config.NewComponentIDWithName(typeStr, "use_host_bindings")]

	ext, err := newObserver(zap.NewNop(), extHostBindings.(*Config))
	require.NoError(t, err)
	require.NotNil(t, ext)

	obvs := ext.(*dockerObserver)

	c := containerJSON(t)
	cEndpoints := obvs.endpointsForContainer(&c)

	want := []observer.Endpoint{
		{
			ID:     "babc5a6d7af2a48e7f52e1da26047024dcf98b737e754c9c3459bb84d1e4f80c:8080",
			Target: "127.0.0.1",
			Details: &observer.Container{
				Name:        "/agitated_wu",
				Image:       "nginx",
				Command:     "nginx -g daemon off;",
				ContainerID: "babc5a6d7af2a48e7f52e1da26047024dcf98b737e754c9c3459bb84d1e4f80c",
				Transport:   observer.ProtocolTCP,
				Labels: map[string]string{
					"hello":      "world",
					"maintainer": "NGINX Docker Maintainers",
					"mstumpf":    "",
				},
				Port:          8080,
				AlternatePort: 80,
				Host:          "127.0.0.1",
			},
		},
	}

	require.Equal(t, cEndpoints, want)
}

func TestCollectEndpointsIgnoreNonHostBindings(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Extensions[typeStr] = factory
	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)

	require.Nil(t, err)
	require.NotNil(t, cfg)

	extIgnoreHostBindings := cfg.Extensions[config.NewComponentIDWithName(typeStr, "ignore_non_host_bindings")]

	ext, err := newObserver(zap.NewNop(), extIgnoreHostBindings.(*Config))
	require.NoError(t, err)
	require.NotNil(t, ext)

	obvs := ext.(*dockerObserver)

	c := containerJSON(t)
	cEndpoints := obvs.endpointsForContainer(&c)

	want := []observer.Endpoint{
		{
			ID:     "babc5a6d7af2a48e7f52e1da26047024dcf98b737e754c9c3459bb84d1e4f80c:8080",
			Target: "172.17.0.2",
			Details: &observer.Container{
				Name:        "/agitated_wu",
				Image:       "nginx",
				Command:     "nginx -g daemon off;",
				ContainerID: "babc5a6d7af2a48e7f52e1da26047024dcf98b737e754c9c3459bb84d1e4f80c",
				Transport:   observer.ProtocolTCP,
				Labels: map[string]string{
					"hello":      "world",
					"maintainer": "NGINX Docker Maintainers",
					"mstumpf":    "",
				},
				Port:          80,
				AlternatePort: 8080,
				Host:          "172.17.0.2",
			},
		},
	}

	require.Equal(t, cEndpoints, want)
}
