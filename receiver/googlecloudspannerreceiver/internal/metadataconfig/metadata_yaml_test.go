// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadataconfig

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadataparser"
)

// Kind of sanity check test of metadata.yaml used for production usage
func TestParsingMetadataYaml(t *testing.T) {
	content, err := os.ReadFile("metadata.yaml")

	require.NoError(t, err)

	metadataMap, err := metadataparser.ParseMetadataConfig(content)

	require.NoError(t, err)
	require.NotNil(t, metadataMap)
}
