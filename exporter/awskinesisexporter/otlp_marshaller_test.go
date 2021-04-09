// Copyright 2020 The OpenTelemetry Authors
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

package awskinesisexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/pdata"
)

func TestCorrectEncoding(t *testing.T) {
	t.Parallel()
	m := otlpProtoMarshaller{}
	assert.Equal(t, otlpProto, m.Encoding())
}

func TestOTLPMetricsMarshaller(t *testing.T) {
	t.Parallel()
	td := pdata.NewMetrics()
	td.ResourceMetrics().Resize(1)
	td.ResourceMetrics().At(0).Resource().Attributes().InsertString("foo", "bar")
	expected, err := td.ToOtlpProtoBytes()
	require.NoError(t, err)
	require.NotNil(t, expected)

	m := otlpProtoMarshaller{}
	payload, err := m.MarshalMetrics(td)
	require.NoError(t, err)
	assert.Equal(t, expected, payload)
}

func TestOTLPTracesMarshaller(t *testing.T) {
	t.Parallel()
	td := pdata.NewTraces()
	td.ResourceSpans().Resize(1)
	td.ResourceSpans().At(0).Resource().Attributes().InsertString("foo", "bar")

	expected, err := td.ToOtlpProtoBytes()
	require.NoError(t, err)
	require.NotNil(t, expected)

	m := otlpProtoMarshaller{}
	payload, err := m.MarshalTraces(td)
	require.NoError(t, err)
	assert.Equal(t, expected, payload)
}
