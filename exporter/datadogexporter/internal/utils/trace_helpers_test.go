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
	"bytes"
	"strings"
	"testing"
	"unicode"

	"github.com/DataDog/datadog-agent/pkg/trace/exportable/pb"
	"github.com/stretchr/testify/assert"
)

// TestGetRootFromCompleteTrace ensures that GetRoot returns a root span or fails gracefully from a complete trace chunk
// From: https://github.com/DataDog/datadog-agent/blob/eb819b117fba4d57ccc35f1a74d4618b57daf8aa/pkg/trace/traceutil/trace_test.go#L15
func TestGetRootFromCompleteTrace(t *testing.T) {
	assert := assert.New(t)

	trace := &pb.APITrace{
		TraceID:   uint64(1234),
		Spans:     []*pb.Span{},
		StartTime: 0,
		EndTime:   0,
	}

	trace.Spans = []*pb.Span{
		{TraceID: uint64(1234), SpanID: uint64(12341), Service: "s1", Name: "n1", Resource: ""},
		{TraceID: uint64(1234), SpanID: uint64(12342), ParentID: uint64(12341), Service: "s1", Name: "n1", Resource: ""},
		{TraceID: uint64(1234), SpanID: uint64(12343), ParentID: uint64(12341), Service: "s1", Name: "n1", Resource: ""},
		{TraceID: uint64(1234), SpanID: uint64(12344), ParentID: uint64(12342), Service: "s2", Name: "n2", Resource: ""},
		{TraceID: uint64(1234), SpanID: uint64(12345), ParentID: uint64(12344), Service: "s2", Name: "n2", Resource: ""},
	}

	assert.Equal(GetRoot(trace).SpanID, uint64(12341))
}

// TestGetRootFromPartialTrace ensures that GetRoot returns a root span or fails gracefully from a partial trace chunk
// From: https://github.com/DataDog/datadog-agent/blob/eb819b117fba4d57ccc35f1a74d4618b57daf8aa/pkg/trace/traceutil/trace_test.go#L29
func TestGetRootFromPartialTrace(t *testing.T) {
	assert := assert.New(t)

	trace := &pb.APITrace{
		TraceID:   uint64(1234),
		Spans:     []*pb.Span{},
		StartTime: 0,
		EndTime:   0,
	}

	trace.Spans = []*pb.Span{
		{TraceID: uint64(1234), SpanID: uint64(12341), ParentID: uint64(12340), Service: "s1", Name: "n1", Resource: ""},
		{TraceID: uint64(1234), SpanID: uint64(12342), ParentID: uint64(12341), Service: "s1", Name: "n1", Resource: ""},
		{TraceID: uint64(1234), SpanID: uint64(12343), ParentID: uint64(12342), Service: "s2", Name: "n2", Resource: ""},
	}

	assert.Equal(GetRoot(trace).SpanID, uint64(12341))
}

// ensure that truncation helper function truncates strings as expected
// and accounts for the limit and multi byte ending characters
// from https://github.com/DataDog/datadog-agent/blob/140a4ee164261ef2245340c50371ba989fbeb038/pkg/trace/traceutil/truncate_test.go#L15
func TestTruncateUTF8Strings(t *testing.T) {
	assert.Equal(t, "", TruncateUTF8("", 5))
	assert.Equal(t, "télé", TruncateUTF8("télé", 5))
	assert.Equal(t, "t", TruncateUTF8("télé", 2))
	assert.Equal(t, "éé", TruncateUTF8("ééééé", 5))
	assert.Equal(t, "ééééé", TruncateUTF8("ééééé", 18))
	assert.Equal(t, "ééééé", TruncateUTF8("ééééé", 10))
	assert.Equal(t, "ééé", TruncateUTF8("ééééé", 6))
}

func TestNormalizeTag(t *testing.T) {
	for _, tt := range []struct{ in, out string }{
		{in: "#test_starting_hash", out: "test_starting_hash"},
		{in: "TestCAPSandSuch", out: "testcapsandsuch"},
		{in: "Test Conversion Of Weird !@#$%^&**() Characters", out: "test_conversion_of_weird_characters"},
		{in: "$#weird_starting", out: "weird_starting"},
		{in: "allowed:c0l0ns", out: "allowed:c0l0ns"},
		{in: "1love", out: "love"},
		{in: "ünicöde", out: "ünicöde"},
		{in: "ünicöde:metäl", out: "ünicöde:metäl"},
		{in: "Data🐨dog🐶 繋がっ⛰てて", out: "data_dog_繋がっ_てて"},
		{in: " spaces   ", out: "spaces"},
		{in: " #hashtag!@#spaces #__<>#  ", out: "hashtag_spaces"},
		{in: ":testing", out: ":testing"},
		{in: "_foo", out: "foo"},
		{in: ":::test", out: ":::test"},
		{in: "contiguous_____underscores", out: "contiguous_underscores"},
		{in: "foo_", out: "foo"},
		{in: "\u017Fodd_\u017Fcase\u017F", out: "\u017Fodd_\u017Fcase\u017F"}, // edge-case
		{in: "", out: ""},
		{in: " ", out: ""},
		{in: "ok", out: "ok"},
		{in: "™Ö™Ö™™Ö™", out: "ö_ö_ö"},
		{in: "AlsO:ök", out: "also:ök"},
		{in: ":still_ok", out: ":still_ok"},
		{in: "___trim", out: "trim"},
		{in: "12.:trim@", out: ":trim"},
		{in: "12.:trim@@", out: ":trim"},
		{in: "fun:ky__tag/1", out: "fun:ky_tag/1"},
		{in: "fun:ky@tag/2", out: "fun:ky_tag/2"},
		{in: "fun:ky@@@tag/3", out: "fun:ky_tag/3"},
		{in: "tag:1/2.3", out: "tag:1/2.3"},
		{in: "---fun:k####y_ta@#g/1_@@#", out: "fun:k_y_ta_g/1"},
		{in: "AlsO:œ#@ö))œk", out: "also:œ_ö_œk"},
		{in: "test\x99\x8faaa", out: "test_aaa"},
		{in: "test\x99\x8f", out: "test"},
		{in: strings.Repeat("a", 888), out: strings.Repeat("a", 200)},
		{
			in: func() string {
				b := bytes.NewBufferString("a")
				for i := 0; i < 799; i++ {
					_, err := b.WriteRune('🐶')
					assert.NoError(t, err)
				}
				_, err := b.WriteRune('b')
				assert.NoError(t, err)
				return b.String()
			}(),
			out: "a", // 'b' should have been truncated
		},
		{"a" + string(unicode.ReplacementChar), "a"},
		{"a" + string(unicode.ReplacementChar) + string(unicode.ReplacementChar), "a"},
		{"a" + string(unicode.ReplacementChar) + string(unicode.ReplacementChar) + "b", "a_b"},
	} {
		t.Run("", func(t *testing.T) {
			assert.Equal(t, tt.out, NormalizeTag(tt.in), tt.in)
		})
	}
}
