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

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/internal"

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

type Expectation struct {
	Err    error
	Reason string
}

func (e Expectation) Validate(t *testing.T, err error) {
	if e.Err == nil {
		require.NoError(t, err, e.Reason)
		return
	}
	require.Equal(t, e.Err, err, e.Reason)
}

func MaskResourceAttributeValue(res pcommon.Resource, attr string) {
	if _, ok := res.Attributes().Get(attr); ok {
		res.Attributes().Remove(attr)
	}
}
