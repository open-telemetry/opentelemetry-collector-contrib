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

package datadogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHostTags(t *testing.T) {
	c := Config{
		TagsConfig: TagsConfig{
			Hostname: "customhost",
			Env:      "customenv",
			// Service and version should be only used for traces
			Service: "customservice",
			Version: "customversion",
		},

		HostMetadata: HostMetadataConfig{
			Tags: []string{"key1:val1", "key2:val2"},
		},
	}

	assert.ElementsMatch(t,
		[]string{
			"env:customenv",
			"key1:val1",
			"key2:val2",
		},
		getHostTags(&c),
	)

	c = Config{
		TagsConfig: TagsConfig{
			Hostname: "customhost",
			Env:      "customenv",
			// Service and version should be only used for traces
			Service:    "customservice",
			Version:    "customversion",
			EnvVarTags: "key3:val3 key4:val4",
		},

		HostMetadata: HostMetadataConfig{
			Tags: []string{"key1:val1", "key2:val2"},
		},
	}

	assert.ElementsMatch(t,
		[]string{
			"env:customenv",
			"key1:val1",
			"key2:val2",
		},
		getHostTags(&c),
	)

	c = Config{
		TagsConfig: TagsConfig{
			Hostname: "customhost",
			Env:      "customenv",
			// Service and version should be only used for traces
			Service:    "customservice",
			Version:    "customversion",
			EnvVarTags: "key3:val3 key4:val4",
		},
	}

	assert.ElementsMatch(t,
		[]string{
			"env:customenv",
			"key3:val3",
			"key4:val4",
		},
		getHostTags(&c),
	)
}
