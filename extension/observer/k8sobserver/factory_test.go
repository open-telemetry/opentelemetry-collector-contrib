// Copyright 2019, OpenTelemetry Authors
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

package k8sobserver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

func TestFactory_Type(t *testing.T) {
	factory := Factory{}
	require.Equal(t, typeStr, factory.Type())
}

var nilClient = func(k8sconfig.APIConfig) (kubernetes.Interface, error) {
	return &kubernetes.Clientset{}, nil
}

func TestFactory_CreateDefaultConfig(t *testing.T) {
	factory := Factory{createK8sClientset: nilClient}
	cfg := factory.CreateDefaultConfig()
	assert.Equal(t, &Config{
		ExtensionSettings: config.NewExtensionSettings(config.NewComponentID(typeStr)),
		APIConfig:         k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
	},
		cfg)

	assert.NoError(t, configtest.CheckConfigStruct(cfg))
	ext, err := factory.CreateExtension(context.Background(), componenttest.NewNopExtensionCreateSettings(), cfg)
	require.NoError(t, err)
	require.NotNil(t, ext)
}

func TestFactory_CreateExtension(t *testing.T) {
	factory := Factory{createK8sClientset: nilClient}
	cfg := factory.CreateDefaultConfig().(*Config)

	ext, err := factory.CreateExtension(context.Background(), componenttest.NewNopExtensionCreateSettings(), cfg)
	require.NoError(t, err)
	require.NotNil(t, ext)
}

func TestNewFactory(t *testing.T) {
	f := NewFactory()
	require.IsType(t, f, &Factory{})
	ff := f.(*Factory)
	cs, err := ff.createK8sClientset(k8sconfig.APIConfig{AuthType: "none"})
	assert.Error(t, err)
	assert.Nil(t, cs)

	rc := &rest.Config{Host: "fake-host"}
	f = NewFactoryWithConfig(rc)
	require.IsType(t, f, &Factory{})
	ff = f.(*Factory)
	cs, err = ff.createK8sClientset(k8sconfig.APIConfig{})
	assert.NoError(t, err)
	require.NotNil(t, cs)
	rq := cs.AuthenticationV1().RESTClient().Get()
	assert.Equal(t, "http://fake-host/apis/authentication.k8s.io/v1", rq.URL().String())
}
