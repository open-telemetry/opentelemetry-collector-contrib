// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package goldendataset

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateTraces(t *testing.T) {
	rscSpans, err := GenerateTraces("testdata/generated_pict_pairs_traces.txt",
		"testdata/generated_pict_pairs_spans.txt")
	assert.NoError(t, err)
	assert.Len(t, rscSpans, 32)
}
