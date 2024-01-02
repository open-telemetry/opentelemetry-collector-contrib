// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadataparser // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadataparser"

import (
	"gopkg.in/yaml.v3"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"
)

func ParseMetadataConfig(metadataContentYaml []byte) ([]*metadata.MetricsMetadata, error) {
	var config MetadataConfig

	err := yaml.Unmarshal(metadataContentYaml, &config)
	if err != nil {
		return nil, err
	}

	result := make([]*metadata.MetricsMetadata, len(config.Metadata))

	for i, parsedMetadata := range config.Metadata {
		mData, err := parsedMetadata.MetricsMetadata()
		if err != nil {
			return nil, err
		}

		result[i] = mData
	}

	return result, nil
}
