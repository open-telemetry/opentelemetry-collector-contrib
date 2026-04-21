// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
