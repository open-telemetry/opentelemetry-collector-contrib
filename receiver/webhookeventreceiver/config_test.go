// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package webhookeventreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/genericwebhookreceiver"

import (
	"testing"

	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func testValidateConfig(t *testing.T) {

}

func testLoadConfig(t *testing.T) {
    t.Parallel()

    cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	// LoadConf includes the TypeStr which NewFactory does not set
	id := component.NewIDWithName(typeStr, "")
	cmNoStr, err := cm.Sub(id.String())
	require.NoError(t, err)
}
