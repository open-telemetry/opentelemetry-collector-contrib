package kafkaexporter

import (
	"testing"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestPdataTracesMarshaler_Marshal(t *testing.T) {
	tests := []struct {
		name                 string
		maxMessageBytes      int
		partitionedByTraceID bool
		traces               ptrace.Traces
		wantChunks           int
		wantError            bool
	}{
		{
			name:                 "empty_trace",
			maxMessageBytes:      1000,
			partitionedByTraceID: false,
			traces:               ptrace.NewTraces(),
			wantChunks:           0,
			wantError:            false,
		},
		{
			name:                 "single_small_trace",
			maxMessageBytes:      1000,
			partitionedByTraceID: false,
			traces:               createTracesWithSize(1, 1, 10), // 1 resource, 1 span, small attributes
			wantChunks:           1,
			wantError:            false,
		},
		{
			name:                 "single_large_trace_needs_splitting",
			maxMessageBytes:      10000,
			partitionedByTraceID: false,
			traces:               createTracesWithSize(1, 10, 50), // 1 resource, 10 spans, larger attributes
			wantChunks:           2,                               // Exact number depends on size
			wantError:            false,
		},
		{
			name:                 "multiple_traces_partitioned",
			maxMessageBytes:      1000,
			partitionedByTraceID: true,
			traces:               createTracesWithMultipleTraceIDs(3, 1, 10), // 3 traces, 1 span each, small attributes
			wantChunks:           1,
			wantError:            false,
		},
		{
			name:                 "oversized_single_span",
			maxMessageBytes:      10,
			partitionedByTraceID: false,
			traces:               createTracesWithSize(1, 1, 1000), // 1 resource, 1 span, very large attributes
			wantChunks:           0,
			wantError:            true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			marshaler := newPdataTracesMarshaler(
				&ptrace.ProtoMarshaler{},
				"proto",
				tt.partitionedByTraceID,
				tt.maxMessageBytes,
			)

			chunks, err := marshaler.Marshal(tt.traces, "test-topic")

			if tt.wantError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantChunks, len(chunks))

			for _, chunk := range chunks {
				for _, msg := range chunk.Messages {
					assert.LessOrEqual(t, msg.ByteSize(2), tt.maxMessageBytes)
				}
			}
		})
	}
}

func TestPdataTracesMarshaler_FindSplitPoint(t *testing.T) {
	tests := []struct {
		name            string
		maxMessageBytes int
		trace           ptrace.Traces
		wantSplitPoint  int
		wantError       bool
	}{
		{
			name:            "empty_trace",
			maxMessageBytes: 1000,
			trace:           ptrace.NewTraces(),
			wantSplitPoint:  0,
			wantError:       true,
		},
		{
			name:            "single_span_fits",
			maxMessageBytes: 1000,
			trace:           createTracesWithSize(1, 1, 10),
			wantSplitPoint:  1,
			wantError:       false,
		},
		{
			name:            "single_span_too_large",
			maxMessageBytes: 10,
			trace:           createTracesWithSize(1, 1, 1000),
			wantSplitPoint:  0,
			wantError:       true,
		},
		{
			name:            "multiple_spans_partial_fit",
			maxMessageBytes: 3000,
			trace:           createTracesWithSize(1, 10, 100),
			wantSplitPoint:  1,
			wantError:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			marshaler := newPdataTracesMarshaler(
				&ptrace.ProtoMarshaler{},
				"proto",
				false,
				tt.maxMessageBytes,
			)

			splitPoint, err := marshaler.(*pdataTracesMarshaler).findSplitPoint(tt.trace)

			if tt.wantError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			if tt.wantSplitPoint > 0 {
				assert.Equal(t, tt.wantSplitPoint, splitPoint)
			}

			// Verify split point produces valid sized messages
			protoMarshaler := &ptrace.ProtoMarshaler{}
			if splitPoint > 0 {
				firstHalf, _ := marshaler.(*pdataTracesMarshaler).splitTraceBySpans(tt.trace, 0, splitPoint)
				bts, err := protoMarshaler.MarshalTraces(firstHalf)
				require.NoError(t, err)

				msgSize := (&sarama.ProducerMessage{
					Value: sarama.ByteEncoder(bts),
				}).ByteSize(2)

				assert.LessOrEqual(t, msgSize, tt.maxMessageBytes)
			}
		})
	}
}

