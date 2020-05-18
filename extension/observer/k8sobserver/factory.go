// Copyright 2020, OpenTelemetry Authors
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

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/k8sconfig"
)

const (
	// The value of extension "type" in configuration.
	typeStr configmodels.Type = "k8s_observer"
)

// Factory is the factory for the extension.
type Factory struct {
	// createK8sClientset being a field in the struct provides an easy way
	// to mock k8s config in tests.
	createK8sClientset func(config k8sconfig.APIConfig) (*kubernetes.Clientset, error)
}

var _ component.Factory = (*Factory)(nil)

// Type gets the type of the config created by this factory.
func (f *Factory) Type() configmodels.Type {
	return typeStr
}

// CreateDefaultConfig creates the default configuration for the extension.
func (f *Factory) CreateDefaultConfig() configmodels.Extension {
	return &Config{
		ExtensionSettings: configmodels.ExtensionSettings{
			TypeVal: typeStr,
			NameVal: string(typeStr),
		},
		APIConfig: k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
	}
}

// CreateExtension creates the extension based on this config.
func (f *Factory) CreateExtension(
	ctx context.Context,
	params component.ExtensionCreateParams,
	cfg configmodels.Extension,
) (component.ServiceExtension, error) {
	config := cfg.(*Config)

	clientset, err := f.createK8sClientset(config.APIConfig)
	if err != nil {
		return nil, err
	}

	listWatch := cache.NewListWatchFromClient(
		clientset.CoreV1().RESTClient(), "pods", v1.NamespaceAll,
		fields.OneTermEqualSelector("spec.nodeName", config.Node))

	return newObserver(params.Logger, config, listWatch)
}

// NewFactory should be called to create a factory with default values.
func NewFactory() component.ExtensionFactory {
	return &Factory{createK8sClientset: func(config k8sconfig.APIConfig) (*kubernetes.Clientset, error) {
		return k8sconfig.MakeClient(config)
	}}
}

// NewFactoryWithConfig creates a k8s observer factory with the given k8s API config.
func NewFactoryWithConfig(config *rest.Config) *Factory {
	return &Factory{createK8sClientset: func(k8sconfig.APIConfig) (*kubernetes.Clientset, error) {
		return kubernetes.NewForConfig(config)
	}}
}
