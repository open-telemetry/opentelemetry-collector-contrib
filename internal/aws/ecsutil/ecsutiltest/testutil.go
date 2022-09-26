// Copyright The OpenTelemetry Authors
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

package ecsutiltest // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil/ecsutiltest"

import (
	_ "embed"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil/endpoints"
)

//go:embed testdata/container_metadata.json
var ContainerMetadataTestResponse []byte

//go:embed testdata/task_metadata.json
var TaskMetadataTestResponse []byte

// GetTestdataResponseByPath will return example metadata for a given path.
func GetTestdataResponseByPath(_ *testing.T, path string) ([]byte, error) {
	switch path {
	case endpoints.TaskMetadataPath:
		return TaskMetadataTestResponse, nil
	case endpoints.ContainerMetadataPath:
		return ContainerMetadataTestResponse, nil
	}
	return nil, nil
}
