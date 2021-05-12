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

package utils

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestDoWithRetries(t *testing.T) {
	i, err := DoWithRetries(3, func() error { return nil })
	require.NoError(t, err)
	assert.Equal(t, i, 0)

	_, err = DoWithRetries(1, func() error { return errors.New("action failed") })
	require.Error(t, err)
}
