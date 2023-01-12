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

package comparetest // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/comparetest"

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type expectation struct {
	err    error
	reason string
}

func (e expectation) validate(t *testing.T, err error) {
	if e.err == nil {
		require.NoError(t, err, e.reason)
		return
	}
	require.Equal(t, e.err, err, e.reason)
}
