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

package metadataparser // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadataparser"

import (
	"gopkg.in/yaml.v3"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"
)

// This function will filter output labels from the MetricsMetadata in accordance with boolean flags passed.
// Function signature is made in such a manner that it would be easy to add more filters in the future.
func filterOutLabelsIfRequired(mData *metadata.MetricsMetadata, hideTopnQuerystatsQuerytext bool) {
	if hideTopnQuerystatsQuerytext == true && mData.Name == "top minute query stats" {
		index := 0 // output index
		for _, label := range mData.QueryLabelValuesMetadata {
			if label.Name() != "query_text" && label.Name() != "query_text_truncated" {
				// copy and increment index
				mData.QueryLabelValuesMetadata[index] = label
				index++
			}
		}
		// Prevent memory leak by erasing truncated values
		for j := index; j < len(mData.QueryLabelValuesMetadata); j++ {
			mData.QueryLabelValuesMetadata[j] = nil
		}
		mData.QueryLabelValuesMetadata = mData.QueryLabelValuesMetadata[:index]
	}
}

func ParseMetadataConfig(metadataContentYaml []byte, hideTopnQuerystatsQuerytext bool) ([]*metadata.MetricsMetadata, error) {
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

		filterOutLabelsIfRequired(mData, hideTopnQuerystatsQuerytext)
		result[i] = mData
	}

	return result, nil
}