func TestPdataTracesMarshaler_SplitTraceBySpans(t *testing.T) {
	tests := []struct {
		name       string
		trace      ptrace.Traces
		low        int
		high       int
		wantFirst  int
		wantSecond int
	}{
		{
			name:       "split_empty_trace",
			trace:      ptrace.NewTraces(),
			low:        0,
			high:       0,
			wantFirst:  0,
			wantSecond: 0,
		},
		{
			name:       "split_single_span",
			trace:      createTracesWithSize(1, 1, 10),
			low:        0,
			high:       1,
			wantFirst:  1,
			wantSecond: 0,
		},
		{
			name:       "split_multiple_spans",
			trace:      createTracesWithSize(1, 10, 10),
			low:        0,
			high:       5,
			wantFirst:  5,
			wantSecond: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			marshaler := newPdataTracesMarshaler(
				&ptrace.ProtoMarshaler{},
				"proto",
				false,
				1000,
			)

			first, second := marshaler.(*pdataTracesMarshaler).splitTraceBySpans(tt.trace, tt.low, tt.high)

			if tt.trace.ResourceSpans().Len() > 0 {
				firstSpans := first.ResourceSpans().At(0).ScopeSpans().At(0).Spans()
				secondSpans := second.ResourceSpans().At(0).ScopeSpans().At(0).Spans()
				assert.Equal(t, tt.wantFirst, firstSpans.Len())
				assert.Equal(t, tt.wantSecond, secondSpans.Len())

				if tt.wantFirst > 0 {
					assert.Equal(t,
						tt.trace.ResourceSpans().At(0).Resource().Attributes().AsRaw(),
						first.ResourceSpans().At(0).Resource().Attributes().AsRaw(),
					)
				}
				if tt.wantSecond > 0 {
					assert.Equal(t,
						tt.trace.ResourceSpans().At(0).Resource().Attributes().AsRaw(),
						second.ResourceSpans().At(0).Resource().Attributes().AsRaw(),
					)
				}
			}
		})
	}
}

// Helper function

func createTracesWithSize(resourceSpans, spansPerResource int, attrSize int) ptrace.Traces {
	traces := ptrace.NewTraces()

	for i := 0; i < resourceSpans; i++ {
		rs := traces.ResourceSpans().AppendEmpty()
		rs.Resource().Attributes().PutStr("resource", "resource-"+string(rune(i)))

		ss := rs.ScopeSpans().AppendEmpty()
		ss.Scope().SetName("scope-" + string(rune(i)))

		for j := 0; j < spansPerResource; j++ {
			span := ss.Spans().AppendEmpty()
			span.SetName("span-" + string(rune(j)))
			attrs := span.Attributes()
			for k := 0; k < attrSize; k++ {
				attrs.PutStr("key-"+string(rune(k)), "value-"+string(rune(k)))
			}
		}
	}

	return traces
}

func createTracesWithMultipleTraceIDs(numTraces, spansPerTrace, attrSize int) ptrace.Traces {
	traces := ptrace.NewTraces()

	for i := 0; i < numTraces; i++ {
		rs := traces.ResourceSpans().AppendEmpty()
		ss := rs.ScopeSpans().AppendEmpty()

		for j := 0; j < spansPerTrace; j++ {
			span := ss.Spans().AppendEmpty()
			traceID := pcommon.TraceID([16]byte{byte(i), byte(j)})
			span.SetTraceID(traceID)

			attrs := span.Attributes()
			for k := 0; k < attrSize; k++ {
				attrs.PutStr("key-"+string(rune(k)), "value-"+string(rune(k)))
			}
		}
	}

	return traces
}
