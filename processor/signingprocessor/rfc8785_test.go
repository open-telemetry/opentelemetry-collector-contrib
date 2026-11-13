// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signingprocessor

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/probityai/agent-evidence-vectors/jcsvectors"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

// minVectors stops a bump that drops vectors from passing on fewer of them.
const minVectors = 50

var canonicalLeaf = regexp.MustCompile(`"(?:[^"\\]|\\.)*":?|true|false|-?\d[-+.0-9e]*`)

// TestRFC8785Vectors runs published RFC 8785 vectors through
// serializeLogRecord, so a jcs or Go bump that moves the signed bytes fails
// here. The vectors come from a module pinned in go.mod and go.sum.
//
// Only the accept cases are run. The reject cases are lone surrogates,
// non-finite numbers and duplicate members, which never reach jcs.Transform
// on this path: TestInvalidUTF8Rejected and TestFloat64SpecialValuesRejected
// cover the first two, and a pcommon.Map cannot hold a duplicate member.
func TestRFC8785Vectors(t *testing.T) {
	vectors, err := jcsvectors.Canonical()
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(vectors), minVectors)

	p := &signingProcessor{}
	for _, vector := range vectors {
		t.Run(vector.Corpus+"/"+vector.ID, func(t *testing.T) {
			var decoded any
			require.NoError(t, json.Unmarshal(vector.Input, &decoded))
			value := pcommon.NewValueEmpty()
			require.NoError(t, value.FromRaw(decoded))
			lr := plog.NewLogRecord()
			value.CopyTo(lr.Attributes().PutEmpty("v"))

			got, err := p.serializeLogRecord(lr)
			require.NoError(t, err)
			require.Equal(t, `{"attributes":{"v":`+wrapLeaves(string(vector.Canonical))+`}}`, string(got))
		})
	}
}

// wrapLeaves gives canonical JSON the shape valueToInterface gives the same
// value: each scalar becomes a one-member object, and member order is unchanged.
func wrapLeaves(canonical string) string {
	return canonicalLeaf.ReplaceAllStringFunc(canonical, func(leaf string) string {
		switch {
		case strings.HasSuffix(leaf, `":`):
			return leaf // a member name
		case leaf[0] == '"':
			return `{"stringValue":` + leaf + `}`
		case leaf == "true" || leaf == "false":
			return `{"boolValue":` + leaf + `}`
		default:
			return `{"doubleValue":` + leaf + `}`
		}
	})
}
