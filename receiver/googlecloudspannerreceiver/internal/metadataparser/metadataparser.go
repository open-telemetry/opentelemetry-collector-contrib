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

package metadataparser

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
