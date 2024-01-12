// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package configschema

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver"
)

const gcpCollectorPath = "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"

func TestTokensToPartialPath(t *testing.T) {
	path, err := requireTokensToPartialPath([]string{gcpCollectorPath, "42"})
	require.NoError(t, err)
	assert.Equal(t, "github.com/!google!cloud!platform/opentelemetry-operations-go/exporter/collector@42", path)
}

func TestTypeToPackagePath_Error(t *testing.T) {
	dr := NewDirResolver("foo/bar", DefaultModule)
	_, err := dr.TypeToPackagePath(reflect.ValueOf(redisreceiver.Config{}).Type())
	require.Error(t, err)
}
