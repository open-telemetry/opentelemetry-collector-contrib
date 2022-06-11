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

package noop

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestBuildValid(t *testing.T) {
	cfg := NewConfig("test")
	op, err := cfg.Build(testutil.Logger(t))
	require.NoError(t, err)
	require.IsType(t, &Transformer{}, op)
}

func TestBuildInvalid(t *testing.T) {
	cfg := NewConfig("test")
	_, err := cfg.Build(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "build context is missing a logger")
}

func TestProcess(t *testing.T) {
	cfg := NewConfig("test")
	cfg.OutputIDs = []string{"fake"}
	op, err := cfg.Build(testutil.Logger(t))
	require.NoError(t, err)

	fake := testutil.NewFakeOutput(t)
	require.NoError(t, op.SetOutputs([]operator.Operator{fake}))

	entry := entry.New()
	entry.AddAttribute("label", "value")
	entry.AddResourceKey("resource", "value")
	entry.TraceID = []byte{0x01}
	entry.SpanID = []byte{0x01}
	entry.TraceFlags = []byte{0x01}

	expected := entry.Copy()
	err = op.Process(context.Background(), entry)
	require.NoError(t, err)

	fake.ExpectEntry(t, expected)
}
