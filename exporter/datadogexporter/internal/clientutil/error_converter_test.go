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

package clientutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/clientutil"

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumererror"
)

func TestWrapError(t *testing.T) {
	respOK := http.Response{StatusCode: 200}
	respRetriable := http.Response{StatusCode: 402}
	respNonRetriable := http.Response{StatusCode: 404}
	err := fmt.Errorf("Test error")
	assert.False(t, consumererror.IsPermanent(WrapError(err, &respOK)))
	assert.False(t, consumererror.IsPermanent(WrapError(err, &respRetriable)))
	assert.True(t, consumererror.IsPermanent(WrapError(err, &respNonRetriable)))
	assert.False(t, consumererror.IsPermanent(WrapError(nil, &respNonRetriable)))
	assert.False(t, consumererror.IsPermanent(WrapError(err, nil)))
}
