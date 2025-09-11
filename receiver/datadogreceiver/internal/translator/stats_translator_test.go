// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator

import (
	"net/http"
	"strings"
	"testing"

	pb "github.com/DataDog/datadog-agent/pkg/proto/pbgo/trace"
	"github.com/DataDog/sketches-go/ddsketch"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

// The sketch's relative accuracy and maximum number of bins is identical
// to the one used in the trace-agent for consistency:
// https://github.com/DataDog/datadog-agent/blob/cbac965/pkg/trace/stats/statsraw.go#L18-L26
const (
	sketchRelativeAccuracy = 0.01
	sketchMaxBins          = 2048
)

func TestTranslateStats(t *testing.T) {
	want := &pb.ClientStatsPayload{
		Hostname:         "host",
		Env:              "prod",
		Version:          "v1.2",
		Lang:             "go",
		TracerVersion:    "v44",
		RuntimeID:        "123jkl",
		Sequence:         2,
		AgentAggregation: "blah",
		Service:          "mysql",
		ContainerID:      "abcdef123456",
		Tags:             []string{"a:b", "c:d"},
		Stats: []*pb.ClientStatsBucket{
			{
				Start:    10,
				Duration: 1,
				Stats: []*pb.ClientGroupedStats{
					{
						Service:        "mysql",
						Name:           "db.query",
						Resource:       "UPDATE name",
						HTTPStatusCode: 100,
						Type:           "sql",
						DBType:         "postgresql",
						Synthetics:     true,
						Hits:           5,
						Errors:         2,
						Duration:       100,
						OkSummary:      getTestSketchBytes(1, 2, 3),
						ErrorSummary:   getTestSketchBytes(4, 5, 6),
						TopLevelHits:   3,
					},
					{
						Service:        "kafka",
						Name:           "queue.add",
						Resource:       "append",
						HTTPStatusCode: 220,
						Type:           "queue",
						Hits:           15,
						Errors:         3,
						Duration:       143,
						OkSummary:      nil,
						ErrorSummary:   nil,
						TopLevelHits:   5,
					},
				},
			},
			{
				Start:    20,
				Duration: 3,
				Stats: []*pb.ClientGroupedStats{
					{
						Service:        "php-go",
						Name:           "http.post",
						Resource:       "user_profile",
						HTTPStatusCode: 440,
						Type:           "web",
						Hits:           11,
						Errors:         3,
						Duration:       987,
						OkSummary:      getTestSketchBytes(7, 8),
						ErrorSummary:   getTestSketchBytes(9, 10, 11),
						TopLevelHits:   1,
					},
				},
			},
		},
	}

	lang := "kotlin"
	tracerVersion := "1.2.3"

	t.Run("same", func(t *testing.T) {
		st := NewStatsTranslator()
		mx, err := st.TranslateStats(want, lang, tracerVersion)
		assert.NoError(t, err)

		var results []*pb.StatsPayload
		for i := 0; i < mx.ResourceMetrics().Len(); i++ {
			rm := mx.ResourceMetrics().At(i)
			for j := 0; j < rm.ScopeMetrics().Len(); j++ {
				sm := rm.ScopeMetrics().At(j)
				for k := 0; k < sm.Metrics().Len(); k++ {
					md := sm.Metrics().At(k)
					// these metrics are an APM Stats payload; consume it as such
					for l := 0; l < md.Sum().DataPoints().Len(); l++ {
						if payload, ok := md.Sum().DataPoints().At(l).Attributes().Get(keyStatsPayload); ok {
							stats := &pb.StatsPayload{}
							err = proto.Unmarshal(payload.Bytes().AsRaw(), stats)
							assert.NoError(t, err)
							results = append(results, stats)
						}
					}
					assert.NoError(t, err)
				}
			}
		}

		assert.Len(t, results, 1)
		assert.True(t, proto.Equal(want, results[0].Stats[0]))
	})
}

func getTestSketchBytes(nums ...float64) []byte {
	sketch, err := ddsketch.LogCollapsingLowestDenseDDSketch(sketchRelativeAccuracy, sketchMaxBins)
	if err != nil {
		// the only possible error is if the relative accuracy is < 0 or > 1;
		// we know that's not the case because it's a constant defined as 0.01
		panic(err)
	}
	for _, num := range nums {
		err := sketch.Add(num)
		if err != nil {
			panic(err)
		}
	}

	buf, err2 := proto.Marshal(sketch.ToProto())
	if err2 != nil {
		// there should be no error under any circumstances here
		panic(err2)
	}
	return buf
}

func TestProcessStats(t *testing.T) {
	testCases := []struct {
		in            *pb.ClientStatsPayload
		lang          string
		tracerVersion string
		out           *pb.ClientStatsPayload
	}{
		{
			in: &pb.ClientStatsPayload{
				Hostname: "tracer_hots",
				Env:      "tracer_env",
				Version:  "code_version",
				Stats: []*pb.ClientStatsBucket{
					{
						Start:    1,
						Duration: 2,
						Stats: []*pb.ClientGroupedStats{
							{
								Service:        "service",
								Name:           "name------",
								Resource:       "resource",
								HTTPStatusCode: http.StatusOK,
								Type:           "web",
							},
							{
								Service:        "redis_service",
								Name:           "name-2",
								Resource:       "SET k v",
								HTTPStatusCode: http.StatusOK,
								Type:           "redis",
							},
						},
					},
				},
			},
			lang:          "java",
			tracerVersion: "v1",
			out: &pb.ClientStatsPayload{
				Hostname:      "tracer_hots",
				Env:           "tracer_env",
				Version:       "code_version",
				Lang:          "java",
				TracerVersion: "v1",
				Stats: []*pb.ClientStatsBucket{
					{
						Start:    1,
						Duration: 2,
						Stats: []*pb.ClientGroupedStats{
							{
								Service:        "service",
								Name:           "name",
								Resource:       "resource",
								HTTPStatusCode: 200,
								Type:           "web",
							},
							{
								Service:        "redis_service",
								Name:           "name_2",
								Resource:       "SET",
								HTTPStatusCode: 200,
								Type:           "redis",
							},
						},
					},
				},
			},
		},
	}
	for _, testCase := range testCases {
		st := NewStatsTranslator()
		out := st.processStats(testCase.in, testCase.lang, testCase.tracerVersion)
		assert.Equal(t, testCase.out, out)
	}
}

func TestMergeDuplicates(t *testing.T) {
	in := &pb.ClientStatsBucket{
		Stats: []*pb.ClientGroupedStats{
			{
				Service:      "s1",
				Resource:     "r1",
				Name:         "n1",
				Hits:         2,
				TopLevelHits: 2,
				Errors:       1,
				Duration:     123,
			},
			{
				Service:      "s2",
				Resource:     "r1",
				Name:         "n1",
				Hits:         2,
				TopLevelHits: 2,
				Errors:       0,
				Duration:     123,
			},
			{
				Service:      "s1",
				Resource:     "r1",
				Name:         "n1",
				Hits:         2,
				TopLevelHits: 2,
				Errors:       1,
				Duration:     123,
			},
			{
				Service:      "s2",
				Resource:     "r1",
				Name:         "n1",
				Hits:         2,
				TopLevelHits: 2,
				Errors:       0,
				Duration:     123,
			},
		},
	}
	expected := &pb.ClientStatsBucket{
		Stats: []*pb.ClientGroupedStats{
			{
				Service:      "s1",
				Resource:     "r1",
				Name:         "n1",
				Hits:         4,
				TopLevelHits: 2,
				Errors:       2,
				Duration:     246,
			},
			{
				Service:      "s2",
				Resource:     "r1",
				Name:         "n1",
				Hits:         4,
				TopLevelHits: 2,
				Errors:       0,
				Duration:     246,
			},
			{
				Service:      "s1",
				Resource:     "r1",
				Name:         "n1",
				Hits:         0,
				TopLevelHits: 2,
				Errors:       0,
				Duration:     0,
			},
			{
				Service:      "s2",
				Resource:     "r1",
				Name:         "n1",
				Hits:         0,
				TopLevelHits: 2,
				Errors:       0,
				Duration:     0,
			},
		},
	}
	mergeDuplicates(in)
	assert.Equal(t, expected, in)
}

func TestTruncateResource(t *testing.T) {
	t.Run("over", func(t *testing.T) {
		r, ok := truncateResource("resource")
		assert.True(t, ok)
		assert.Equal(t, "resource", r)
	})

	t.Run("under", func(t *testing.T) {
		s := strings.Repeat("a", maxResourceLen)
		r, ok := truncateResource(s + "extra string")
		assert.False(t, ok)
		assert.Equal(t, s, r)
	})
}

func TestObfuscateStatsGroup(t *testing.T) {
	statsGroup := func(typ, resource string) *pb.ClientGroupedStats {
		return &pb.ClientGroupedStats{
			Type:     typ,
			Resource: resource,
		}
	}
	for _, tt := range []struct {
		in  *pb.ClientGroupedStats // input stats
		out string                 // output obfuscated resource
	}{
		{statsGroup("sql", "SELECT 1 FROM db"), "SELECT ? FROM db"},
		{statsGroup("sql", "SELECT 1\nFROM Blogs AS [b\nORDER BY [b]"), textNonParsable},
		{statsGroup("redis", "ADD 1, 2"), "ADD"},
		{statsGroup("other", "ADD 1, 2"), "ADD 1, 2"},
	} {
		st := NewStatsTranslator()
		st.obfuscateStatsGroup(tt.in)
		assert.Equal(t, tt.in.Resource, tt.out)
	}
}
