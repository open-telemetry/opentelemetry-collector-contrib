// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobjectsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sobjectsreceiver"

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apiWatch "k8s.io/apimachinery/pkg/watch"
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

type ErrorMode string

const (
	PropagateError ErrorMode = "propagate"
	IgnoreError    ErrorMode = "ignore"
	SilentError    ErrorMode = "silent"
)

type K8sObjectsConfig struct {
	Name             string               `mapstructure:"name"`
	Group            string               `mapstructure:"group"`
	Namespaces       []string             `mapstructure:"namespaces"`
	Mode             mode                 `mapstructure:"mode"`
	LabelSelector    string               `mapstructure:"label_selector"`
	FieldSelector    string               `mapstructure:"field_selector"`
	Interval         time.Duration        `mapstructure:"interval"`
	ResourceVersion  string               `mapstructure:"resource_version"`
	ExcludeWatchType []apiWatch.EventType `mapstructure:"exclude_watch_type"`
	exclude          map[apiWatch.EventType]bool
	gvr              *schema.GroupVersionResource
}

type Config struct {
	k8sconfig.APIConfig `mapstructure:",squash"`

	Objects   []*K8sObjectsConfig `mapstructure:"objects"`
	ErrorMode ErrorMode           `mapstructure:"error_mode"`

	K8sLeaderElector *component.ID `mapstructure:"k8s_leader_elector"`

	// For mocking purposes only.
	makeDiscoveryClient func() (discovery.ServerResourcesInterface, error)
	makeDynamicClient   func() (dynamic.Interface, error)
}

func (c *Config) Validate() error {
	switch c.ErrorMode {
	case PropagateError, IgnoreError, SilentError:
	default:
		return fmt.Errorf("invalid error_mode %q: must be one of 'propagate', 'ignore', or 'silent'", c.ErrorMode)
	}

	for _, object := range c.Objects {
		if object.Mode == "" {
			object.Mode = defaultMode
		} else if _, ok := modeMap[object.Mode]; !ok {
			return fmt.Errorf("invalid mode: %v", object.Mode)
		}

		if object.Mode == PullMode && object.Interval == 0 {
			object.Interval = defaultPullInterval
		}

		if object.Mode == PullMode && len(object.ExcludeWatchType) != 0 {
			return errors.New("the Exclude config can only be used with watch mode")
		}
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
		// Check if Partial result is returned from discovery client, that means some API servers have issues,
		// but we can still continue, as we check for the needed groups later in Validate function.
		if res != nil && !discovery.IsGroupDiscoveryFailedError(err) {
			return nil, err
		}
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

func (k *K8sObjectsConfig) DeepCopy() *K8sObjectsConfig {
	copied := &K8sObjectsConfig{
		Name:            k.Name,
		Group:           k.Group,
		Mode:            k.Mode,
		LabelSelector:   k.LabelSelector,
		FieldSelector:   k.FieldSelector,
		Interval:        k.Interval,
		ResourceVersion: k.ResourceVersion,
	}

	copied.Namespaces = make([]string, len(k.Namespaces))
	if k.Namespaces != nil {
		copy(copied.Namespaces, k.Namespaces)
	}

	copied.ExcludeWatchType = make([]apiWatch.EventType, len(k.ExcludeWatchType))
	if k.ExcludeWatchType != nil {
		copy(copied.ExcludeWatchType, k.ExcludeWatchType)
	}

	copied.exclude = make(map[apiWatch.EventType]bool)
	for key, val := range k.exclude {
		copied.exclude[key] = val
	}

	if k.gvr != nil {
		copied.gvr = &schema.GroupVersionResource{
			Group:    k.gvr.Group,
			Version:  k.gvr.Version,
			Resource: k.gvr.Resource,
		}
	}

	return copied
}
