// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package geoipprocessor

import (
	"os"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/provider"
	maxmind "github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/provider/maxmindprovider"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/provider/maxmindprovider/testdata"
)

func TestProcessorWithMaxMind(t *testing.T) {
	tmpDBfiles := testdata.GenerateLocalDB(t, "./internal/provider/maxmindprovider/testdata/")
	defer os.RemoveAll(tmpDBfiles)

	maxmindConfig := maxmind.Config{
		DatabasePath: tmpDBfiles + "/" + "GeoLite2-City-Test.mmdb",
	}

	for _, tt := range testCases {
		t.Run("maxmind_"+tt.name, func(t *testing.T) {
			cfg := &Config{Context: tt.context, Providers: map[string]provider.Config{"maxmind": &maxmindConfig}}

			compareAllSignals(cfg, tt.goldenDir)(t)
		})
	}
}
