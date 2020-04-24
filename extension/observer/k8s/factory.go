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

package k8s

import (
	"context"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector/component"
	"github.com/open-telemetry/opentelemetry-collector/config/configmodels"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

const (
	// The value of extension "type" in configuration.
	typeStr = "k8s_observer"
)

// Factory is the factory for the extension.
type Factory struct {
	createK8sConfig func() (*rest.Config, error)
}

var _ (component.Factory) = (*Factory)(nil)

// Type gets the type of the config created by this factory.
func (f *Factory) Type() string {
	return typeStr
}

// CreateDefaultConfig creates the default configuration for the extension.
func (f *Factory) CreateDefaultConfig() configmodels.Extension {
	return &Config{
		ExtensionSettings: configmodels.ExtensionSettings{
			TypeVal: typeStr,
			NameVal: typeStr,
		},
	}
}

// CreateExtension creates the extension based on this config.
func (f *Factory) CreateExtension(
	ctx context.Context,
	params component.ExtensionCreateParams,
	cfg configmodels.Extension,
) (component.ServiceExtension, error) {
	config := cfg.(*Config)

	restConfig, err := f.createK8sConfig()
	if err != nil {
		return nil, fmt.Errorf("failed creating Kubernetes in-cluster REST config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
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
	return &Factory{createK8sConfig: func() (*rest.Config, error) {
		restConfig, err := rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
		return restConfig, nil
	}}
}

// NewFactoryWithConfig creates a k8s observer factory with the given k8s API config.
func NewFactoryWithConfig(config *rest.Config) *Factory {
	return &Factory{createK8sConfig: func() (*rest.Config, error) {
		return config, nil
	}}
}
