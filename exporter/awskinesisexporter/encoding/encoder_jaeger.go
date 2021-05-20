package encoding

import (
	awskinesis "github.com/signalfx/opencensus-go-exporter-kinesis"
	"go.opentelemetry.io/collector/consumer/pdata"
	jaegertranslator "go.opentelemetry.io/collector/translator/trace/jaeger"
)

type jaeger struct {
	kinesis *awskinesis.Exporter
}

// Ensure the jaeger encoder meets the interface at compile time.
var _ Encoder = (*jaeger)(nil)

func Jaeger(kinesis *awskinesis.Exporter) Encoder {
	return &jaeger{kinesis: kinesis}
}

func (j *jaeger) EncodeTraces(td pdata.Traces) error {
	traces, err := jaegertranslator.InternalTracesToJaegerProto(td)
	if err != nil {
		return err
	}

	for _, trace := range traces {
		for _, span := range trace.GetSpans() {
			if span.Process == nil {
				span.Process = trace.Process
			}
			if err := j.kinesis.ExportSpan(span); err != nil {
				return err
			}
		}
	}

	return nil
}

func (j *jaeger) EncodeMetrics(_ pdata.Metrics) error { return ErrUnsupportedEncodedType }

func (j *jaeger) EncodeLogs(_ pdata.Logs) error { return ErrUnsupportedEncodedType }
