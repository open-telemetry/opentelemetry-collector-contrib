package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tinybirdexporter/internal"

type Encoder interface {
	Encode(v any) error
}
