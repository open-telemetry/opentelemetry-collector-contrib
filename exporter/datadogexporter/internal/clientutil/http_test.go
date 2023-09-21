// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clientutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/clientutil"

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
)

var (
	buildInfo = component.BuildInfo{
		Command: "otelcontribcol",
		Version: "1.0",
	}
)

func TestUserAgent(t *testing.T) {

	assert.Equal(t, UserAgent(buildInfo), "otelcontribcol/1.0")
}

func TestDDHeaders(t *testing.T) {
	header := http.Header{}
	apiKey := "apikey"
	SetDDHeaders(header, buildInfo, apiKey)
	assert.Equal(t, header.Get("DD-Api-Key"), apiKey)
	assert.Equal(t, header.Get("USer-Agent"), "otelcontribcol/1.0")

}
