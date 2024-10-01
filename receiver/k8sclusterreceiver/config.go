// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclusterreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver"

import (
	"fmt"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

// Config defines configuration for kubernetes cluster receiver.
type Config struct {
	k8sconfig.APIConfig `mapstructure:",squash"`

	// Collection interval for metrics.
	CollectionInterval time.Duration `mapstructure:"collection_interval"`

	// Node condition types to report. See all condition types, see
	// here: https://kubernetes.io/docs/concepts/architecture/nodes/#condition.
	NodeConditionTypesToReport []string `mapstructure:"node_conditions_to_report"`
	// Allocate resource types to report. See all resource types, see
	// here: https://kubernetes.io/docs/concepts/architecture/nodes/#capacity
	AllocatableTypesToReport []string `mapstructure:"allocatable_types_to_report"`
	// List of exporters to which metadata from this receiver should be forwarded to.
	MetadataExporters []string `mapstructure:"metadata_exporters"`

	// Whether OpenShift support should be enabled or not.
	Distribution string `mapstructure:"distribution"`

	// Collection interval for metadata.
	// Metadata of the particular entity in the cluster is collected when the entity changes.
	// In addition metadata of all entities is collected periodically even if no changes happen.
	// Setting the duration to 0 will disable periodic collection (however will not impact
	// metadata collection on changes).
	MetadataCollectionInterval time.Duration `mapstructure:"metadata_collection_interval"`

	// MetricsBuilderConfig allows customizing scraped metrics/attributes representation.
	metadata.MetricsBuilderConfig `mapstructure:",squash"`
}

func (cfg *Config) Validate() error {
	switch cfg.Distribution {
	case distributionOpenShift:
	case distributionKubernetes:
	default:
		return fmt.Errorf("\"%s\" is not a supported distribution. Must be one of: \"openshift\", \"kubernetes\"", cfg.Distribution)
	}
	return nil
}
