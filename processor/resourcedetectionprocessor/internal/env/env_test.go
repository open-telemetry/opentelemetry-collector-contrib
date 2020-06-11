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

package env

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

func TestDetectTrue(t *testing.T) {
	os.Setenv(envVar, "key=value")

	detector := &Detector{}
	res, err := detector.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, internal.NewResource(map[string]string{"key": "value"}), res)
}

func TestDetectFalse(t *testing.T) {
	os.Setenv(envVar, "")

	detector := &Detector{}
	res, err := detector.Detect(context.Background())
	require.NoError(t, err)
	assert.True(t, internal.IsEmptyResource(res))
}

func TestDetectError(t *testing.T) {
	os.Setenv(envVar, "key=value,key")

	detector := &Detector{}
	res, err := detector.Detect(context.Background())
	assert.Error(t, err)
	assert.True(t, internal.IsEmptyResource(res))
}

func TestDecodeLabels(t *testing.T) {
	cases := []struct {
		encoded    string
		wantLabels pdata.AttributeMap
		wantFail   bool
	}{
		{
			encoded:    ` example.org/test-1 =  test $ %3A \" ,  Abc=Def  `,
			wantLabels: internal.NewAttributeMap(map[string]string{"example.org/test-1": `test $ : \"`, "Abc": "Def"}),
		}, {
			encoded:    `single=key`,
			wantLabels: internal.NewAttributeMap(map[string]string{"single": "key"}),
		},
		{encoded: `invalid-char-ü="test"`, wantFail: true},
		{encoded: `invalid-char="ü-test"`, wantFail: true},
		{encoded: `extra=chars, a`, wantFail: true},
	}

	for i, c := range cases {
		t.Run(fmt.Sprintf("case-%d", i), func(t *testing.T) {
			am := pdata.NewAttributeMap()
			err := initializeAttributeMap(am, c.encoded)

			if c.wantFail {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				c.wantLabels.Sort()
				am.Sort()
				assert.Equal(t, c.wantLabels, am)
			}
		})
	}
}
