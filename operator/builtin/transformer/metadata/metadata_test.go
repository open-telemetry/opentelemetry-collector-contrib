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

package metadata

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/helper"
	"github.com/open-telemetry/opentelemetry-log-collection/testutil"
)

func TestMetadata(t *testing.T) {
	os.Setenv("TEST_METADATA_OPERATOR_ENV", "foo")
	defer os.Unsetenv("TEST_METADATA_OPERATOR_ENV")

	cases := []struct {
		name      string
		configMod func(*MetadataOperatorConfig)
		input     *entry.Entry
		expected  *entry.Entry
	}{
		{
			"AddAttributeLiteral",
			func(cfg *MetadataOperatorConfig) {
				cfg.Attributes = map[string]helper.ExprStringConfig{
					"label1": "value1",
				}
			},
			entry.New(),
			func() *entry.Entry {
				e := entry.New()
				e.Attributes = map[string]string{
					"label1": "value1",
				}
				return e
			}(),
		},
		{
			"AddAttributeExpr",
			func(cfg *MetadataOperatorConfig) {
				cfg.Attributes = map[string]helper.ExprStringConfig{
					"label1": `EXPR("start" + "end")`,
				}
			},
			entry.New(),
			func() *entry.Entry {
				e := entry.New()
				e.Attributes = map[string]string{
					"label1": "startend",
				}
				return e
			}(),
		},
		{
			"AddAttributeEnv",
			func(cfg *MetadataOperatorConfig) {
				cfg.Attributes = map[string]helper.ExprStringConfig{
					"label1": `EXPR(env("TEST_METADATA_OPERATOR_ENV"))`,
				}
			},
			entry.New(),
			func() *entry.Entry {
				e := entry.New()
				e.Attributes = map[string]string{
					"label1": "foo",
				}
				return e
			}(),
		},
		{
			"AddResourceLiteral",
			func(cfg *MetadataOperatorConfig) {
				cfg.Resource = map[string]helper.ExprStringConfig{
					"key1": "value1",
				}
			},
			entry.New(),
			func() *entry.Entry {
				e := entry.New()
				e.Resource = map[string]string{
					"key1": "value1",
				}
				return e
			}(),
		},
		{
			"AddResourceExpr",
			func(cfg *MetadataOperatorConfig) {
				cfg.Resource = map[string]helper.ExprStringConfig{
					"key1": `EXPR("start" + "end")`,
				}
			},
			entry.New(),
			func() *entry.Entry {
				e := entry.New()
				e.Resource = map[string]string{
					"key1": "startend",
				}
				return e
			}(),
		},
		{
			"AddResourceEnv",
			func(cfg *MetadataOperatorConfig) {
				cfg.Resource = map[string]helper.ExprStringConfig{
					"key1": `EXPR(env("TEST_METADATA_OPERATOR_ENV"))`,
				}
			},
			entry.New(),
			func() *entry.Entry {
				e := entry.New()
				e.Resource = map[string]string{
					"key1": "foo",
				}
				return e
			}(),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := NewMetadataOperatorConfig("test_operator_id")
			cfg.OutputIDs = []string{"fake"}
			tc.configMod(cfg)
			op, err := cfg.Build(testutil.NewBuildContext(t))
			require.NoError(t, err)

			fake := testutil.NewFakeOutput(t)
			err = op.SetOutputs([]operator.Operator{fake})
			require.NoError(t, err)

			err = op.Process(context.Background(), tc.input)
			require.NoError(t, err)

			select {
			case e := <-fake.Received:
				require.Equal(t, e.Attributes, tc.expected.Attributes)
				require.Equal(t, e.Resource, tc.expected.Resource)
			case <-time.After(time.Second):
				require.FailNow(t, "Timed out waiting for entry to be processed")
			}
		})
	}
}
