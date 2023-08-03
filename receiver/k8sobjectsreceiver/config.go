// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobjectsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sobjectsreceiver"

import (
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

type mode string

const (
	PullMode  mode = "pull"
	WatchMode mode = "watch"

	defaultPullInterval    time.Duration = time.Hour
	defaultMode            mode          = PullMode
	defaultResourceVersion               = "1"
)

var modeMap = map[mode]bool{
	PullMode:  true,
	WatchMode: true,
}

type K8sObjectsConfig struct {
	Name            string        `mapstructure:"name"`
	Group           string        `mapstructure:"group"`
	Namespaces      []string      `mapstructure:"namespaces"`
	Mode            mode          `mapstructure:"mode"`
	LabelSelector   string        `mapstructure:"label_selector"`
	FieldSelector   string        `mapstructure:"field_selector"`
	Interval        time.Duration `mapstructure:"interval"`
	ResourceVersion string        `mapstructure:"resource_version"`
	gvr             *schema.GroupVersionResource
}

type Config struct {
	k8sconfig.APIConfig `mapstructure:",squash"`

	Objects []*K8sObjectsConfig `mapstructure:"objects"`

	// For mocking purposes only.
	makeDiscoveryClient func() (discovery.ServerResourcesInterface, error)
	makeDynamicClient   func() (dynamic.Interface, error)
}

func (c *Config) Validate() error {

	validObjects, err := c.getValidObjects()
	if err != nil {
		return err
	}
	for _, object := range c.Objects {
		gvrs, ok := validObjects[object.Name]
		if !ok {
			availableResource := make([]string, len(validObjects))
			for k := range validObjects {
				availableResource = append(availableResource, k)
			}
			return fmt.Errorf("resource %v not found. Valid resources are: %v", object.Name, availableResource)
		}

		gvr := gvrs[0]
		for i := range gvrs {
			if gvrs[i].Group == object.Group {
				gvr = gvrs[i]
				break
			}
		}

		if object.Mode == "" {
			object.Mode = defaultMode
		} else if _, ok := modeMap[object.Mode]; !ok {
			return fmt.Errorf("invalid mode: %v", object.Mode)
		}

		if object.Mode == PullMode && object.Interval == 0 {
			object.Interval = defaultPullInterval
		}

		if object.Mode == WatchMode && object.ResourceVersion == "" {
			object.ResourceVersion = defaultResourceVersion
		}

		object.gvr = gvr
	}
	return nil
}

func (c *Config) getDiscoveryClient() (discovery.ServerResourcesInterface, error) {
	if c.makeDiscoveryClient != nil {
		return c.makeDiscoveryClient()
	}

	client, err := k8sconfig.MakeClient(c.APIConfig)
	if err != nil {
		return nil, err
	}

	return client.Discovery(), nil
}

func (c *Config) getDynamicClient() (dynamic.Interface, error) {
	if c.makeDynamicClient != nil {
		return c.makeDynamicClient()
	}

	return k8sconfig.MakeDynamicClient(c.APIConfig)
}

func (c *Config) getValidObjects() (map[string][]*schema.GroupVersionResource, error) {
	dc, err := c.getDiscoveryClient()
	if err != nil {
		return nil, err
	}

	res, err := dc.ServerPreferredResources()
	if err != nil {
		return nil, err
	}

	validObjects := make(map[string][]*schema.GroupVersionResource)

	for _, group := range res {
		split := strings.Split(group.GroupVersion, "/")
		if len(split) == 1 && group.GroupVersion == "v1" {
			split = []string{"", "v1"}
		}
		for _, resource := range group.APIResources {
			validObjects[resource.Name] = append(validObjects[resource.Name], &schema.GroupVersionResource{
				Group:    split[0],
				Version:  split[1],
				Resource: resource.Name,
			})
		}

	}
	return validObjects, nil
}
